// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// viewerKey is the context key of the fake viewer: the saved locale of the user that a
// request is authenticated as.
type viewerKey struct{}

func withViewer(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, viewerKey{}, locale)
}

func viewerLocale(ctx context.Context) string {
	l, _ := ctx.Value(viewerKey{}).(string)
	return l
}

func TestResolverLocale(t *testing.T) {
	b := load(t)
	tests := []struct {
		name            string
		viewer          string // "" is an anonymous request
		acceptLanguage  string
		instanceDefault string
		want            string
	}{
		{"nothing", "", "", "", "en"},
		{"user setting wins", "ja", "en", "en", "ja"},
		{"user setting wins over the instance default", "en", "", "ja", "en"},
		{"accept-language", "", "fr;q=0.9, ja-JP;q=0.8, en;q=0.1", "", "ja"},
		{"accept-language before instance default", "", "en-GB", "ja", "en"},
		{"instance default without accept-language", "", "", "ja", "ja"},
		{"unsupported accept-language uses instance default", "", "fr, de", "ja", "ja"},
		{"unsupported user setting is skipped", "zh-Hant-TW", "", "ja", "ja"},
		{"unsupported instance default falls back to en", "", "fr", "de", "en"},
		{"malformed values are skipped", "!!", "*, ja_jp", "en", "ja"},
		{"malformed instance default falls back to en", "", "", "!!", "en"},
		{"pseudo-locales are not selectable by default", "en-XA", "", "ja", "en"},
	}
	for _, tt := range tests {
		r := &Resolver{
			Bundle:          b,
			UserLocale:      viewerLocale,
			InstanceDefault: func(context.Context) string { return tt.instanceDefault },
		}
		ctx := t.Context()
		if tt.viewer != "" {
			ctx = withViewer(ctx, tt.viewer)
		}
		if got := r.Locale(ctx, tt.acceptLanguage); got != tt.want {
			t.Errorf("%s: Locale(viewer %q, %q) with default %q = %q, want %q",
				tt.name, tt.viewer, tt.acceptLanguage, tt.instanceDefault, got, tt.want)
		}
	}

	pseudo := &Resolver{Bundle: load(t, WithPseudoLocales())}
	if got := pseudo.Locale(t.Context(), "ar-XB"); got != "ar-XB" {
		t.Errorf("Locale(ar-XB) with pseudo-locales = %q", got)
	}
	if got := (&Resolver{Bundle: b}).Locale(t.Context(), ""); got != "en" {
		t.Errorf("Locale without hooks = %q, want en", got)
	}
}

// TestResolverReadsInstanceDefaultPerRequest checks that a changed default applies to the
// next request, and that the default is not read when an earlier step decides.
func TestResolverReadsInstanceDefaultPerRequest(t *testing.T) {
	current, reads := "en", 0
	r := &Resolver{Bundle: load(t), InstanceDefault: func(context.Context) string { reads++; return current }}

	if got := r.Locale(t.Context(), ""); got != "en" {
		t.Fatalf("Locale = %q, want en", got)
	}
	current = "ja"
	if got := r.RequestLocalizer(httptest.NewRequest(http.MethodGet, "/", nil)).Locale(); got != "ja" {
		t.Errorf("Locale after changing the default = %q, want ja", got)
	}
	reads = 0
	if got := r.Locale(t.Context(), "en"); got != "en" || reads != 0 {
		t.Errorf("Locale(en) = %q with %d reads of the default, want en with none", got, reads)
	}
}

func TestResolverUserLocalizer(t *testing.T) {
	r := &Resolver{Bundle: load(t), InstanceDefault: func(context.Context) string { return "ja" }}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en")
	for user, want := range map[string]string{"ja": "ja", "": "en", "zh-Hant-TW": "en", "!!": "en"} {
		if got := r.UserLocalizer(req, user).Locale(); got != want {
			t.Errorf("UserLocalizer(Accept-Language en, user %q) = %q, want %q", user, got, want)
		}
	}
}
