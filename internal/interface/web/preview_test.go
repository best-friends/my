// SPDX-License-Identifier: AGPL-3.0-only

package web_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/interface/web"
)

const (
	aliceID  = "0192f0c1-0000-7000-8000-00000000a11c"
	statusID = "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f"
)

// fakePreviews serves the profiles and statuses it holds; everything else is not found.
type fakePreviews struct {
	profiles map[string]appcontent.ProfilePreview
	statuses map[string]appcontent.StatusPreview
}

func (f *fakePreviews) Profile(_ context.Context, username string) (appcontent.ProfilePreview, error) {
	if p, ok := f.profiles[username]; ok {
		return p, nil
	}
	return appcontent.ProfilePreview{}, fmt.Errorf("profile %s: %w", username, appcontent.ErrNotFound)
}

func (f *fakePreviews) Status(_ context.Context, id string) (appcontent.StatusPreview, error) {
	if s, ok := f.statuses[id]; ok {
		return s, nil
	}
	return appcontent.StatusPreview{}, fmt.Errorf("status %s: %w", id, appcontent.ErrNotFound)
}

func alice(displayName, note string) appcontent.ProfilePreview {
	return appcontent.ProfilePreview{
		Actor: appaccount.Actor{
			Account: account.Account{ID: aliceID, URI: "https://social.example/ap/actors/" + aliceID},
			Local:   &account.LocalAccount{AccountID: aliceID, Username: "alice"},
			Profile: &account.ProfileRevision{ID: "rev-1", DisplayName: displayName, NoteHTML: note, Indexable: true},
		},
		Language: "ja",
	}
}

func status(author appcontent.ProfilePreview, vis content.Visibility, bodyHTML string) appcontent.StatusPreview {
	return appcontent.StatusPreview{
		Post: appcontent.Post{
			Status: content.Status{
				ID: statusID, URI: "https://social.example/ap/objects/" + statusID, AuthorAccountID: aliceID, IsLocal: true,
				Visibility: vis, PublishedAt: time.Date(2026, 10, 16, 12, 34, 56, 789, time.UTC),
			},
			Revision: content.Revision{
				ID: "status-rev-1", StatusID: statusID, Seq: 1, Language: "en",
				Bodies: []content.Body{{Language: "en", ContentHTML: bodyHTML}},
			},
		},
		Author: author, Visibility: vis, Language: "en",
	}
}

func previewHandler(t *testing.T, p *fakePreviews, o web.Options) http.Handler {
	t.Helper()
	o.Previews = p
	if o.AccountDomain == "" {
		o.AccountDomain = "example.org"
	}
	if o.InstanceName == "" {
		o.InstanceName = "Example Social"
	}
	_, mux := newHandler(t, o)
	return mux
}

// metas returns the content of every <meta property|name=...> and the href of every <link
// rel=canonical|alternate>, unescaped, by key.
func metas(t *testing.T, body string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`<meta (?:property|name)="([^"]+)" content="([^"]*)">`).FindAllStringSubmatch(body, -1) {
		if _, dup := out[m[1]]; dup {
			t.Errorf("meta %s appears twice", m[1])
		}
		out[m[1]] = html.UnescapeString(m[2])
	}
	for _, m := range regexp.MustCompile(`<link rel="(canonical|alternate)"(?: type="[^"]+")? href="([^"]*)">`).FindAllStringSubmatch(body, -1) {
		out["link:"+m[1]] = html.UnescapeString(m[2])
	}
	return out
}

