// package nope is a blocklist plugin for CoreDNS.
package nope

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/exp/constraints"
)

var log = clog.NewWithPlugin("nope")

// Nope manages a blocklist and passes on or denies DNS requests.
type Nope struct {
	next plugin.Handler

	blockers []blocker
}

// Name implements plugin.Handler
func (Nope) Name() string { return "nope" }

// Ready implements ready.Ready. It's not super likely to be checked more than
// once, but we may as well be thorough.
func (n *Nope) Ready() bool { return true }

// ServeDNS implements plugin.Handler
func (n *Nope) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	if n := len(r.Question); n != 1 {
		// TODO: proper error
		return 0, fmt.Errorf("%d questions is not 1", n)
	}
	// TODO: probably not all question types and classes
	host := dns.CanonicalName(r.Question[0].Name)
	if ok, list := n.Block(host); ok {
		log.Debugf("%q: blocked by %q", host, list)
		// Return an NXDOMAIN, pretend it doesn't exist.
		// TODO: there may be better things to do.
		requestCount.WithLabelValues(metrics.WithServer(ctx), list, host, "blocked").Inc()
		resp := &dns.Msg{}
		resp.SetRcode(r, dns.RcodeNameError)
		if err := w.WriteMsg(resp); err != nil {
			return 0, err
		}
		return dns.RcodeNameError, nil
	}

	// pass it along.
	requestCount.WithLabelValues(metrics.WithServer(ctx), "", host, "allowed").Inc()
	return plugin.NextOrFailure(n.Name(), n.next, ctx, w, r)
}

// requestCount exports a prometheus metric that tracks blocked/allowed requests.
// TODO: adding the host as a label is a bit silly, stop it.
var requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: plugin.Namespace,
	Subsystem: Nope{}.Name(),
	Name:      "request_count_total",
	Help:      "counter of requests made",
}, []string{"server", "blocker", "host", "outcome"})

// Block returns true if a host is blocked, false if the request should proceed.
func (n *Nope) Block(host string) (bool, string) {
	for _, b := range n.blockers {
		if b.block(host) {
			return true, b.name()
		}
	}
	return false, ""
}

// blocker is something that blocks hosts.
type blocker interface {
	// name returns an identifier for the blocklist, for use in metrics etc.
	name() string
	// blocked receives a canonicalised FQDN and returns true if the domain
	// should be blocked.
	block(string) bool
	// ready returns true if the blocker is ready to receive calls to block.
	ready() bool
}

// blocklist is a blocker that never changes, and once constructed is
// always ready.
type blocklist struct {
	title   string
	blocked []string // sorted by backwardsStringCompare
}

func newBlocklist(name string, blocked []string) blocklist {
	blocked = slices.Clone(blocked)
	for i := range blocked {
		blocked[i] = dns.CanonicalName(blocked[i])
	}
	// sort with a regular lexical comparison but backwards, so that domains sort
	// before all of their subdomains.
	slices.SortFunc(blocked, backwardsStringCompare)

	return blocklist{
		title:   name,
		blocked: blocked,
	}
}

func newBlocklistFromFile(name, path string) (blocklist, error) {
	f, err := os.Open(path)
	if err != nil {
		return blocklist{}, err
	}
	defer f.Close()

	var (
		scan    = bufio.NewScanner(f)
		blocked []string
	)
	for scan.Scan() {
		blocked = append(blocked, scan.Text())
	}
	if err := scan.Err(); err != nil {
		return blocklist{}, err
	}
	return newBlocklist(name, blocked), nil
}

// name implements blocker.
func (s blocklist) name() string { return s.title }

// ready implements blocker, although a static list is always ready.
func (blocklist) ready() bool { return true }

// block implements blocker.
func (s blocklist) block(host string) bool {
	if len(s.blocked) == 0 {
		return false
	}

	i, ok := slices.BinarySearchFunc(s.blocked, host, backwardsStringCompare)
	if ok {
		// Exact match, easy.
		return true
	}
	// i is where host would go in the list. We already know it's not an
	// exact match, so it can only be blocked if it's a subdomain of
	// something in the list, which must be something that would sort
	// immediately in front of host. So take one off i. Unless it's zero in
	// which case it can't possibly be a subdomain of anything in the list.
	if i == 0 {
		return false
	}
	i--
	return dns.IsSubDomain(s.blocked[i], host)
}

// backwardsStringCompare compares strings byte-wise, starting from their last
// character and working towards the beginning. In the usual fashion, returns a
// negative number if a < b, a positive number if b < a and zero if they're
// equal.
// TODO: this works but it's ugly, do better
func backwardsStringCompare(a, b string) int {
	switch nA, nB := len(a), len(b); {
	case nA == 0 && nB == 0:
		return 0
	case nA == 0 && nB != 0:
		// "" < anything
		return -1
	case nA != 0 && nB == 0:
		// anything > ""
		return 1
	}
	// Neither is empty.
	var (
		i = len(a) - 1
		j = len(b) - 1
	)
	for i >= 0 && j >= 0 {
		if c := int(a[i]) - int(b[j]); c != 0 {
			return signum[int](c)
		}
		i--
		j--
	}
	// They're equivalent so far, check if one is longer than the other.
	if i == -1 && j == -1 {
		return 0
	}
	if i == -1 {
		// a is shorter than b, so a < b
		return -1
	}
	if j == -1 {
		return 1
	}
	panic("unreachable")
}

func signum[E constraints.Signed](i int) int { return min(1, max(-1, i)) }
