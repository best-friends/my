// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/platform"
)

// TestHTTPClientGuard checks that the App's HTTPClient refuses a loopback server by default
// and reaches it, with a warning at start, when http_allow_private_networks is on.
func TestHTTPClientGuard(t *testing.T) {
	t.Parallel()
	var ua atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua.Store(r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	req := &kernel.HTTPRequest{URL: srv.URL, Purpose: kernel.HTTPPurposeJSON}
	const warning = "http_allow_private_networks is on"

	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{name: "default"},
		{name: "private networks", env: map[string]string{"MY_PROFILE": "development", "MY_HTTP_ALLOW_PRIVATE_NETWORKS": "true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"MY_DATABASE_URL": newDatabase(t, false)}
			for k, v := range tc.env {
				env[k] = v
			}
			logs := &syncBuffer{}
			app, err := platform.NewApp(t.Context(), loadConfig(t, env), platform.AppOptions{Logger: platform.NewLogger(logs, slog.LevelDebug), Version: "1.2.3"})
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			t.Cleanup(func() { _ = app.Close() })
			resp, err := app.HTTPClient.Do(t.Context(), req)
			if tc.env == nil {
				if !errors.Is(err, kernel.ErrForbiddenDestination) {
					t.Errorf("Do(loopback) = %v, want ErrForbiddenDestination", err)
				}
				if strings.Contains(logs.String(), warning) {
					t.Errorf("logs warn about private networks: %s", logs.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("Do(loopback) = %v", err)
			}
			_, _ = resp.ReadAll()
			if want := "my/1.2.3 (+https://example.test/)"; ua.Load() != want {
				t.Errorf("User-Agent = %q, want %q", ua, want)
			}
			if !strings.Contains(logs.String(), warning) {
				t.Errorf("logs do not warn about private networks: %s", logs.String())
			}
		})
	}
}