func TestProfilePreview(t *testing.T) {
	a := alice("Alice", `<p>Hello, <a href="https://social.example/tags/go">#go</a><br>world &amp; friends</p>`)
	a.Avatar = &appcontent.PreviewImage{URL: "https://social.example/media/av/full.png", Width: 400, Height: 400}
	mux := previewHandler(t, &fakePreviews{profiles: map[string]appcontent.ProfilePreview{"alice": a}}, web.Options{})

	for _, path := range []string{"/@alice", "/@alice@example.org", "/@alice@social.example"} {
		rec := get(mux, path, map[string]string{"Accept-Language": "ar"})
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		want := map[string]string{
			"og:type":           "profile",
			"og:site_name":      "Example Social",
			"og:title":          "Alice（@alice@example.org）",
			"og:url":            "https://social.example/@alice",
			"og:description":    "Hello, #go world & friends",
			"og:locale":         "ja",
			"og:image":          "https://social.example/media/av/full.png",
			"og:image:width":    "400",
			"og:image:height":   "400",
			"og:image:alt":      "Alice のアバター",
			"profile:username":  "alice@example.org",
			"twitter:card":      "summary",
			"fediverse:creator": "@alice@example.org",
			"link:canonical":    "https://social.example/@alice",
			"link:alternate":    "https://social.example/ap/actors/" + aliceID,
			"theme-color":       "#ffffff",
			"viewport":          "width=device-width, initial-scale=1, viewport-fit=cover",
			"article:author":    "",
			"robots":            "",
		}
		got := metas(t, body)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("GET %s: %s = %q, want %q", path, k, got[k], v)
			}
		}
		// The page's language is the account's, never Accept-Language; the client's default
		// stays the instance's.
		if !strings.Contains(body, `<html lang="ja" dir="ltr">`) || !strings.Contains(body, `"defaultLocale":"en"`) ||
			!strings.Contains(body, "<title>Alice（@alice@example.org）</title>") {
			t.Errorf("GET %s:\n%s", path, body)
		}
		if !strings.Contains(body, `<link rel="alternate" type="application/activity+json" href="https://social.example/ap/actors/`+aliceID+`">`) {
			t.Errorf("no ActivityPub alternate:\n%s", body)
		}
		h := rec.Header()
		if h.Get("Cache-Control") != "public, max-age=60, stale-while-revalidate=300" || h.Get("Vary") != "Accept" ||
			h.Get("ETag") == "" || h.Get("X-Robots-Tag") != "" || !strings.Contains(h.Get("Content-Security-Policy"), "script-src 'self';") {
			t.Errorf("GET %s headers = %v", path, h)
		}
	}
}

func TestPostPreview(t *testing.T) {
	a := alice("Alice", "")
	a.Header = &appcontent.PreviewImage{URL: "https://social.example/media/hd/full.jpg", Width: 1500, Height: 500}
	long := strings.Repeat("word ", 60)
	s := status(a, content.VisibilityPublic, "<p>"+long+"</p>")
	s.Image = &appcontent.PreviewImage{URL: "https://social.example/media/im/full.jpg", Width: 1200, Height: 630, Alt: "A cat"}
	unlisted := status(a, content.VisibilityUnlisted, "<p>quiet</p>")
	unlisted.Post.Status.ID = "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e70"
	unlisted.Language = "ar"
	p := &fakePreviews{statuses: map[string]appcontent.StatusPreview{statusID: s, unlisted.Post.Status.ID: unlisted}}
	mux := previewHandler(t, p, web.Options{})

	rec := get(mux, "/@alice/"+statusID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("post page = %d", rec.Code)
	}
	got := metas(t, rec.Body.String())
	for k, v := range map[string]string{
		"og:type":                "article",
		"og:title":               "Alice (@alice@example.org)",
		"og:url":                 "https://social.example/@alice/" + statusID,
		"og:locale":              "en",
		"og:image":               "https://social.example/media/im/full.jpg",
		"og:image:width":         "1200",
		"og:image:height":        "630",
		"og:image:alt":           "A cat",
		"article:published_time": "2026-10-16T12:34:56Z",
		"article:author":         "https://social.example/@alice",
		"twitter:card":           "summary_large_image",
		"fediverse:creator":      "@alice@example.org",
		"link:canonical":         "https://social.example/@alice/" + statusID,
		"link:alternate":         "https://social.example/ap/objects/" + statusID,
		"profile:username":       "",
		"robots":                 "",
	} {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if d := got["og:description"]; !strings.HasPrefix(d, "word word") || !strings.HasSuffix(d, "…") || len([]rune(d)) > 200 {
		t.Errorf("og:description = %q, want the text truncated to 200 characters", d)
	}
	if rec.Header().Get("X-Robots-Tag") != "" {
		t.Errorf("a public post is noindex")
	}

	// An unlisted post has a preview but is kept out of search engines.
	rec = get(mux, "/@alice/"+unlisted.Post.Status.ID, nil)
	body := rec.Body.String()
	got = metas(t, body)
	if rec.Code != http.StatusOK || got["robots"] != "noindex" || rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("unlisted post = %d, robots %q, X-Robots-Tag %q", rec.Code, got["robots"], rec.Header().Get("X-Robots-Tag"))
	}
	// Without an attached image, the header image is the preview; the post's language sets
	// <html lang dir>.
	if got["og:image"] != a.Header.URL || got["og:image:alt"] != "Header image of Alice" || got["twitter:card"] != "summary_large_image" ||
		got["og:locale"] != "ar" || !strings.Contains(body, `<html lang="ar" dir="rtl">`) {
		t.Errorf("unlisted post meta = %v\n%s", got, body)
	}
}

