package nope

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() { plugin.Register("nope", setup) }

func setup(c *caddy.Controller) error {
	// TODO: parse config
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return &Nope{next: next}
	})
	return nil
}
