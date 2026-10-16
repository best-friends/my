// SPDX-License-Identifier: AGPL-3.0-only

package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/best-friends/my/internal/interface/web"
)

// fakeBuild is a Vite build with an entry that imports a shared chunk, CSS on both, a
// lazily loaded chunk that must not be preloaded, and the server pages' stylesheet entry.
func fakeBuild() fstest.MapFS {
	return fstest.MapFS{
		".vite/manifest.json": {Data: []byte(`{
			"index.html": {"file": "assets/index-AAAA.js", "src": "index.html", "isEntry": true,
				"imports": ["_shared-BBBB.js"], "dynamicImports": ["src/lazy.tsx"], "css": ["assets/index-CCCC.css"]},
			"_shared-BBBB.js": {"file": "assets/shared-BBBB.js", "css": ["assets/shared-DDDD.css"]},
			"src/lazy.tsx": {"file": "assets/lazy-EEEE.js", "src": "src/lazy.tsx", "isDynamicEntry": true},
			"src/server-pages.css": {"file": "assets/server-pages-FFFF.css", "src": "src/server-pages.css", "isEntry": true}
		}`)},
		"index.html":                   {Data: []byte("<!doctype html><title>vite</title>")},
		"assets/index-AAAA.js":         {Data: []byte("console.log('entry')")},
		"assets/shared-BBBB.js":        {Data: []byte("export {}")},
		"assets/index-CCCC.css":        {Data: []byte("body{}")},
		"assets/shared-DDDD.css":       {Data: []byte("p{}")},
		"assets/lazy-EEEE.js":          {Data: []byte("export {}")},
		"assets/server-pages-FFFF.css": {Data: []byte("main{}")},
		"icon.svg":                     {Data: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")},
		"sw.js":                        {Data: []byte("self.addEventListener('fetch', () => {})")},
	}
}

func locale(l string) func(context.Context) string {
	return func(context.Context) string { return l }
}

func newHandler(t *testing.T, o web.Options) (*web.Handler, *http.ServeMux) {
	t.Helper()
	if o.Build == nil {
		o.Build = fakeBuild()
	}
	if o.Host == "" {
		o.Host = "social.example"
	}
	h, err := web.New(o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	mux.Handle("GET /", h)
	return h, mux
}

func get(mux http.Handler, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestShellLoadsTheBuildFromTheManifest(t *testing.T) {
	_, mux := newHandler(t, web.Options{InstanceName: "Example Social"})
	rec := get(mux, "/@alice/0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("shell = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<html lang="en" dir="ltr">`,
		`<title>Example Social</title>`,
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<link rel="icon" href="/icon.svg" type="image/svg+xml">`,
		`<link rel="stylesheet" href="/assets/index-CCCC.css">` + "\n" + `<link rel="stylesheet" href="/assets/shared-DDDD.css">`,
		`<link rel="modulepreload" href="/assets/shared-BBBB.js">`,
		`<script type="module" src="/assets/index-AAAA.js"></script>`,
		`<div id="root"></div>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "lazy-EEEE") || strings.Contains(body, "vite</title>") || strings.Contains(body, "server-pages") {
		t.Errorf("shell preloads a lazy chunk, loads the server pages' stylesheet, or is Vite's index.html:\n%s", body)
	}
	h := rec.Header()
	if h.Get("Cache-Control") != "public, max-age=60, stale-while-revalidate=300" || h.Get("Vary") != "Accept" || h.Get("ETag") == "" {
		t.Errorf("headers = %v, want short public caching, Vary: Accept and an ETag", h)
	}

	// Every route gets the same shell, so a conditional request revalidates it.
	again := get(mux, "/home", map[string]string{"If-None-Match": h.Get("ETag")})
	if again.Code != http.StatusNotModified {
		t.Errorf("conditional shell = %d, want 304", again.Code)
	}
}

func TestStylesheetIsTheServerPagesEntry(t *testing.T) {
	h, mux := newHandler(t, web.Options{})
	if got := h.Stylesheet(); got != "/assets/server-pages-FFFF.css" {
		t.Fatalf("Stylesheet() = %q, want the server pages' entry", got)
	}
	if rec := get(mux, h.Stylesheet(), nil); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/css; charset=utf-8" {
		t.Errorf("GET %s = %d %q", h.Stylesheet(), rec.Code, rec.Header().Get("Content-Type"))
	}

	build := fakeBuild()
	build[".vite/manifest.json"] = &fstest.MapFile{Data: []byte(`{"index.html": {"file": "assets/index-AAAA.js", "isEntry": true}}`)}
	without, _ := newHandler(t, web.Options{Build: build})
	if got := without.Stylesheet(); got != "" {
		t.Errorf("Stylesheet() without the entry = %q, want empty", got)
	}
}

// TestShellCSP checks that the shell runs under the nonce-free policy of ADR-0019 §9: every
// script is an external file from the server's own origin, apart from the JSON data block,
// which is not executed.
func TestShellCSP(t *testing.T) {
	_, mux := newHandler(t, web.Options{})
	rec := get(mux, "/", nil)
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src 'self';", "style-src 'self';", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'", "require-trusted-types-for 'script'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %s", csp, want)
		}
	}
	for _, bad := range []string{"nonce-", "'unsafe-inline'", "'unsafe-eval'", "sha256-", "http:", "https:"} {
		if strings.Contains(csp, bad) {
			t.Errorf("CSP %q contains %s", csp, bad)
		}
	}
	body := rec.Body.String()
	scripts := regexp.MustCompile(`<script([^>]*)>([^<]*)</script>`).FindAllStringSubmatch(body, -1)
	if len(scripts) != 2 {
		t.Fatalf("shell has %d scripts, want the data block and the entry:\n%s", len(scripts), body)
	}
	for _, s := range scripts {
		attrs, content := s[1], s[2]
		switch {
		case attrs == ` type="application/json" id="config"`:
		case strings.HasPrefix(attrs, ` type="module" src="/`) && !strings.HasPrefix(attrs, ` type="module" src="//`) && content == "":
		default:
			t.Errorf("script %q would need a nonce or a remote origin under %q", s[0], csp)
		}
	}
	if strings.Contains(body, " style=") || strings.Contains(body, "<style") || strings.Contains(body, " on") {
		t.Errorf("shell has inline styles or event handlers:\n%s", body)
	}
}

// TestRuntimeConfigIsEscaped is a regression test: instance settings reach the shell's JSON
// data block through html/template, which must keep them from ending the script element.
func TestRuntimeConfigIsEscaped(t *testing.T) {
	name := `</script><script>alert(1)</script><!-- "quoted" & 'single' \ ` + "\u2028"
	_, mux := newHandler(t, web.Options{InstanceName: name, Host: "social.example", AccountDomain: "example.org", DefaultLocale: locale("ja")})
	body := get(mux, "/", nil).Body.String()
	if strings.Count(body, "</script>") != 2 || strings.Contains(body, "<script>alert") || strings.Contains(body, "<!--") {
		t.Fatalf("the instance name escaped the data block:\n%s", body)
	}
	m := regexp.MustCompile(`<script type="application/json" id="config">([^<]*)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no data block:\n%s", body)
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(m[1]), &cfg); err != nil {
		t.Fatalf("data block %q is not JSON: %v", m[1], err)
	}
	want := map[string]string{"apiOrigin": "https://social.example", "instanceName": name, "accountDomain": "example.org", "defaultLocale": "ja"}
	for k, v := range want {
		if cfg[k] != v {
			t.Errorf("config[%q] = %q, want %q", k, cfg[k], v)
		}
	}
	if !strings.Contains(body, "<title>&lt;/script&gt;&lt;script&gt;alert(1)") {
		t.Errorf("title is not HTML-escaped:\n%s", body)
	}
}

func TestShellLanguageAndDirection(t *testing.T) {
	current := "ar"
	_, mux := newHandler(t, web.Options{DefaultLocale: func(context.Context) string { return current }})
	ar := get(mux, "/", map[string]string{"Accept-Language": "ja"})
	if body := ar.Body.String(); !strings.Contains(body, `<html lang="ar" dir="rtl">`) {
		t.Errorf("shell for an Arabic instance:\n%s", body)
	}
	// A change of the default locale applies to the next request, with a new ETag.
	current = "ja"
	ja := get(mux, "/", nil)
	if body := ja.Body.String(); !strings.Contains(body, `<html lang="ja" dir="ltr">`) || !strings.Contains(body, `"defaultLocale":"ja"`) {
		t.Errorf("shell after the default locale changed:\n%s", body)
	}
	if ja.Header().Get("ETag") == ar.Header().Get("ETag") {
		t.Errorf("the ETag did not change with the locale")
	}
}

func TestReservedPathsAreNotTheShell(t *testing.T) {
	var notFound []string
	_, mux := newHandler(t, web.Options{NotFound: func(w http.ResponseWriter, r *http.Request) {
		notFound = append(notFound, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}})
	reserved := []string{"/api", "/api/v1/nope", "/oauth/authorize", "/auth/login", "/.well-known/nope", "/nodeinfo", "/nodeinfo/2.0", "/ap/nope", "/media/x.png", "/uploads/x"}
	for _, p := range reserved {
		if rec := get(mux, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
	if len(notFound) != len(reserved) {
		t.Errorf("NotFound served %q, want %q", notFound, reserved)
	}
	for _, p := range []string{"/apple", "/application/x", "/authors", "/mediathek", "/settings/profile", "/tags/go"} {
		if rec := get(mux, p, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="root"`) {
			t.Errorf("GET %s = %d, want the shell", p, rec.Code)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	_, mux := newHandler(t, web.Options{})
	tests := []struct {
		path, cache, contentType, body string
	}{
		{"/assets/index-AAAA.js", "public, max-age=31536000, immutable", "text/javascript; charset=utf-8", "console.log('entry')"},
		{"/assets/index-CCCC.css", "public, max-age=31536000, immutable", "text/css; charset=utf-8", "body{}"},
		{"/sw.js", "no-cache", "text/javascript; charset=utf-8", "self.addEventListener('fetch', () => {})"},
		{"/icon.svg", "no-cache", "image/svg+xml", "<svg xmlns='http://www.w3.org/2000/svg'/>"},
	}
	for _, tt := range tests {
		rec := get(mux, tt.path, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != tt.body {
			t.Errorf("GET %s = %d %q, want %q", tt.path, rec.Code, rec.Body, tt.body)
			continue
		}
		h := rec.Header()
		if h.Get("Cache-Control") != tt.cache || h.Get("Content-Type") != tt.contentType || h.Get("ETag") == "" {
			t.Errorf("GET %s headers = %v, want Cache-Control %q, Content-Type %q and an ETag", tt.path, h, tt.cache, tt.contentType)
		}
		if rec := get(mux, tt.path, map[string]string{"If-None-Match": h.Get("ETag")}); rec.Code != http.StatusNotModified {
			t.Errorf("conditional GET %s = %d, want 304", tt.path, rec.Code)
		}
	}
	for _, p := range []string{"/assets/missing.js", "/assets/", "/assets/.vite/manifest.json"} {
		if rec := get(mux, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
	// Vite's index.html and the build manifest are never served as files.
	for _, p := range []string{"/index.html", "/.vite/manifest.json"} {
		if rec := get(mux, p, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="config"`) {
			t.Errorf("GET %s = %d %q, want the shell", p, rec.Code, rec.Body)
		}
	}
}

func TestWebManifest(t *testing.T) {
	_, mux := newHandler(t, web.Options{InstanceName: "Example Social", DefaultLocale: locale("ar")})
	rec := get(mux, web.PathWebManifest, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" || rec.Header().Get("ETag") == "" {
		t.Fatalf("manifest = %d %v", rec.Code, rec.Header())
	}
	if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") || cc == "" {
		t.Errorf("manifest Cache-Control = %q, want short caching", cc)
	}
	var m struct {
		ID, Name, Lang, Dir, Display string
		ShortName                    string `json:"short_name"`
		StartURL                     string `json:"start_url"`
		Scope                        string
		ThemeColor                   string `json:"theme_color"`
		BackgroundColor              string `json:"background_color"`
		Icons                        []struct{ Src, Sizes, Type string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "Example Social" || m.ShortName != "Example Social" || m.Lang != "ar" || m.Dir != "rtl" ||
		m.StartURL != "/" || m.Scope != "/" || m.ID != "/" || m.Display != "standalone" ||
		m.ThemeColor != web.DefaultThemeColor || m.BackgroundColor != web.DefaultBackgroundColor {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Icons) != 1 || m.Icons[0].Src != "/icon.svg" || m.Icons[0].Type != "image/svg+xml" {
		t.Errorf("icons = %+v, want the SVG icon of the build", m.Icons)
	}
}

// TestWithoutBuild checks the binary that Go builds without `make web-build`: it starts,
// warns, and serves a shell that loads nothing.
func TestWithoutBuild(t *testing.T) {
	var logs bytes.Buffer
	build := fstest.MapFS{".gitkeep": {}}
	_, mux := newHandler(t, web.Options{Build: build, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	rec := get(mux, "/", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `type="module"`) || !strings.Contains(rec.Body.String(), `id="config"`) {
		t.Errorf("shell without a build = %d:\n%s", rec.Code, rec.Body)
	}
	if !strings.Contains(logs.String(), "not built") {
		t.Errorf("no warning about the missing build: %q", logs.String())
	}
	if rec := get(mux, "/.gitkeep", nil); !strings.Contains(rec.Body.String(), `id="config"`) {
		t.Errorf("GET /.gitkeep = %q, want the shell", rec.Body)
	}

	// The embedded build is the placeholder, or the build of `make web-build`.
	if _, err := web.New(web.Options{Host: "social.example", Build: web.Embedded()}); err != nil {
		t.Errorf("New on the embedded build: %v", err)
	}
}

func TestBrokenManifest(t *testing.T) {
	for name, manifest := range map[string]string{
		"not JSON":      `{`,
		"no entry":      `{"src/main.tsx": {"file": "assets/main.js", "isEntry": true}}`,
		"missing chunk": `{"index.html": {"file": "assets/index.js", "isEntry": true, "imports": ["_gone.js"]}}`,
	} {
		_, err := web.New(web.Options{Host: "social.example", Build: fstest.MapFS{".vite/manifest.json": {Data: []byte(manifest)}}})
		if err == nil {
			t.Errorf("%s: New succeeded", name)
		}
	}
}

func TestDevViteOrigin(t *testing.T) {
	_, mux := newHandler(t, web.Options{DevViteOrigin: "http://localhost:5173", Host: "localhost:8080"})
	rec := get(mux, "/", nil)
	body := rec.Body.String()
	for _, want := range []string{
		`<script type="module" src="http://localhost:5173/@vite/client"></script>` + "\n" + `<script type="module" src="http://localhost:5173/src/main.tsx"></script>`,
		`"apiOrigin":"http://localhost:8080"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dev shell lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "/assets/") {
		t.Errorf("dev shell loads the embedded build:\n%s", body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' http://localhost:5173;") || !strings.Contains(csp, "ws://localhost:5173") {
		t.Errorf("dev CSP = %q, want the dev server's scripts and WebSocket", csp)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("dev shell Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
	for _, bad := range []string{"localhost:5173/", "ftp://localhost", "localhost:5173", "http://user@localhost:5173"} {
		if _, err := web.New(web.Options{Host: "localhost:8080", Build: fakeBuild(), DevViteOrigin: bad}); err == nil {
			t.Errorf("dev origin %q accepted", bad)
		}
	}
}
