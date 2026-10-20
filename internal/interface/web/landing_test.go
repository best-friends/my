// SPDX-License-Identifier: AGPL-3.0-only

package web_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/interface/web"
)

// TestLandingPreview checks that the landing page (/) previews the instance in its default
// locale: its name, its summary, and its preview image, without the meta tags of an account.
func TestLandingPreview(t *testing.T) {
	names := map[string]string{"ja": "例のサーバー", "en": "Example Social"}
	summary := "  Gardeners & friends\n" + strings.Repeat("あ", 250)
	_, mux := newHandler(t, web.Options{
		Settings:      crawlerSettings("allow", "allow"),
		DefaultLocale: locale("ja"),
		InstanceName:  func(_ context.Context, l string) string { return names[l] },
		InstanceDescription: func(_ context.Context, l string) string {
			if l != "ja" {
				t.Errorf("summary read for %q, want the default locale", l)
			}
			return summary
		},
		DefaultImage: func(context.Context, string) (*web.Image, error) {
			return &web.Image{URL: "https://social.example/media/p/full.png", Width: 1200, Height: 630}, nil
		},
	})
	rec := get(mux, "/", map[string]string{"Accept-Language": "en"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	got := metas(t, body)
	want := map[string]string{
		"og:type":           "website",
		"og:site_name":      "例のサーバー",
		"og:title":          "例のサーバー",
		"og:url":            "https://social.example/",
		"og:locale":         "ja",
		"og:image":          "https://social.example/media/p/full.png",
		"twitter:card":      "summary_large_image",
		"link:canonical":    "https://social.example/",
		"fediverse:creator": "",
		"link:alternate":    "",
		"robots":            "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	// An image without alt text gets a generic one naming the instance.
	if !strings.Contains(got["og:image:alt"], "例のサーバー") {
		t.Errorf("og:image:alt = %q", got["og:image:alt"])
	}
	if d := []rune(got["og:description"]); !strings.HasPrefix(got["og:description"], "Gardeners & friends") || len(d) > 200 {
		t.Errorf("og:description = %q (%d characters)", got["og:description"], len(d))
	}
	if !strings.Contains(body, `<html lang="ja" dir="ltr">`) || !strings.Contains(body, "<title>例のサーバー</title>") {
		t.Errorf("GET /:\n%s", body)
	}
	h := rec.Header()
	if h.Get("Cache-Control") != "public, max-age=60, stale-while-revalidate=300" || h.Get("Vary") != "Accept" || h.Get("ETag") == "" {
		t.Errorf("headers = %v", h)
	}

	// A new summary is a new version of the page.
	before := h.Get("ETag")
	summary = "Another summary"
	if again := get(mux, "/", nil); again.Header().Get("ETag") == before || metas(t, again.Body.String())["og:description"] != "Another summary" {
		t.Errorf("the page did not change with the summary")
	}
	// Without a summary, there is no og:description.
	summary = " "
	if got := get(mux, "/", nil).Body.String(); strings.Contains(got, "og:description") {
		t.Errorf("an empty summary is described:\n%s", got)
	}
}

// TestAboutPreview checks that the information page (/about) previews the instance like the
// landing page, under its own address and title, in the instance's default locale.
func TestAboutPreview(t *testing.T) {
	names := map[string]string{"ja": "例のサーバー", "en": "Example Social"}
	_, mux := newHandler(t, web.Options{
		DefaultLocale:       locale("ja"),
		InstanceName:        func(_ context.Context, l string) string { return names[l] },
		InstanceDescription: func(context.Context, string) string { return "Gardeners & friends" },
	})
	rec := get(mux, "/about", map[string]string{"Accept-Language": "en"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /about = %d", rec.Code)
	}
	body := rec.Body.String()
	got := metas(t, body)
	want := map[string]string{
		"og:type":           "website",
		"og:site_name":      "例のサーバー",
		"og:title":          "例のサーバー について",
		"og:url":            "https://social.example/about",
		"og:description":    "Gardeners & friends",
		"og:locale":         "ja",
		"link:canonical":    "https://social.example/about",
		"fediverse:creator": "",
		"link:alternate":    "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if !strings.Contains(body, "<title>例のサーバー について</title>") {
		t.Errorf("GET /about:\n%s", body)
	}
	if rec.Header().Get("ETag") == "" || rec.Header().Get("ETag") == get(mux, "/", nil).Header().Get("ETag") {
		t.Errorf("ETag of /about = %q, want its own", rec.Header().Get("ETag"))
	}
}
