// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/best-friends/my/internal/app/kernel"
)

// Forwarding headers that trusted proxies append the client IP to (config trusted_proxy_header).
const (
	headerXForwardedFor = "X-Forwarded-For"
	headerForwarded     = "Forwarded"
)

// clientIPResolver tells the client IP of a request (ADR-0019 §5): the connection's peer,
// unless the peer is a trusted proxy, in which case it is the rightmost address of the
// forwarding header that is not a trusted proxy. Walking from the right, every address up to
// that one was appended by a proxy we trust; anything to its left came from the client and may
// be forged.
type clientIPResolver struct {
	trusted []netip.Prefix
	// forwarded selects the for parameters of Forwarded (RFC 7239) instead of X-Forwarded-For.
	forwarded bool
}

func newClientIPResolver(trusted []netip.Prefix, header string) clientIPResolver {
	return clientIPResolver{trusted: trusted, forwarded: header == "forwarded"}
}

func (c clientIPResolver) trusts(a netip.Addr) bool {
	return slices.ContainsFunc(c.trusted, func(p netip.Prefix) bool { return p.Contains(a) })
}

// resolve returns the client IP of r. When the walk through the header reaches a value that is
// not an address (garbage, unknown, an obfuscated identifier, a missing for parameter), it stops
// at the trusted proxy that reported it, so that a malformed header never yields an address the
// client chose. When every address is trusted, it returns the leftmost one.
func (c clientIPResolver) resolve(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	client := normalizeAddr(peer)
	if len(c.trusted) == 0 || !c.trusts(client) {
		return client.String()
	}
	hops := c.hops(r.Header)
	for i := len(hops) - 1; i >= 0; i-- {
		a, ok := parseNode(hops[i])
		if !ok {
			break
		}
		client = a
		if !c.trusts(a) {
			break
		}
	}
	return client.String()
}

// hops returns the node of each hop in the configured header, left to right, across every field
// line in order (RFC 9110 §5.3).
func (c clientIPResolver) hops(h http.Header) []string {
	var hops []string
	if !c.forwarded {
		for _, line := range h.Values(headerXForwardedFor) {
			for v := range strings.SplitSeq(line, ",") {
				if v = strings.TrimSpace(v); v != "" {
					hops = append(hops, v)
				}
			}
		}
		return hops
	}
	for _, line := range h.Values(headerForwarded) {
		// The elements are split at every comma, quoted or not: a quoted comma belongs only in
		// parameters other than for, and honouring quotes would let a client's unbalanced quote
		// swallow the elements that the proxies append after it.
		for elem := range strings.SplitSeq(line, ",") {
			if elem = strings.TrimSpace(elem); elem != "" {
				hops = append(hops, forwardedFor(elem))
			}
		}
	}
	return hops
}

// forwardedFor returns the value of the for parameter of one Forwarded element, unquoted, or
// "" when it has none.
func forwardedFor(elem string) string {
	for pair := range strings.SplitSeq(elem, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "for") {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		return value
	}
	return ""
}

// parseNode parses one hop's node: an IPv4 or IPv6 address, bracketed or not, with an optional
// port, as X-Forwarded-For writers and RFC 7239 §6 write it.
func parseNode(v string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(v); err == nil {
		return normalizeAddr(a), true
	}
	if ap, err := netip.ParseAddrPort(v); err == nil {
		return normalizeAddr(ap.Addr()), true
	}
	if len(v) > 2 && v[0] == '[' && v[len(v)-1] == ']' {
		if a, err := netip.ParseAddr(v[1 : len(v)-1]); err == nil && a.Is6() {
			return normalizeAddr(a), true
		}
	}
	return netip.Addr{}, false
}

// normalizeAddr drops the zone and unmaps IPv4-mapped IPv6, so that one client has one address.
func normalizeAddr(a netip.Addr) netip.Addr { return a.WithZone("").Unmap() }

// clientIPs puts the client IP of each request in its context (kernel.WithClientIP), for the
// IP rate limits of the handlers behind it (kernel.RequestClientIP).
func (a *App) clientIPs(next http.Handler) http.Handler {
	c := newClientIPResolver(a.Config.TrustedProxies, a.Config.TrustedProxyHeader)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(kernel.WithClientIP(r.Context(), c.resolve(r))))
	})
}
