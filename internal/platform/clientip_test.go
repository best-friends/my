// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/platform/config"
)

func prefixes(ps ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ps))
	for i, p := range ps {
		out[i] = netip.MustParsePrefix(p)
	}
	return out
}

func TestClientIPResolver(t *testing.T) {
	t.Parallel()
	proxies := prefixes("10.0.0.0/8", "2001:db8:ffff::/48", "192.0.2.1/32")
	tests := []struct {
		name    string
		trusted []netip.Prefix
		header  string
		remote  string
		xff     []string
		fwd     []string
		want    string
	}{
		{name: "no trusted proxies ignore X-Forwarded-For", remote: "10.0.0.1:5000", xff: []string{"203.0.113.9"}, want: "10.0.0.1"},
		{name: "untrusted IPv4 peer's X-Forwarded-For is ignored", trusted: proxies, remote: "198.51.100.7:5000", xff: []string{"203.0.113.9"}, want: "198.51.100.7"},
		{name: "untrusted IPv6 peer's X-Forwarded-For is ignored", trusted: proxies, remote: "[2001:db8:1::7]:443", xff: []string{"203.0.113.9"}, want: "2001:db8:1::7"},
		{name: "untrusted peer's Forwarded is ignored", trusted: proxies, header: "forwarded", remote: "198.51.100.7:5000", fwd: []string{"for=203.0.113.9"}, want: "198.51.100.7"},
		{name: "trusted peer without a header", trusted: proxies, remote: "10.0.0.1:5000", want: "10.0.0.1"},
		{name: "one trusted IPv4 hop", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9"}, want: "203.0.113.9"},
		{name: "one trusted IPv6 hop", trusted: proxies, remote: "[2001:db8:ffff::1]:5000", xff: []string{"2001:db8:1::9"}, want: "2001:db8:1::9"},
		{name: "forged addresses left of the client are skipped", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"1.1.1.1, 10.9.9.9, 203.0.113.9"}, want: "203.0.113.9"},
		{name: "chain of trusted proxies", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9, 192.0.2.1, 10.0.0.2"}, want: "203.0.113.9"},
		{name: "multiple header lines are one list", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"1.1.1.1", "203.0.113.9, 10.0.0.2"}, want: "203.0.113.9"},
		{name: "a later header line is nearer", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9", "198.51.100.1"}, want: "198.51.100.1"},
		{name: "every hop trusted gives the leftmost", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"10.0.0.3, 10.0.0.2"}, want: "10.0.0.3"},
		{name: "addresses with ports and brackets", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"[2001:db8:1::9]:4711, 10.0.0.2:80"}, want: "2001:db8:1::9"},
		{name: "bracketed IPv6 without a port", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"[2001:db8:1::9]"}, want: "2001:db8:1::9"},
		{name: "IPv4-mapped IPv6 is unmapped", trusted: proxies, remote: "[::ffff:10.0.0.1]:5000", xff: []string{"::ffff:203.0.113.9"}, want: "203.0.113.9"},
		{name: "zones are dropped", trusted: prefixes("fe80::/10"), remote: "[fe80::1%eth0]:5000", xff: []string{"203.0.113.9"}, want: "203.0.113.9"},
		{name: "empty elements are skipped", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9,, ,"}, want: "203.0.113.9"},
		{name: "malformed rightmost value stops at the proxy", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9, garbage"}, want: "10.0.0.1"},
		{name: "malformed value behind a trusted hop stops at that hop", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"203.0.113.9, 999.1.1.1, 10.0.0.2"}, want: "10.0.0.2"},
		{name: "malformed value left of the client is not reached", trusted: proxies, remote: "10.0.0.1:5000", xff: []string{"garbage, 203.0.113.9"}, want: "203.0.113.9"},
		{name: "X-Forwarded-For setting ignores Forwarded", trusted: proxies, remote: "10.0.0.1:5000", fwd: []string{"for=203.0.113.9"}, want: "10.0.0.1"},
		{name: "Forwarded setting ignores X-Forwarded-For", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", xff: []string{"203.0.113.9"}, want: "10.0.0.1"},
		{name: "Forwarded for", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{"for=203.0.113.9;proto=https;by=10.0.0.1"}, want: "203.0.113.9"},
		{name: "Forwarded quoted IPv6 with port and upper-case name", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{`proto=https; For="[2001:db8:1::9]:4711"`}, want: "2001:db8:1::9"},
		{name: "Forwarded chain over multiple lines", trusted: proxies, header: "forwarded", remote: "[2001:db8:ffff::1]:5000", fwd: []string{"for=1.1.1.1, for=203.0.113.9", `for="[2001:db8:ffff::2]"`}, want: "203.0.113.9"},
		{name: "Forwarded unknown stops at the proxy", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{"for=203.0.113.9, for=unknown"}, want: "10.0.0.1"},
		{name: "Forwarded obfuscated node stops at the proxy", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{"for=_hidden"}, want: "10.0.0.1"},
		{name: "Forwarded element without for stops at the proxy", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{"for=203.0.113.9, by=10.0.0.2;proto=https"}, want: "10.0.0.1"},
		{name: "Forwarded unbalanced quote cannot swallow the proxy's element", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{`for="1.1.1.1, for=203.0.113.9`}, want: "203.0.113.9"},
		{name: "Forwarded unbalanced quote as the rightmost value", trusted: proxies, header: "forwarded", remote: "10.0.0.1:5000", fwd: []string{`for="203.0.113.9`}, want: "10.0.0.1"},
		{name: "remote address without a port", trusted: proxies, remote: "10.0.0.1", xff: []string{"203.0.113.9"}, want: "203.0.113.9"},
		{name: "remote address that is not an IP", trusted: proxies, remote: "@", xff: []string{"203.0.113.9"}, want: "@"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			for _, v := range tt.fwd {
				r.Header.Add("Forwarded", v)
			}
			header := tt.header
			if header == "" {
				header = "x-forwarded-for"
			}
			if got := newClientIPResolver(tt.trusted, header).resolve(r); got != tt.want {
				t.Errorf("resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIPsMiddleware(t *testing.T) {
	t.Parallel()
	a := &App{Config: &config.Config{TrustedProxies: prefixes("10.0.0.0/8"), TrustedProxyHeader: "x-forwarded-for"}}
	var got string
	h := a.clientIPs(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = kernel.RequestClientIP(r) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "203.0.113.9" {
		t.Errorf("RequestClientIP() behind the middleware = %q, want 203.0.113.9", got)
	}
	// Without the middleware, the peer is the client: the header is never read.
	if ip := kernel.RequestClientIP(r); ip != "10.0.0.1" {
		t.Errorf("RequestClientIP() without the middleware = %q, want 10.0.0.1", ip)
	}
}