// TestPreviewImageOrder checks the order of ADR-0028: the attached image, the header image or
// avatar, the instance's default image, and no image.
func TestPreviewImageOrder(t *testing.T) {
	attached := &appcontent.PreviewImage{URL: "https://social.example/media/at/full.png", Width: 800, Height: 800, Alt: "attached"}
	header := &appcontent.PreviewImage{URL: "https://social.example/media/hd/full.png", Width: 1500, Height: 500}
	avatar := &appcontent.PreviewImage{URL: "https://social.example/media/av/full.png", Width: 400, Height: 400}
	def := &web.Image{URL: "https://social.example/default.png", Width: 1200, Height: 630, Alt: "Example Social"}
	tests := []struct {
		name                     string
		attached, header, avatar *appcontent.PreviewImage
		def                      *web.Image
		want, alt, card          string
	}{
		{"attached", attached, header, avatar, def, attached.URL, "attached", "summary"},
		{"header", nil, header, avatar, def, header.URL, "Header image of Alice", "summary_large_image"},
		{"avatar", nil, nil, avatar, def, avatar.URL, "Avatar of Alice", "summary"},
		{"default", nil, nil, nil, def, def.URL, "Example Social", "summary_large_image"},
		{"none", nil, nil, nil, nil, "", "", "summary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := alice("Alice", "")
			a.Language = "en"
			a.Header, a.Avatar = tt.header, tt.avatar
			s := status(a, content.VisibilityPublic, "<p>hi</p>")
			s.Image = tt.attached
			mux := previewHandler(t, &fakePreviews{statuses: map[string]appcontent.StatusPreview{statusID: s}}, web.Options{DefaultImage: tt.def})
			got := metas(t, get(mux, "/@alice/"+statusID, nil).Body.String())
			if got["og:image"] != tt.want || got["og:image:alt"] != tt.alt || got["twitter:card"] != tt.card {
				t.Errorf("image %q alt %q card %q, want %q %q %q", got["og:image"], got["og:image:alt"], got["twitter:card"], tt.want, tt.alt, tt.card)
			}
		})
	}
}

