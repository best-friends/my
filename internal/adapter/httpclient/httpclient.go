// SPDX-License-Identifier: AGPL-3.0-only

// Package httpclient is the SSRF-guarded kernel.HTTPClient (ADR-0019 §1).
//
// The destination is checked twice. Before a request and on every redirect hop, the URL's
// scheme, port, and host name are checked, and a host that is an IP address (in any notation
// inet_aton accepts) is checked as an address. Then, right before each connection, the
// net.Dialer's Control hook checks the IP address and port that the socket actually connects
// to. Name resolution results are never trusted on their own, so DNS rebinding cannot reach
// an internal address.
package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/best-friends/my/internal/adapter/telemetry/sysmetrics"
	"github.com/best-friends/my/internal/app/kernel"
)

// Transport timeouts and limits of ADR-0019 §1.
const (
	connectTimeout        = 5 * time.Second
	tlsHandshakeTimeout   = 5 * time.Second
	responseHeaderTimeout = 10 * time.Second
	idleConnTimeout       = 90 * time.Second
	maxConnsPerHost       = 8
	maxResponseHeaderSize = 64 << 10
)

// DefaultPorts are the ports a URL may name unless Options.AnyPort is set.
var DefaultPorts = []int{443, 80}

// Options configure a Client. The zero value is the production configuration.
type Options struct {
	// UserAgent is sent with every request; the default is "my".
	UserAgent string
	// Limits overrides kernel.DefaultHTTPLimits for some purposes.
	Limits map[kernel.HTTPPurpose]kernel.HTTPLimits
	// AllowHTTP permits the http scheme besides https. A redirect from https to http is still refused.
	AllowHTTP bool
	// Ports lists the ports a URL may name; the default is DefaultPorts.
	Ports []int
	// AnyPort permits every port.
	AnyPort bool
	// AllowPrivateNetworks turns the address and host name checks off, for development against
	// servers on loopback or a private network (ADR-0019 §1, http_allow_private_networks).
	AllowPrivateNetworks bool
	// RootCAs replaces the system certificate pool, for a private CA in tests.
	RootCAs *x509.CertPool
	// LookupNetIP resolves host names; the default is net.DefaultResolver.LookupNetIP.
	LookupNetIP func(ctx context.Context, host string) ([]netip.Addr, error)
	// ExemptAddrPorts are treated as public addresses. Tests set it to reach their servers on
	// loopback with every other check on; the server never sets it.
	ExemptAddrPorts []netip.AddrPort
	// Meter records the outbound HTTP metrics of ADR-0056 §1; nil records nothing.
	Meter kernel.Meter
}

// Client is the kernel.HTTPClient adapter.
type Client struct {
	opts   Options
	dialer *net.Dialer
	http   *http.Client

	// duration is nil when opts.Meter records nothing; conns is then nil too.
	duration kernel.Histogram
	conns    *sysmetrics.ConnCounter
}

var _ kernel.HTTPClient = (*Client)(nil)

// New returns a Client.
func New(opts Options) *Client {
	if opts.UserAgent == "" {
		opts.UserAgent = "my"
	}
	if opts.Ports == nil {
		opts.Ports = DefaultPorts
	}
	if opts.LookupNetIP == nil {
		opts.LookupNetIP = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	c := &Client{opts: opts}
	if !kernel.IsNop(opts.Meter) {
		c.duration = opts.Meter.Histogram(sysmetrics.HTTPClientRequestDuration)
		c.conns = sysmetrics.NewOpenConnCounter(opts.Meter, sysmetrics.HTTPClientOpenConnections)
	}
	c.dialer = &net.Dialer{Timeout: connectTimeout, Control: c.control}
	c.http = &http.Client{
		Transport: &http.Transport{
			// Proxy is nil: proxy environment variables are ignored (ADR-0019 §1).
			Proxy:                  nil,
			DialContext:            c.dial,
			ForceAttemptHTTP2:      true,
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: opts.RootCAs},
			TLSHandshakeTimeout:    tlsHandshakeTimeout,
			ResponseHeaderTimeout:  responseHeaderTimeout,
			ExpectContinueTimeout:  time.Second,
			IdleConnTimeout:        idleConnTimeout,
			MaxIdleConns:           100,
			MaxConnsPerHost:        maxConnsPerHost,
			MaxResponseHeaderBytes: maxResponseHeaderSize,
		},
		CheckRedirect: c.checkRedirect,
	}
	return c
}

// Do implements kernel.HTTPClient.
func (c *Client) Do(ctx context.Context, r *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	if c.duration == nil {
		return c.do(ctx, r)
	}
	start := time.Now()
	resp, err := c.do(ctx, r)
	c.record(ctx, r, resp, err, time.Since(start))
	return resp, err
}

