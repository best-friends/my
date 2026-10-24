// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mycroblog/my/api"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

func TestServeOpenAPI(t *testing.T) {
	_, h := newServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/openapi.yaml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), api.Spec) {
		t.Error("the body is not the bundled spec")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Error("Cache-Control is empty")
	}
}

func TestDeprecationHeaders(t *testing.T) {
	const (
		deprecated = 1793491200 // 2026-11-01
		sunset     = 1801440000 // 2027-02-01
	)
	s, _ := newServer(t)
	clientapi.WithOperationAccess(t, "GET /api/v1/old", "getOld", true, deprecated, sunset)
	clientapi.WithOperationAccess(t, "GET /api/v1/new", "getNew", true, 0, 0)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	old := httptest.NewRecorder()
	clientapi.Guard(s, "GET /api/v1/old", ok).ServeHTTP(old, httptest.NewRequest(http.MethodGet, "/api/v1/old", nil))
	if got := old.Header().Get("Deprecation"); got != "@1793491200" {
		t.Errorf("Deprecation = %q, want @1793491200", got)
	}
	if got := old.Header().Get("Sunset"); got != "Mon, 01 Feb 2027 00:00:00 GMT" {
		t.Errorf("Sunset = %q", got)
	}

	cur := httptest.NewRecorder()
	clientapi.Guard(s, "GET /api/v1/new", ok).ServeHTTP(cur, httptest.NewRequest(http.MethodGet, "/api/v1/new", nil))
	if cur.Code != http.StatusNoContent {
		t.Fatalf("status = %d", cur.Code)
	}
	for _, name := range []string{"Deprecation", "Sunset"} {
		if v := cur.Header().Get(name); v != "" {
			t.Errorf("%s = %q on an operation that is not deprecated", name, v)
		}
	}
}
