// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"testing"

	"github.com/mycroblog/my/internal/domain/federation"
)

func TestQuoteRequestStatus(t *testing.T) {
	const id = "01890a5d-ac96-774b-bcce-b302099a8057"
	h := &QuoteHandlers{origin: "https://social.example"}
	tests := []struct {
		name string
		iri  string
		want string
		ok   bool
	}{
		{"canonical", "https://social.example/ap/objects/" + id + "/quote_request", id, true},
		{"uppercase", "https://social.example/ap/objects/01890A5D-AC96-774B-BCCE-B302099A8057/quote_request", "", false},
		{"no hyphens", "https://social.example/ap/objects/01890a5dac96774bbcceb302099a8057/quote_request", "", false},
		{"braces", "https://social.example/ap/objects/{" + id + "}/quote_request", "", false},
		{"uuidv4", "https://social.example/ap/objects/3f2b8c1e-9d4a-4e6b-8a3c-1f2e3d4c5b6a/quote_request", "", false},
		{"empty", "https://social.example/ap/objects//quote_request", "", false},
		{"garbage", "https://social.example/ap/objects/x/quote_request", "", false},
		{"nested", "https://social.example/ap/objects/" + id + "/x/quote_request", "", false},
		{"other host", "https://other.example/ap/objects/" + id + "/quote_request", "", false},
		{"other segment", "https://social.example/ap/objects/" + id + "/activity", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := h.quoteRequestStatus(federation.IRI(tt.iri))
			if got != tt.want || ok != tt.ok {
				t.Errorf("quoteRequestStatus(%q) = %q, %v, want %q, %v", tt.iri, got, ok, tt.want, tt.ok)
			}
		})
	}
}
