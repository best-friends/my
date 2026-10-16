// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/best-friends/my/internal/i18n"
)

// Defaults of the Web App Manifest until the instance settings hold them.
const (
	DefaultThemeColor      = "#ffffff"
	DefaultBackgroundColor = "#ffffff"
)

// Options configure a Handler.
type Options struct {
	// Build is the Vite build of app/web; Embedded() in production.
	Build fs.FS
	// DevViteOrigin, when set, is the origin of a Vite dev server (http://localhost:5173).
	// The shell then loads the client from it with hot module replacement, and its
	// Content-Security-Policy is relaxed for that origin. Development only.
	DevViteOrigin string

	// Host is the serving domain of the instance (ADR-0018).
	Host string
	// AccountDomain is the domain of handles; empty means Host.
	AccountDomain string
	// InstanceName is the name the shell and the Web App Manifest show.
	InstanceName string
	// DefaultLocale returns the instance's current default locale, for <html lang dir>, the
	// runtime config, and the manifest; nil means the source locale. It is read per request so
	// that a change of the setting applies without a restart. The HTML never depends on
	// Accept-Language, so that shared caches keep one copy (ADR-0028).
	DefaultLocale func(ctx context.Context) string
	// ThemeColor and BackgroundColor go into the manifest; empty means the defaults.
	ThemeColor, BackgroundColor string

	// Previews reads the profiles and statuses that the shells of profile pages (/@alice) and
	// post pages (/@alice/{id}) describe in their link preview meta tags; nil serves the
	// generic shell there.
	Previews Previews
	// Messages formats the texts of link previews; nil loads the embedded catalogs.
	Messages *i18n.Bundle
	// DefaultImage returns the instance's preview image for profiles and posts without images,
	// for a preview in language lang (ADR-0028), or nil for the built-in image at
	// PathDefaultImage. An image without alt text gets a generic one naming the instance.
	// It is called per request, so that a change of the setting applies without a restart; nil
	// means the built-in image.
	DefaultImage func(ctx context.Context, lang string) (*Image, error)

	// NotFound writes the 404 response of the server, for paths the web client does not own.
	NotFound func(w http.ResponseWriter, r *http.Request)
	// Logger logs warnings at start; nil means slog.Default().
	Logger *slog.Logger
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Paths that the Handler serves besides the SPA routes.
const (
	PathAssets      = "/" + assetsDir + "/"
	PathWebManifest = "/manifest.webmanifest"
)

// reservedPrefixes are the paths of the server's other endpoints. The SPA has no routes under
// them, so a request there that no endpoint serves is the server's 404, never the shell.
// /uploads is the blob prefix of media as received, which is never served (it may carry EXIF).
var reservedPrefixes = []string{"/api", "/oauth", "/auth", "/.well-known", "/nodeinfo", "/ap", "/media", "/uploads"}

// Cache-Control values (ADR-0028).
const (
	// cacheShell lets shared caches keep the shell briefly; it is the same for every visitor.
	cacheShell = "public, max-age=60, stale-while-revalidate=300"
	// cacheImmutable is for content-hashed files, whose URL changes with their content.
	cacheImmutable = "public, max-age=31536000, immutable"
	// cacheRevalidate is for files at fixed URLs, such as sw.js and icon.svg.
	cacheRevalidate = "no-cache"
	cacheDev        = "no-store"
)

// Handler serves the web client. Its ServeHTTP serves the shell at SPA routes, and the
// files at the root of the build (sw.js, icon.svg).
type Handler struct {
	o         Options
	tags      entryTags
	apiOrigin string
	hasIcon   bool
	// hasAppleTouchIcon reports whether the build has apple-touch-icon.png.
	hasAppleTouchIcon bool
	// icons are the manifest icons whose files the build has.
	icons []manifestIcon
	csp   string
	// localized holds the shell and the manifest of each default locale seen, rendered once.
	localized sync.Map // locale -> *localizedDocuments
	// files are the servable files of the build, by path, with their ETags.
	files map[string]string
	// previewVersion identifies the template, the assets, and the instance settings that
	// previews render, for their ETags.
	previewVersion string
}

// localizedDocuments are the documents that depend on the default locale.
type localizedDocuments struct {
	shell, manifest document
}

// document is a response rendered once.
type document struct {
	body        []byte
	etag        string
	contentType string
	// noIndex asks search engines not to index the document (X-Robots-Tag).
	noIndex bool
}

// New returns a Handler on o. A build without a Vite manifest is not an error: the shell then
// loads no client, and New logs a warning.
func New(o Options) (*Handler, error) {
	if o.Build == nil {
		return nil, errors.New("web: no build")
	}
	if o.AccountDomain == "" {
		o.AccountDomain = o.Host
	}
	if o.InstanceName == "" {
		o.InstanceName = o.Host
	}
	if o.DefaultLocale == nil {
		o.DefaultLocale = func(context.Context) string { return i18n.SourceLocale }
	}
	if o.ThemeColor == "" {
		o.ThemeColor = DefaultThemeColor
	}
	if o.BackgroundColor == "" {
		o.BackgroundColor = DefaultBackgroundColor
	}
	if o.NotFound == nil {
		o.NotFound = func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Messages == nil {
		m, err := i18n.Load()
		if err != nil {
			return nil, err
		}
		o.Messages = m
	}
	h := &Handler{o: o}

	files, err := buildFiles(o.Build)
	if err != nil {
		return nil, err
	}
	h.files = files
	_, h.hasIcon = files[iconPath]
	_, h.hasAppleTouchIcon = files[appleTouchIconPath]
	h.icons = []manifestIcon{}
	for _, icon := range appIcons {
		if _, ok := files[strings.TrimPrefix(icon.Src, "/")]; ok {
			h.icons = append(h.icons, icon)
		}
	}

	var tags entryTags
	apiOrigin := "https://" + o.Host
	if o.DevViteOrigin != "" {
		dev, err := devOrigin(o.DevViteOrigin)
		if err != nil {
			return nil, err
		}
		h.o.DevViteOrigin = dev.String()
		tags = entryTags{Script: h.o.DevViteOrigin + "/src/main.tsx"}
		h.csp = devContentSecurityPolicy(dev)
		// The dev server setup is plain HTTP on localhost.
		apiOrigin = "http://" + o.Host
	} else {
		tags, err = readEntry(o.Build)
		if errors.Is(err, errNoBuild) {
			o.Logger.Warn("the web client is not built into this binary; the shell loads no client", slog.Any("error", err))
		} else if err != nil {
			return nil, err
		}
		h.csp = contentSecurityPolicy
	}

	h.tags, h.apiOrigin = tags, apiOrigin
	version := []string{
		shellSource, h.o.DevViteOrigin, h.apiOrigin, h.o.Host, h.o.AccountDomain, h.o.InstanceName, h.o.ThemeColor,
		strconv.FormatBool(h.hasIcon), strconv.FormatBool(h.hasAppleTouchIcon), tags.Script,
	}
	version = append(append(version, tags.Stylesheet...), tags.Preload...)
	h.previewVersion = etag([]byte(strings.Join(version, "\x00")))
	// Render once now, so that a template or encoding error fails the start.
	if _, err := h.render(i18n.SourceLocale); err != nil {
		return nil, err
	}
	return h, nil
}

// instanceLocale returns the instance's current default locale.
func (h *Handler) instanceLocale(ctx context.Context) string {
	if locale := h.o.DefaultLocale(ctx); locale != "" {
		return locale
	}
	return i18n.SourceLocale
}

// documents returns the shell and the manifest for the current default locale.
func (h *Handler) documents(ctx context.Context) (*localizedDocuments, error) {
	locale := h.instanceLocale(ctx)
	if d, ok := h.localized.Load(locale); ok {
		return d.(*localizedDocuments), nil
	}
	return h.render(locale)
}

// render renders and caches the documents of a default locale.
func (h *Handler) render(locale string) (*localizedDocuments, error) {
	shell, err := renderShell(shellData{
		Lang: locale, Dir: i18n.Direction(locale), Title: h.o.InstanceName, ThemeColor: h.o.ThemeColor,
		HasIcon: h.hasIcon, HasAppleTouchIcon: h.hasAppleTouchIcon, ViteClient: viteClient(h.o.DevViteOrigin), Entry: h.tags,
		Config: h.runtimeConfig(locale),
	})
	if err != nil {
		return nil, err
	}
	manifest, err := renderWebManifest(&h.o, locale, h.icons)
	if err != nil {
		return nil, err
	}
	d, _ := h.localized.LoadOrStore(locale, &localizedDocuments{shell: shell, manifest: manifest})
	return d.(*localizedDocuments), nil
}

// runtimeConfig returns the runtime config of the shell for the default locale.
func (h *Handler) runtimeConfig(locale string) runtimeConfig {
	return runtimeConfig{
		APIOrigin: h.apiOrigin, InstanceName: h.o.InstanceName, AccountDomain: h.o.AccountDomain, DefaultLocale: locale,
	}
}

func viteClient(origin string) string {
	if origin == "" {
		return ""
	}
	return origin + "/@vite/client"
}

// devOrigin parses the origin of a Vite dev server.
func devOrigin(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || s != u.Scheme+"://"+u.Host {
		return nil, fmt.Errorf("web: dev Vite origin %q must be an origin such as http://localhost:5173, without path or trailing slash", s)
	}
	return u, nil
}

// buildFiles lists the files of the build that are served as they are, with their ETags:
// every file except the Vite manifest and other hidden files, and the index.html that Vite
// writes (the shell replaces it).
func buildFiles(build fs.FS) (map[string]string, error) {
	files := map[string]string{}
	err := fs.WalkDir(build, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || p == entryKey {
			return nil
		}
		data, err := fs.ReadFile(build, p)
		if err != nil {
			return err
		}
		files[p] = etag(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("web: list the build: %w", err)
	}
	return files, nil
}

// etag is a strong entity tag of content.
func etag(content []byte) string {
	sum := sha256.Sum256(content)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:18]) + `"`
}

// Register registers the hashed assets, the Web App Manifest, and the built-in preview image on r. The shell and the
// files at the root of the build are served by ServeHTTP, which the caller routes to.
func (h *Handler) Register(r Router) {
	r.Handle("GET "+PathAssets+"{path...}", http.HandlerFunc(h.asset))
	r.Handle("GET "+PathWebManifest, http.HandlerFunc(h.webManifest))
	r.Handle("GET "+PathDefaultImage, http.HandlerFunc(h.serveDefaultImage))
}

// ServeHTTP serves a GET request for an SPA route: a file at the root of the build when one
// has the requested name, the server's 404 under the paths of other endpoints, the shell with
// link preview meta tags at profile and post pages, and the generic shell otherwise.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || reserved(p) {
		h.o.NotFound(w, r)
		return
	}
	if name := strings.TrimPrefix(p, "/"); name != "" && !strings.Contains(name, "/") {
		if _, ok := h.files[name]; ok {
			if name == serviceWorkerPath {
				// A worker runs under the CSP of its own script's response, not the page's; the server's
				// default for non-documents (default-src 'none') would block its fetches.
				w.Header().Set("Content-Security-Policy", serviceWorkerContentSecurityPolicy)
			}
			h.serveFile(w, r, name, cacheRevalidate)
			return
		}
	}
	if h.o.Previews != nil {
		if route, ok := h.parsePreviewRoute(p); ok {
			h.servePreview(w, r, route)
			return
		}
	}
	h.serveShell(w, r)
}