func (c *Client) do(ctx context.Context, r *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	limits, ok := c.limits(r.Purpose)
	if !ok {
		return nil, fmt.Errorf("%w: unknown purpose %d", kernel.ErrInvalidHTTPRequest, r.Purpose)
	}
	u, err := url.Parse(r.URL)
	if err != nil || !u.IsAbs() {
		return nil, fmt.Errorf("%w: the URL must be absolute", kernel.ErrInvalidHTTPRequest)
	}
	if err := c.checkURL(u); err != nil {
		return nil, err
	}
	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if r.Body != nil {
		body = bytes.NewReader(r.Body)
	}
	ctx, cancel := context.WithTimeout(ctx, limits.Timeout)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%w: %w", kernel.ErrInvalidHTTPRequest, err)
	}
	if r.Header != nil {
		req.Header = r.Header.Clone()
	}
	// The transport asks for gzip and decompresses it only when the caller does not set
	// Accept-Encoding, and the body limit must apply to the decompressed size.
	req.Header.Del("Accept-Encoding")
	req.Header.Set("User-Agent", c.opts.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.ContentLength > limits.MaxBodyBytes {
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("%w: Content-Length %d exceeds %d bytes", kernel.ErrResponseTooLarge, resp.ContentLength, limits.MaxBodyBytes)
	}
	return &kernel.HTTPResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		URL:        resp.Request.URL.String(),
		Body:       &limitedBody{r: resp.Body, limit: limits.MaxBodyBytes, remaining: limits.MaxBodyBytes, cancel: cancel},
	}, nil
}

func (c *Client) limits(p kernel.HTTPPurpose) (kernel.HTTPLimits, bool) {
	l, ok := kernel.DefaultHTTPLimits(p)
	if !ok {
		return l, false
	}
	if o, ok := c.opts.Limits[p]; ok {
		if o.MaxBodyBytes > 0 {
			l.MaxBodyBytes = o.MaxBodyBytes
		}
		if o.Timeout > 0 {
			l.Timeout = o.Timeout
		}
	}
	return l, true
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > kernel.MaxRedirects {
		return fmt.Errorf("%w: more than %d", kernel.ErrTooManyRedirects, kernel.MaxRedirects)
	}
	if via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("%w: redirect from https to %s", kernel.ErrForbiddenDestination, req.URL.Scheme)
	}
	return c.checkURL(req.URL)
}

// checkURL checks the scheme, port, and host of u; a host that is an IP address is checked
// as an address here, and every address is checked again when connecting.
func (c *Client) checkURL(u *url.URL) error {
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && c.opts.AllowHTTP:
	default:
		return fmt.Errorf("%w: scheme %q", kernel.ErrForbiddenDestination, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: the URL must not contain user info", kernel.ErrInvalidHTTPRequest)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: the URL has no host", kernel.ErrInvalidHTTPRequest)
	}
	port := 443
	if u.Scheme == "http" {
		port = 80
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("%w: port %q", kernel.ErrInvalidHTTPRequest, p)
		}
		port = n
	}
	if !c.opts.AnyPort && !slices.Contains(c.opts.Ports, port) {
		return fmt.Errorf("%w: port %d", kernel.ErrForbiddenDestination, port)
	}
	if c.opts.AllowPrivateNetworks {
		return nil
	}
	addr, numeric, ok := parseHostAddr(host)
	switch {
	case ok:
		if !c.allowedAddrPort(netip.AddrPortFrom(addr, uint16(port))) { // #nosec G115 -- port is 1 to 65535
			return fmt.Errorf("%w: address %s", kernel.ErrForbiddenDestination, addr)
		}
	case numeric:
		return fmt.Errorf("%w: malformed IP address %q", kernel.ErrInvalidHTTPRequest, host)
	case deniedHostName(strings.TrimSuffix(strings.ToLower(host), ".")):
		return fmt.Errorf("%w: host %q", kernel.ErrForbiddenDestination, host)
	}
	return nil
}

// dial resolves the host of address and connects to its addresses in turn; control checks
// each address right before the connection.
func (c *Client) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("%w: port %q", kernel.ErrInvalidHTTPRequest, portStr)
	}
	var addrs []netip.Addr
	if a, numeric, ok := parseHostAddr(host); ok {
		addrs = []netip.Addr{a}
	} else if numeric {
		return nil, fmt.Errorf("%w: malformed IP address %q", kernel.ErrInvalidHTTPRequest, host)
	} else if addrs, err = c.opts.LookupNetIP(ctx, host); err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	var errs []error
	for _, a := range addrs {
		conn, err := c.dialer.DialContext(ctx, network, netip.AddrPortFrom(a, uint16(port)).String())
		if err == nil {
			return c.conns.Track(ctx, conn), nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

// control runs right before each connection with the address the socket connects to.
func (c *Client) control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: unparsable address %q", kernel.ErrForbiddenDestination, address)
	}
	if !c.allowedAddrPort(ap) {
		return fmt.Errorf("%w: address %s", kernel.ErrForbiddenDestination, ap.Addr())
	}
	return nil
}

func (c *Client) allowedAddrPort(ap netip.AddrPort) bool {
	if c.opts.AllowPrivateNetworks {
		return true
	}
	unmapped := netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
	if slices.Contains(c.opts.ExemptAddrPorts, unmapped) {
		return true
	}
	return allowedAddr(ap.Addr())
}

// limitedBody fails reads past limit with kernel.ErrResponseTooLarge and ends the request's
// timeout when closed.
type limitedBody struct {
	r         io.ReadCloser
	limit     int64
	remaining int64
	cancel    context.CancelFunc
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining < 0 {
		return 0, b.tooLarge()
	}
	if int64(len(p)) > b.remaining+1 {
		p = p[:b.remaining+1]
	}
	n, err := b.r.Read(p)
	if int64(n) > b.remaining {
		n = int(b.remaining)
		b.remaining = -1
		return n, b.tooLarge()
	}
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBody) tooLarge() error {
	return fmt.Errorf("%w: the body exceeds %d bytes", kernel.ErrResponseTooLarge, b.limit)
}

func (b *limitedBody) Close() error {
	err := b.r.Close()
	b.cancel()
	return err
}
