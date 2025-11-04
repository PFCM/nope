package nope

import (
	"testing"

	"github.com/coredns/caddy"
	"github.com/google/go-cmp/cmp"
)

func TestParseNope(t *testing.T) {
	for _, c := range []struct {
		name         string
		in           string
		wantErr      bool
		wantBlockers []blocker
	}{{
		name:    "empty",
		in:      "nope",
		wantErr: true,
	}, {
		name: "inline",
		in:   "nope example.com example.org",
		wantBlockers: []blocker{
			newBlocklist("inline_config", []string{"example.com", "example.org"}),
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			ctrl := caddy.NewTestController("dns", c.in)
			n, err := parseNope(ctrl)
			if err != nil {
				if !c.wantErr {
					t.Errorf("expected error, got: %v", n)
				}
				return
			}
			if d := cmp.Diff(
				n.blockers,
				c.wantBlockers,
				cmp.AllowUnexported(Nope{}, blocklist{}, dynamicBlocklist{}),
			); d != "" {
				t.Fatalf("unexpected blockers (-got, +want):\n%v", d)
			}
		})
	}
}
