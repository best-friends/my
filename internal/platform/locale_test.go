// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/platform"
)

// TestDefaultLocaleAppliesWithoutRestart checks that every HTTP surface resolves the locale
// of a request without Accept-Language from the current default_locale setting.
func TestDefaultLocaleAppliesWithoutRestart(t *testing.T) {
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	h := app.Handler()

	paths := map[string]string{
		"platform problem":   "/nope",
		"client API problem": "/api/v1/nope",
		"sign-in screen":     authpages.PathSignIn,
	}
	check := func(want string, header map[string]string) {
		t.Helper()
		for name, path := range paths {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			for k, v := range header {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if got := w.Header().Get("Content-Language"); got != want {
				t.Errorf("%s (%s, %v): Content-Language = %q, want %q", name, path, header, got, want)
			}
		}
	}

	check("en", nil)
	if _, err := app.Instance.SetSetting(t.Context(), appinstance.SetSettingInput{Key: instance.SettingDefaultLocale, Value: "ja"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	check("ja", nil)
	check("en", map[string]string{"Accept-Language": "en-US"})
	check("ja", map[string]string{"Accept-Language": "fr"})
}
