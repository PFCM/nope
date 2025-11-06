package nope

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// dynamicBlocklist is a blocklist that regularly updates its contents by
// downloading an ABP file over HTTP. The blocklist file is also stored locally,
// and must be present at start up to avoid bootstrapping problems.
type dynamicBlocklist struct {
	listName string // short name for metrics
	source   string // upstream URL to fetch.
	path     string // where to read/write the list to storage

	// blocklistMu controls access to blocklist. It only controls the field,
	// not the contents as they are read-only. This could be an RWMutex, but
	// that's usually slower.
	// TODO: benchmarks to prove that?
	blocklistMu sync.Mutex
	blocklist   blocklist
}

func newDynamicBlocklist(name, source, path string) (*dynamicBlocklist, func(), error) {
	// read initial list from disk
	log.Infof("Reading initial list from %q", path)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	hosts, err := readABPFile(f)
	if err != nil {
		return nil, nil, err
	}
	d := &dynamicBlocklist{
		listName: name,
		source:   source,
		path:     path,

		blocklist: newBlocklist(name, hosts),
	}

	// start background loop for updates
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// TODO: configure somehow
		const period = time.Hour
		for {
			delay := time.Duration(float64(period) * (0.9 + rand.Float64()*0.2))
			log.Infof("Waiting %v to refresh blocklist", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if err := d.update(ctx); err != nil {
				log.Error(err)
			}
		}
	}()
	return d, cancel, nil
}

func (d *dynamicBlocklist) getList() blocklist {
	d.blocklistMu.Lock()
	defer d.blocklistMu.Unlock()
	return d.blocklist
}

func (d *dynamicBlocklist) name() string { return d.listName }

func (d *dynamicBlocklist) ready() bool {
	bl := d.getList()
	return bl.ready()
}

func (d *dynamicBlocklist) block(host string) bool {
	return d.getList().block(host)
}

// update updates the blocklist from its remote source.
// TODO: cache control? if-modified-since etags etc etc
// TODO: log a bit more
// TODO: test this
func (d *dynamicBlocklist) update(ctx context.Context) error {
	getCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(getCtx, http.MethodGet, d.source, nil)
	if err != nil {
		return err
	}

	log.Debugf("fetching %q", req.URL)
	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status from %q: %v", d.source, resp.Status)
	}

	// Read it into memory and check it parses before writing to file.
	var b bytes.Buffer
	if _, err := io.Copy(&b, resp.Body); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	hosts, err := readABPFile(bytes.NewReader(b.Bytes()))
	if err != nil {
		return err
	}
	log.Debugf("fetched %d hosts in %v", len(hosts), time.Since(t0))
	newList := newBlocklist(d.listName, hosts)
	// Seems good, write it.
	if err := os.WriteFile(d.path, b.Bytes(), 0777); err != nil {
		return err
	}

	d.blocklistMu.Lock()
	d.blocklist = newList
	d.blocklistMu.Unlock()

	return nil
}

// TODO: read more?
// TODO: tests
func readABPFile(r io.Reader) ([]string, error) {
	var (
		scan  = bufio.NewScanner(r)
		hosts []string
	)
	for scan.Scan() {
		l := scan.Text()
		if !strings.HasPrefix(l, "||") {
			continue
		}
		if !strings.HasSuffix(l, "^") {
			continue
		}
		hosts = append(hosts, strings.TrimPrefix(strings.TrimSuffix(l, "^"), "||"))
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}
