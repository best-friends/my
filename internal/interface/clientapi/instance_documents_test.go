// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeDocuments holds rendered documents by key.
type fakeDocuments map[instance.TextKey]appinstance.Document

func (f fakeDocuments) Document(_ context.Context, key instance.TextKey) (appinstance.Document, bool, error) {
	d, ok := f[key]
	return d, ok, nil
}

func TestGetInstanceDocument(t *testing.T) {
	revised := time.Date(2026, 10, 1, 14, 30, 0, 0, time.UTC)
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Documents = fakeDocuments{
			instance.TextTerms: {Key: instance.TextTerms, HTML: "<h2>1</h2>\n<p>Be kind</p>", RevisedAt: revised},
		}
	})

	res := e.get(t, "/instance/documents/terms", map[string]string{"Accept-Language": "ja"})
	got := decodeJSON[clientapi.InstanceDocument](t, res)
	if res.StatusCode != http.StatusOK || got.Kind != clientapi.InstanceDocumentKind("terms") || got.Html != "<h2>1</h2>\n<p>Be kind</p>" || !got.RevisedAt.Equal(revised) {
		t.Errorf("GET terms = %d %+v", res.StatusCode, got)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=60" {
		t.Errorf("Cache-Control = %q, want public, max-age=60", cc)
	}

	// A document that is not published, and a key that is not a document, are a 404.
	for _, kind := range []string{"privacy_policy", "commerce_disclosure", "name", "nope"} {
		if res := e.get(t, "/instance/documents/"+kind, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", kind, res.StatusCode)
		}
	}
}
