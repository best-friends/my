// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/uow"
	domain "github.com/best-friends/my/internal/domain/content"
	domaininstance "github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/pkg/testkit"
)

// remoteServer serves a PNG at /a.png and an SVG declared as a PNG at /b.png, counting the
// requests.
func remoteServer(t *testing.T) (*httptest.Server, *atomic.Int32, []byte) {
	t.Helper()
	png := testkit.FixturePNG(4, 3, false)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		switch r.URL.Path {
		case "/a.png":
			_, _ = w.Write(png)
		case "/b.png":
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, png
}

// recordRemote records a remote media file of the instance actor at remoteURL and returns the
// path and query of its proxy URL.
func recordRemote(t *testing.T, app *platform.App, remoteURL string) string {
	t.Helper()
	owner, err := app.Accounts.EnsureInstanceActor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var m domain.RemoteMedia
	err = app.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		m, err = app.Media.RecordRemoteMedia(t.Context(), tx, content.RemoteMediaInput{
			OwnerAccountID: owner.ID, RemoteURL: remoteURL, DeclaredMediaType: domain.MediaTypeImage,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(app.MediaProxy.URL(m.MediaID, domain.VariantFull))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "example.test" {
		t.Errorf("proxy URL = %s, want one on the instance's origin", u)
	}
	return u.RequestURI()
}

// The media proxy relays a recorded remote file with the detected type and the cache TTL of
// the instance settings, and refuses unsigned or tampered URLs and SVG content.
func TestMediaProxy(t *testing.T) {
	remote, _, png := remoteServer(t)
	cfg := loadConfig(t, map[string]string{
		"MY_DATABASE_URL": newDatabase(t, true), "MY_PROFILE": "development", "MY_HTTP_ALLOW_PRIVATE_NETWORKS": "true",
	})
	s := start(t, cfg, platform.ServeOptions{}, nil)
	path := recordRemote(t, s.app, remote.URL+"/a.png")

	resp, body := s.do(t, http.MethodGet, path, nil)
	if resp.StatusCode != http.StatusOK || body != string(png) {
		t.Fatalf("GET %s = %d, %d bytes", path, resp.StatusCode, len(body))
	}
	for k, want := range map[string]string{
		"Content-Type":                 "image/png",
		"X-Content-Type-Options":       "nosniff",
		"Content-Security-Policy":      "default-src 'none'; sandbox",
		"Cross-Origin-Resource-Policy": "cross-origin",
		"Cache-Control":                "public, max-age=86400",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	if _, err := s.app.Instance.SetSetting(t.Context(), instance.SetSettingInput{Key: domaininstance.SettingRemoteMediaCacheTTL, Value: "3600"}); err != nil {
		t.Fatal(err)
	}
	if resp, _ := s.do(t, http.MethodGet, path, nil); resp.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q after setting remote_media_cache_ttl", resp.Header.Get("Cache-Control"))
	}

	unsigned, _, _ := strings.Cut(path, "?")
	last := "A"
	if strings.HasSuffix(path, "A") {
		last = "B"
	}
	tampered := path[:len(path)-1] + last
	for _, p := range []string{unsigned, tampered, recordRemote(t, s.app, remote.URL+"/b.png"), recordRemote(t, s.app, remote.URL+"/missing.png")} {
		if resp, _ := s.do(t, http.MethodGet, p, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, resp.StatusCode)
		}
	}
}

// Without http_allow_private_networks, the proxy's fetch of an internal address is refused by
// the SSRF guard: the client gets 404 and the internal server sees no request.
func TestMediaProxySSRFGuard(t *testing.T) {
	remote, hits, _ := remoteServer(t)
	s := start(t, loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)}), platform.ServeOptions{}, nil)
	internal := strings.Replace(remote.URL, "http://", "https://", 1) + "/a.png"
	for _, target := range []string{internal, remote.URL + "/a.png", "https://localhost/a.png", "https://169.254.169.254/latest/meta-data"} {
		path := recordRemote(t, s.app, target)
		if resp, _ := s.do(t, http.MethodGet, path, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET proxy of %s = %d, want 404", target, resp.StatusCode)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the internal server got %d requests", n)
	}
}
