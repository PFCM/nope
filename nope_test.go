package nope

import (
	"strings"
	"testing"
)

func TestBlocklist(t *testing.T) {
	for _, c := range []struct {
		list  []string
		query string
		want  bool
	}{{
		list:  []string{"exact-single.com"},
		query: "exact-single.com",
		want:  true,
	}, {
		list:  []string{"exact-double.com", "something-else.org"},
		query: "exact-double.com",
		want:  true,
	}, {
		list:  []string{"aaa.info", "bananas.com", "exact-last.org"},
		query: "exact-last.org",
		want:  true,
	}, {
		list:  []string{"a.com", "b.com", "c.com", "d.com"},
		query: "sub.b.com",
		want:  true,
	}, {
		list:  []string{"a.com", "b.com", "c.com", "d.com"},
		query: "sub.a.com",
		want:  true,
	}, {
		list:  []string{"b.com", "c.com", "d.com"},
		query: "a.com",
		want:  false,
	}, {
		list:  []string{"a.com", "c.com", "d.com"},
		query: "b.com",
		want:  false,
	}, {
		list:  []string{"a.com", "b.com", "c.com", "d.com"},
		query: "e.com",
		want:  false,
	}, {
		list:  []string{"sub.a.com", "sub2.a.com", "sub.b.com", "c.com", "d.com"},
		query: "sub.sub.a.com",
		want:  true,
	}, {
		list:  []string{"sub.a.com", "sub2.a.com", "sub.b.com", "c.com", "d.com"},
		query: "sub2b.a.com",
		want:  false,
	}} {
		t.Run(c.query, func(t *testing.T) {
			sbl := newBlocklist("test", c.list)

			if got := sbl.block(c.query); got != c.want {
				t.Fatalf("unexpected result: got: %v, want: %v\nblocklist: %q", got, c.want, sbl)
			}
		})
	}
}

func TestBackwardsStringCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{
		// No surprises with the empty string.
		a:    "",
		b:    "",
		want: 0,
	}, {
		a:    "",
		b:    "not empty",
		want: -1,
	}, {
		// single character strings should behave exactly as usual.
		a:    "a",
		b:    "b",
		want: -1,
	}, {
		a:    "b",
		b:    "c",
		want: -1,
	}, {
		a:    "a",
		b:    "c",
		want: -1,
	}, {
		a:    "a",
		b:    "a",
		want: 0,
	}, {
		// First difference determines the order.
		a:    "edcba",
		b:    "fdcba",
		want: -1,
	}, {
		// Shorter strings, that are otherwise equivalent from the back,
		// sort first.
		a:    "ba",
		b:    "a",
		want: 1,
	}, {
		a:    "subdomain.example.com",
		b:    "example.com",
		want: 1,
	}} {
		t.Run(strings.Join([]string{c.a, c.b}, "/"), func(t *testing.T) {
			if got := backwardsStringCompare(c.a, c.b); got != c.want {
				t.Fatalf("backwardsStringCompare(%q, %q) = %v, want: %v", c.a, c.b, got, c.want)
			}
		})
		t.Run(strings.Join([]string{c.b, c.a}, "/"), func(t *testing.T) {
			want := -c.want
			if got := backwardsStringCompare(c.b, c.a); got != want {
				t.Fatalf("backwardsStringCompare(%q, %q) = %v, want: %v", c.b, c.a, got, want)
			}
		})
	}
}
