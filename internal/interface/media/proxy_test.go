// SPDX-License-Identifier: AGPL-3.0-only

package media_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/interface/media"
)

// fakeRemote opens the file "ok" (signature "good") and fails for everything else with err.
type fakeRemote struct {
	body   io.Reader
	length int64
	err    error
}

func (f fakeRemote) Open(_ context.Context, id, variant, sig string) (*content.ProxiedMedia, error) {
	if id != "ok" || variant != "full" || sig != "good" {
		return nil, fmt.Errorf("proxy: %w", content.ErrNotFound)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &content.ProxiedMedia{ContentType: "image/png", Length: f.length, MaxAge: 90 * time.Minute, Body: io.NopCloser(f.body)}, nil
}

func newProxyServer(r media.RemoteMedia) http.Handler {
	m := mux{http.NewServeMux()}
	media.NewProxy(media.ProxyOptions{Media: r}).Register(m)
	return m
}

func TestProxyRelaysWithSafeHeaders(t *testing.T) {
	h := newProxyServer(fakeRemote{body: strings.NewReader("\x89PNGdata"), length: 8})
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/media/proxy/ok/full?sig=good", nil))
		wantBody := "\x89PNGdata"
		if method == http.MethodHead {
			wantBody = ""
		}
		if rec.Code != http.StatusOK || rec.Body.String() != wantBody {
			t.Fatalf("%s = %d %q", method, rec.Code, rec.Body.String())
		}
		for k, want := range map[string]string{
			"Content-Type":                 "image/png",
			"Content-Length":               "8",
			"X-Content-Type-Options":       "nosniff",
			"Content-Security-Policy":      "default-src 'none'; sandbox",
			"Cross-Origin-Resource-Policy": "cross-origin",
			"Content-Disposition":          "inline",
			"Cache-Control":                "public, max-age=5400",
		} {
			if got := rec.Header().Get(k); got != want {
				t.Errorf("%s: %s = %q, want %q", method, k, got, want)
			}
		}
	}
}

func TestProxyErrors(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		err        error
		want       int
	}{
		{"no signature", "/media/proxy/ok/full", nil, http.StatusNotFound},
		{"bad signature", "/media/proxy/ok/full?sig=bad", nil, http.StatusNotFound},
		{"not relayed", "/media/proxy/ok/full?sig=good", fmt.Errorf("svg: %w", content.ErrNotFound), http.StatusNotFound},
		{"upstream down", "/media/proxy/ok/full?sig=good", fmt.Errorf("down: %w", content.ErrProxyUpstream), http.StatusBadGateway},
		{"internal", "/media/proxy/ok/full?sig=good", errors.New("database is gone"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newProxyServer(fakeRemote{err: tc.err}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.want {
				t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.want)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

// A file that turns out larger than the limit while it is relayed aborts the response.
func TestProxyAbortsOnTooLargeBodies(t *testing.T) {
	body := io.MultiReader(strings.NewReader("\x89PNGdata"), errReader{kernel.ErrResponseTooLarge})
	h := newProxyServer(fakeRemote{body: body, length: -1})
	defer func() {
		if p := recover(); p != http.ErrAbortHandler { //nolint:errorlint // the handler panics with the sentinel itself
			t.Errorf("panic = %v, want http.ErrAbortHandler", p)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/media/proxy/ok/full?sig=good", nil))
	t.Error("the response was not aborted")
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
