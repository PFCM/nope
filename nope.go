// package nope is a blocklist plugin for CoreDNS.
package nope

import (
	"context"
	"fmt"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var log = clog.NewWithPlugin("nope")

// Nope manages a blocklist and passes on or denies DNS requests.
type Nope struct {
	next plugin.Handler
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
	host := r.Question[0].Name
	log.Infof("question.name: %q", host)

	// pass it along.
	return plugin.NextOrFailure(n.Name(), n.next, ctx, w, r)
}

// requestCount exports a prometheus metric that tracks blocked/allowed requests.
// TODO: adding the host as a label is a bit silly, stop it.
var requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: plugin.Namespace,
	Subsystem: Nope{}.Name(),
	Name:      "request_count_total",
	Help:      "counter of requests made",
}, []string{"server", "host"})