// TestHiddenPagesAreTheGenericShell checks that every profile or post page without a preview
// is the generic shell with 404 and noindex, byte for byte the same whatever the reason. The
// reasons that the application layer hides (visibility, deletion, restricted authors) are
// checked end to end in internal/platform.
func TestHiddenPagesAreTheGenericShell(t *testing.T) {
	p := &fakePreviews{profiles: map[string]appcontent.ProfilePreview{"alice": alice("Alice", "")}}
	mux := previewHandler(t, p, web.Options{})
	generic := get(mux, "/home", nil)

	var first *http.Header
	for _, path := range []string{
		"/@nobody",
		"/@not-a-username",
		"/@" + strings.Repeat("a", 100),
		"/@alice/" + statusID, // no such status
		"/@nobody/" + statusID,
	} {
		rec := get(mux, path, nil)
		if rec.Code != http.StatusNotFound || rec.Body.String() != generic.Body.String() {
			t.Errorf("GET %s = %d, want 404 with the generic shell:\n%s", path, rec.Code, rec.Body)
		}
		h := rec.Header()
		if h.Get("X-Robots-Tag") != "noindex" || h.Get("Content-Type") != "text/html; charset=utf-8" || h.Get("Vary") != "Accept" {
			t.Errorf("GET %s headers = %v", path, h)
		}
		if first == nil {
			first = &h
		} else if fmt.Sprint(*first) != fmt.Sprint(h) {
			t.Errorf("GET %s headers %v differ from %v", path, h, *first)
		}
	}
	// Other pages of the web client under a profile, and remote profiles, are the generic
	// shell.
	for _, path := range []string{"/@alice/followers", "/@alice/0192F3A4-7B1C-7D2E-9F00-1A2B3C4D5E6F", "/@bob@remote.example", "/@/x", "/@"} {
		rec := get(mux, path, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != generic.Body.String() || rec.Header().Get("X-Robots-Tag") != "" {
			t.Errorf("GET %s = %d, want the generic shell", path, rec.Code)
		}
	}
}

// TestPostPageRedirectsToTheCurrentUsername checks ADR-0018: a post page is found by the status
// ID, and a URL with another username redirects to the canonical one.
func TestPostPageRedirectsToTheCurrentUsername(t *testing.T) {
	s := status(alice("Alice", ""), content.VisibilityPublic, "<p>hi</p>")
	mux := previewHandler(t, &fakePreviews{statuses: map[string]appcontent.StatusPreview{statusID: s}}, web.Options{})
	rec := get(mux, "/@old_name/"+statusID, nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://social.example/@alice/"+statusID {
		t.Errorf("GET under an old username = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// TestPreviewETag checks that the ETag follows the post's revision, visibility, and language,
// and that a conditional request revalidates the page.
func TestPreviewETag(t *testing.T) {
	s := status(alice("Alice", ""), content.VisibilityPublic, "<p>hi</p>")
	p := &fakePreviews{statuses: map[string]appcontent.StatusPreview{statusID: s}}
	mux := previewHandler(t, p, web.Options{})
	path := "/@alice/" + statusID
	tag := get(mux, path, nil).Header().Get("ETag")
	if rec := get(mux, path, map[string]string{"If-None-Match": tag}); rec.Code != http.StatusNotModified {
		t.Errorf("conditional post page = %d, want 304", rec.Code)
	}
	seen := map[string]string{tag: "original"}
	for name, change := range map[string]func(*appcontent.StatusPreview){
		"revision":   func(s *appcontent.StatusPreview) { s.Post.Revision.ID = "status-rev-2" },
		"visibility": func(s *appcontent.StatusPreview) { s.Visibility = content.VisibilityUnlisted },
		"language":   func(s *appcontent.StatusPreview) { s.Language = "ja" },
		"profile":    func(s *appcontent.StatusPreview) { s.Author.Actor.Profile.ID = "rev-2" },
	} {
		changed := s
		changed.Author.Actor.Profile = new(*s.Author.Actor.Profile)
		change(&changed)
		p.statuses[statusID] = changed
		tag := get(mux, path, nil).Header().Get("ETag")
		if prev, dup := seen[tag]; dup {
			t.Errorf("the ETag after a change of %s equals the one of %s", name, prev)
		}
		seen[tag] = name
	}
}

// TestPreviewEscaping checks that display names, post text, and alt text that try to break
// out of their attributes are escaped in every meta tag and in the title (ADR-0028).
func TestPreviewEscaping(t *testing.T) {
	const attack = `"><script>alert(1)</script><meta property="og:title" content="pwned"><!-- ' & </title>`
	a := alice(attack, "<p>"+html.EscapeString(attack)+"</p>")
	a.Avatar = &appcontent.PreviewImage{URL: "https://social.example/media/av/full.png", Width: 400, Height: 400}
	s := status(a, content.VisibilityPublic, "<p>"+html.EscapeString(attack)+"</p>")
	s.Image = &appcontent.PreviewImage{URL: "https://social.example/media/im/full.png", Width: 400, Height: 400, Alt: attack}
	cw := status(a, content.VisibilityPublic, "<p>hidden</p>")
	cw.Post.Status.ID = "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e70"
	cw.Post.Revision.SpoilerText = attack
	p := &fakePreviews{
		profiles: map[string]appcontent.ProfilePreview{"alice": a},
		statuses: map[string]appcontent.StatusPreview{statusID: s, cw.Post.Status.ID: cw},
	}
	mux := previewHandler(t, p, web.Options{InstanceName: attack})

	for _, path := range []string{"/@alice", "/@alice/" + statusID, "/@alice/" + cw.Post.Status.ID} {
		body := get(mux, path, nil).Body.String()
		if strings.Contains(body, "<script>alert") || strings.Contains(body, "pwned\">") || strings.Contains(body, "<!--") ||
			strings.Count(body, "<script") != 2 || strings.Count(body, "</title>") != 1 || strings.Count(body, `property="og:title"`) != 1 {
			t.Fatalf("GET %s: the attack escaped its context:\n%s", path, body)
		}
		got := metas(t, body)
		title := attack + " (@alice@example.org)"
		if path == "/@alice" {
			title = attack + "（@alice@example.org）"
		}
		checks := map[string]string{"og:title": title, "og:site_name": attack}
		switch path {
		case "/@alice":
			checks["og:description"] = attack
			checks["og:image:alt"] = attack + " のアバター"
		case "/@alice/" + statusID:
			checks["og:description"] = attack
			checks["og:image:alt"] = attack
		default:
			// Behind a content warning, the preview shows only the warning.
			checks["og:description"] = attack
		}
		for k, v := range checks {
			if got[k] != v {
				t.Errorf("GET %s: %s = %q, want %q", path, k, got[k], v)
			}
		}
		if m := regexp.MustCompile(`<title>([^<]*)</title>`).FindStringSubmatch(body); m == nil || html.UnescapeString(m[1]) != title {
			t.Errorf("GET %s: title = %v, want %q", path, m, title)
		}
	}
}
