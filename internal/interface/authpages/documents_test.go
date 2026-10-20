// SPDX-License-Identifier: AGPL-3.0-only

package authpages_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/authpages"
)

// fakeDocuments holds documents by key. err fails every read.
type fakeDocuments struct {
	docs map[instance.TextKey]appinstance.Document
	err  error
}

func (f *fakeDocuments) Document(_ context.Context, key instance.TextKey) (appinstance.Document, bool, error) {
	d, ok := f.docs[key]
	return d, ok, f.err
}

func (f *fakeDocuments) HasDocument(_ context.Context, key instance.TextKey) (bool, error) {
	_, ok := f.docs[key]
	return ok, f.err
}

func newDocumentServer(t *testing.T, id authpages.Identity, docs authpages.Documents) http.Handler {
	t.Helper()
	b, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	m := mux{http.NewServeMux()}
	authpages.New(authpages.Options{Identity: id, Locales: &i18n.Resolver{Bundle: b}, Documents: docs}).Register(m)
	return m
}

var revised = time.Date(2026, 10, 1, 23, 30, 0, 0, time.FixedZone("JST", 9*60*60))

func TestDocumentPages(t *testing.T) {
	docs := &fakeDocuments{docs: map[instance.TextKey]appinstance.Document{
		instance.TextTerms: {Key: instance.TextTerms, HTML: "<h2>第1条</h2>\n<p>守ること</p>", RevisedAt: revised},
		instance.TextPrivacyPolicy: {
			Key: instance.TextPrivacyPolicy, HTML: "<p>We keep <strong>little</strong>.</p>", RevisedAt: revised,
		},
	}}
	b := newBrowser(t, newDocumentServer(t, newFakeIdentity(), docs))

	w := b.do(http.MethodGet, authpages.PathTerms, nil, map[string]string{"Accept-Language": "ja"})
	if w.Code != http.StatusOK {
		t.Fatalf("GET terms = %d", w.Code)
	}
	if h := w.Header(); h.Get("Cache-Control") != "public, max-age=60" || h.Get("Content-Language") != "ja" || len(w.Result().Cookies()) != 0 {
		t.Errorf("headers = %v, cookies = %v", h, w.Result().Cookies())
	}
	body := w.Body.String()
	wantContains(t, body,
		`<title>利用規約</title>`,
		`<h1>利用規約</h1>`,
		`<p class="hint">最終更新日：2026-10-01</p>`, // the UTC date
		"<div class=\"document\" lang=\"\" dir=\"auto\">\n<h2>第1条</h2>\n<p>守ること</p>\n</div>",
		`<footer class="footer">`,
		`<nav aria-label="規約とポリシー">`,
		`<li><a href="/auth/terms">利用規約</a></li>`,
		`<li><a href="/auth/privacy">プライバシーポリシー</a></li>`,
	)
	wantLacks(t, body, "<script", "style=", "<form")
	// The page's locale goes on <html>; the document, one text for every locale in a language
	// the server does not know, is marked as of an unknown language (lang="").
	for _, lang := range []string{"ja", "en"} {
		w = b.do(http.MethodGet, authpages.PathPrivacyPolicy, nil, map[string]string{"Accept-Language": lang})
		wantContains(t, w.Body.String(), `<html lang="`+lang+`"`, `<div class="document" lang="" dir="auto">`,
			"<p>We keep <strong>little</strong>.</p>")
	}
}

func TestMissingDocumentsAreNotFoundAndNotLinked(t *testing.T) {
	docs := &fakeDocuments{docs: map[instance.TextKey]appinstance.Document{
		instance.TextPrivacyPolicy: {Key: instance.TextPrivacyPolicy, HTML: "<p>p</p>", RevisedAt: revised},
	}}
	b := newBrowser(t, newDocumentServer(t, newFakeIdentity(), docs))
	w := b.do(http.MethodGet, authpages.PathTerms, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET terms without terms = %d, want 404", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control of the 404 = %q", w.Header().Get("Cache-Control"))
	}
	wantContains(t, w.Body.String(), "This server has not published terms of service.", `<a href="/auth/privacy">Privacy policy</a>`)
	wantLacks(t, w.Body.String(), `href="/auth/terms"`)

	// Without any document, no page has the footer or the agreement.
	for _, server := range []http.Handler{
		newDocumentServer(t, newFakeIdentity(), &fakeDocuments{}),
		newServer(t, newFakeIdentity()), // no Options.Documents
	} {
		b := newBrowser(t, server)
		if w := b.do(http.MethodGet, authpages.PathPrivacyPolicy, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("GET privacy without documents = %d, want 404", w.Code)
		}
		for _, path := range []string{authpages.PathSignIn, authpages.PathSignUp} {
			wantLacks(t, b.do(http.MethodGet, path, nil, nil).Body.String(), "<footer", "sign-up-agreement", "/auth/terms", "/auth/privacy")
		}
	}
}

func TestSignUpAsksToAgreeToTheDocuments(t *testing.T) {
	docs := &fakeDocuments{docs: map[instance.TextKey]appinstance.Document{
		instance.TextTerms: {Key: instance.TextTerms, HTML: "<p>t</p>", RevisedAt: revised},
	}}
	b := newBrowser(t, newDocumentServer(t, newFakeIdentity(), docs))
	body := b.do(http.MethodGet, authpages.PathSignUp, nil, map[string]string{"Accept-Language": "ja"}).Body.String()
	wantContains(t, body,
		`<p id="sign-up-agreement">アカウントを作成すると、このサーバーの次の文書に同意したことになります。</p>`,
		// The form keeps what was typed: the agreement's links open in a new tab, which assistive technology reads.
		`<li><a href="/auth/terms" target="_blank" rel="noopener noreferrer">利用規約<span class="visually-hidden"> （新しいタブで開きます）</span></a></li>`,
		`<button class="button" type="submit" aria-describedby="sign-up-agreement">`,
		// The footer's links lead from page to page of the server in the same tab.
		`<li><a href="/auth/terms">利用規約</a></li>`,
	)
	wantLacks(t, body, "/auth/privacy")
	// The sign-in page links to them in the footer only.
	body = b.do(http.MethodGet, authpages.PathSignIn, nil, nil).Body.String()
	wantContains(t, body, `<footer class="footer">`, `<a href="/auth/terms">Terms of service</a>`)
	wantLacks(t, body, "sign-up-agreement")
}

func TestDocumentFailures(t *testing.T) {
	docs := &fakeDocuments{err: errors.New("db down")}
	b := newBrowser(t, newDocumentServer(t, newFakeIdentity(), docs))
	if w := b.do(http.MethodGet, authpages.PathTerms, nil, nil); w.Code != http.StatusInternalServerError {
		t.Errorf("GET terms with a failing store = %d, want 500", w.Code)
	}
	// The other pages still work, without the links.
	w := b.do(http.MethodGet, authpages.PathSignIn, nil, nil)
	if w.Code != http.StatusOK {
		t.Errorf("GET sign_in with a failing store = %d, want 200", w.Code)
	}
	wantLacks(t, w.Body.String(), "<footer")
}
