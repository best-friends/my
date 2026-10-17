// SPDX-License-Identifier: AGPL-3.0-only

package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/pkg/testkit"
)

func TestContract(t *testing.T) {
	testkit.HTTPClientContract(t, func(_ *testing.T, n testkit.HTTPClientNetwork) kernel.HTTPClient {
		return New(Options{
			UserAgent:       "my/test (+https://social.example/)",
			AllowHTTP:       n.AllowHTTP,
			Ports:           n.Ports,
			RootCAs:         n.RootCAs,
			LookupNetIP:     n.LookupNetIP,
			ExemptAddrPorts: n.Exempt,
		})
	})
}

func TestAllowedAddr(t *testing.T) {
	for _, a := range []string{"8.8.8.8", "1.1.1.1", "172.32.0.1", "100.128.0.1", "2606:4700:4700::1111", "2001:4860:4860::8888", "64:ff9b::808:808", "2002:808:808::1", "::ffff:8.8.8.8"} {
		if !allowedAddr(netip.MustParseAddr(a)) {
			t.Errorf("allowedAddr(%s) = false, want true", a)
		}
	}
	for _, a := range []string{"10.0.0.1", "fe80::1%eth0", "2606:4700::1%1", "64:ff9b::a00:1", "::ffff:192.168.0.1"} {
		if allowedAddr(netip.MustParseAddr(a)) {
			t.Errorf("allowedAddr(%s) = true, want false", a)
		}
	}
}

func TestParseHostAddr(t *testing.T) {
	for host, want := range map[string]string{
		"2130706433":          "127.0.0.1",
		"0x7f000001":          "127.0.0.1",
		"017700000001":        "127.0.0.1",
		"0x7f.1":              "127.0.0.1",
		"127.1":               "127.0.0.1",
		"127.0.1":             "127.0.0.1",
		"0xa9.0xfe.0xa9.0xfe": "169.254.169.254",
		"8.8.8.8":             "8.8.8.8",
		"8.8.8.8.":            "8.8.8.8",
		"::1":                 "::1",
		"0":                   "0.0.0.0",
	} {
		a, numeric, ok := parseHostAddr(host)
		if !ok || !numeric || a.String() != want {
			t.Errorf("parseHostAddr(%q) = %v, %v, %v; want %s", host, a, numeric, ok, want)
		}
	}
	for _, host := range []string{"256.0.0.1", "1.2.3.4.5", "1.2.3.256", "4294967296", "1.08", "example.1x"} {
		if _, numeric, ok := parseHostAddr(host); ok || (!numeric && host != "example.1x") {
			t.Errorf("parseHostAddr(%q) = numeric %v, ok %v; want a malformed address", host, numeric, ok)
		}
	}
	for _, host := range []string{"social.example", "1.example", "example.com", "0x.example"} {
		if _, numeric, _ := parseHostAddr(host); numeric {
			t.Errorf("parseHostAddr(%q) is numeric, want a name", host)
		}
	}
}

func TestIgnoresProxyEnvironment(t *testing.T) {
	c := New(Options{})
	if tr := c.http.Transport.(*http.Transport); tr.Proxy != nil {
		t.Error("Transport.Proxy is set; proxy environment variables must be ignored")
	}
}

func TestAllowPrivateNetworks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	t.Cleanup(srv.Close)
	req := &kernel.HTTPRequest{URL: srv.URL, Purpose: kernel.HTTPPurposeJSON}
	if _, err := New(Options{AllowHTTP: true, AnyPort: true}).Do(t.Context(), req); !errors.Is(err, kernel.ErrForbiddenDestination) {
		t.Fatalf("Do(loopback) = %v, want ErrForbiddenDestination", err)
	}
	resp, err := New(Options{AllowHTTP: true, AnyPort: true, AllowPrivateNetworks: true}).Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do(loopback) with AllowPrivateNetworks = %v", err)
	}
	if b, err := resp.ReadAll(); err != nil || string(b) != "ok" {
		t.Errorf("body = %q, %v", b, err)
	}
}

func TestTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		<-block
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })
	c := New(Options{
		AllowHTTP: true, AnyPort: true, AllowPrivateNetworks: true,
		Limits: map[kernel.HTTPPurpose]kernel.HTTPLimits{kernel.HTTPPurposeJSON: {Timeout: 100 * time.Millisecond}},
	})
	resp, err := c.Do(t.Context(), &kernel.HTTPRequest{URL: srv.URL, Purpose: kernel.HTTPPurposeJSON})
	if err != nil {
		t.Fatalf("Do() = %v", err)
	}
	start := time.Now()
	if _, err := resp.ReadAll(); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Errorf("ReadAll() = %v, want the deadline", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("ReadAll() took %v", d)
	}
}
