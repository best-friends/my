// SPDX-License-Identifier: AGPL-3.0-only

package kernel

import (
	"context"
	"net"
	"net/http"
)

type clientIPKey struct{}

// WithClientIP returns ctx carrying ip as the client IP of its HTTP request. The HTTP server
// resolves it once per request from the connection's peer and the trusted proxies' forwarding
// header (ADR-0019 §5).
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// RequestClientIP returns the client IP of r, the key of its IP rate limits (ClientKey): the
// address that WithClientIP put in its context, or else the connection's peer, so that a
// forwarding header is never read without the trusted proxy setting.
func RequestClientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok && ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