// reserved reports whether p is under the path of another endpoint of the server.
func reserved(p string) bool {
	for _, prefix := range reservedPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func (h *Handler) serveShell(w http.ResponseWriter, r *http.Request) {
	d, err := h.documents(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.shellHeaders(w.Header())
	d.shell.serve(w, r)
}

// shellHeaders sets the headers that every shell carries: Vary: Accept, since the same URL
// may be an ActivityPub document, the Content-Security-Policy, and short public caching.
func (h *Handler) shellHeaders(hd http.Header) {
	addVary(hd, "Accept")
	hd.Set("Content-Security-Policy", h.csp)
	if h.o.DevViteOrigin != "" {
		hd.Set("Cache-Control", cacheDev)
	} else {
		hd.Set("Cache-Control", cacheShell)
	}
}

func (h *Handler) webManifest(w http.ResponseWriter, r *http.Request) {
	d, err := h.documents(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", cacheShell)
	d.manifest.serve(w, r)
}

// fail answers a document that cannot be rendered with 500 after logging it.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.o.Logger.ErrorContext(r.Context(), "web: render failed", slog.Any("error", err))
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// asset serves GET /assets/{path...}: content-hashed files, cached for a year.
func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	// Only clean paths are in files, so "..", "." and empty segments find nothing.
	name := assetsDir + "/" + r.PathValue("path")
	if _, ok := h.files[name]; !ok {
		h.o.NotFound(w, r)
		return
	}
	h.serveFile(w, r, name, cacheImmutable)
}

// serveFile serves a file of the build with its ETag, answering conditional requests.
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name, cacheControl string) {
	f, err := h.o.Build.Open(name)
	if err != nil {
		h.o.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	content, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(f)
		if err != nil {
			h.o.NotFound(w, r)
			return
		}
		content = bytes.NewReader(data)
	}
	hd := w.Header()
	hd.Set("Cache-Control", cacheControl)
	hd.Set("ETag", h.files[name])
	http.ServeContent(w, r, name, time.Time{}, content)
}

// serve writes the document with its ETag, answering conditional requests.
func (d document) serve(w http.ResponseWriter, r *http.Request) {
	hd := w.Header()
	hd.Set("Content-Type", d.contentType)
	hd.Set("ETag", d.etag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(d.body))
}

// addVary adds value to the Vary header unless it is already there.
func addVary(h http.Header, value string) {
	if !slices.Contains(h.Values("Vary"), value) {
		h.Add("Vary", value)
	}
}

// Stylesheet returns the URL of the stylesheet that the build makes for the server-rendered
// pages from the client's design tokens (app/web/src/server-pages.css, ADR-0030), or "" when the
// build has none (no build, or a Vite dev server).
func (h *Handler) Stylesheet() string { return h.tags.ServerPages }
