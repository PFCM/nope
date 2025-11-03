package nope

import (
	"fmt"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() { plugin.Register("nope", setup) }

func setup(c *caddy.Controller) error {
	n, err := parseNope(c)
	if err != nil {
		return err
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		n.next = next
		return n
	})
	return nil
}

func parseNope(c *caddy.Controller) (*Nope, error) {
	var blockers []blocker
	for c.Next() {
		inline := c.RemainingArgs()
		if len(inline) > 0 {
			blockers = append(blockers, newBlocklist("inline_config", inline))
			fmt.Printf("Loaded inline blocklist: %q\n", inline)
		}
		for c.NextBlock() {
			switch v := c.Val(); v {
			case "static":
				args := c.RemainingArgs()
				switch n := len(args); {
				case n <= 1:
					return nil, c.ArgErr()
				case n == 2:
					b, err := newBlocklistFromFile(args[0], args[1])
					if err != nil {
						return nil, err
					}
					blockers = append(blockers, b)
					fmt.Printf("Loaded static %q: %q: %d hosts\n", args[0], args[1], len(b.blocked))
				default:
					blockers = append(blockers, newBlocklist(args[0], args[1:]))
					fmt.Printf("Loaded static %q: %q\n", args[0], args[1:])
				}

			case "dynamic":
				var name, source, path string
				if !c.Args(&name, &source, &path) {
					return nil, c.ArgErr()
				}
				b, cancel, err := newDynamicBlocklist(name, source, path)
				if err != nil {
					return nil, err
				}
				c.OnShutdown(func() error {
					cancel()
					return nil
				})
				blockers = append(blockers, b)
				fmt.Printf("Loaded dynamic %q: %d hosts in %q, will fetch from %q\n", name, len(b.blocklist.blocked), path, source)
			default:
				return nil, c.Errf("unknown blocklist type %q", v)
			}
		}
	}
	if len(blockers) == 0 {
		return nil, c.Err("no blocklists configured")
	}
	return &Nope{
		blockers: blockers,
	}, nil
}
