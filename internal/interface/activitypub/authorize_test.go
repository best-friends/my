// SPDX-License-Identifier: AGPL-3.0-only

package activitypub_test

import (
	"net/http"
	"testing"
)

// TestAuthorizedFetch checks the answers with authorized fetch (FR-408) off and on, for a
// request without a verified signature and for a signed one.
func TestAuthorizedFetch(t *testing.T) {
	const (
		public  = "public, max-age=60"
		private = "private, no-store"
		noStore = "no-store" // the problem writer's
	)
	actor := "/ap/actors/" + aliceID
	object := "/ap/objects/" + publicStatusID
	tests := []struct {
		name, target string
		// off and on are the answers with authorized fetch off and on, unsigned then signed.
		off, on [2]answer
	}{
		{"user actor", actor, [2]answer{{200, public, "full"}, {200, public, "full"}}, [2]answer{{200, private, "key"}, {200, private, "full"}}},
		{"instance actor", "/ap/actors/" + instanceID, [2]answer{{200, public, "full"}, {200, public, "full"}}, [2]answer{{200, public, "full"}, {200, public, "full"}}},
		{"missing actor", "/ap/actors/" + missingID, [2]answer{{404, noStore, ""}, {404, noStore, ""}}, [2]answer{{404, noStore, ""}, {404, noStore, ""}}},
		{"public note", object, [2]answer{{200, public, "Note"}, {200, public, "Note"}}, [2]answer{{401, noStore, ""}, {200, private, "Note"}}},
		{"public create", object + "/activity", [2]answer{{200, public, "Create"}, {200, public, "Create"}}, [2]answer{{401, noStore, ""}, {200, private, "Create"}}},
		{"followers-only note", "/ap/objects/" + followersOnlyID, [2]answer{{404, noStore, ""}, {200, private, "Note"}}, [2]answer{{401, noStore, ""}, {200, private, "Note"}}},
		{"missing note", "/ap/objects/" + missingStatusID, [2]answer{{404, noStore, ""}, {404, noStore, ""}}, [2]answer{{401, noStore, ""}, {404, noStore, ""}}},
		{"deleted note", "/ap/objects/" + deletedStatusID, [2]answer{{410, public, "Tombstone"}, {410, public, "Tombstone"}}, [2]answer{{401, noStore, ""}, {410, private, "Tombstone"}}},
		{"outbox", actor + "/outbox", [2]answer{{200, public, "OrderedCollection"}, {200, public, "OrderedCollection"}}, [2]answer{{401, noStore, ""}, {200, private, "OrderedCollection"}}},
		{"outbox page", actor + "/outbox?page=true", [2]answer{{200, public, "OrderedCollectionPage"}, {200, public, "OrderedCollectionPage"}}, [2]answer{{401, noStore, ""}, {200, private, "OrderedCollectionPage"}}},
		{"followers", actor + "/followers", [2]answer{{200, public, "OrderedCollection"}, {200, public, "OrderedCollection"}}, [2]answer{{401, noStore, ""}, {200, private, "OrderedCollection"}}},
		{"following", actor + "/following", [2]answer{{200, public, "OrderedCollection"}, {200, public, "OrderedCollection"}}, [2]answer{{401, noStore, ""}, {200, private, "OrderedCollection"}}},
		{"featured", actor + "/featured", [2]answer{{200, public, "OrderedCollection"}, {200, public, "OrderedCollection"}}, [2]answer{{401, noStore, ""}, {200, private, "OrderedCollection"}}},
		{"featured tags", actor + "/featured-tags", [2]answer{{200, public, "Collection"}, {200, public, "Collection"}}, [2]answer{{401, noStore, ""}, {200, private, "Collection"}}},
		{"collection of a missing actor", "/ap/actors/" + missingID + "/outbox", [2]answer{{404, noStore, ""}, {404, noStore, ""}}, [2]answer{{401, noStore, ""}, {404, noStore, ""}}},
	}
	for _, af := range []bool{false, true} {
		s := newServer(t, accountDomain)
		s.objects.authorizedFetch = af
		for _, tt := range tests {
			want := tt.off
			if af {
				want = tt.on
			}
			for i, signer := range []string{"", followerDomain} {
				rec := s.getSigned(t, tt.target, acceptMastodon, signer)
				got := answer{rec.Code, rec.Header().Get("Cache-Control"), ""}
				if rec.Code < 300 || rec.Code == http.StatusGone {
					got.body = kindOf(t, rec.Body.Bytes())
				}
				if got != want[i] {
					t.Errorf("authorized fetch %v, %s signed by %q = %+v, want %+v", af, tt.name, signer, got, want[i])
				}
				refused := rec.Code == http.StatusUnauthorized
				if acc := rec.Header().Get("Accept-Signature"); refused != (acc == `sig1=("@method" "@target-uri");created`) {
					t.Errorf("authorized fetch %v, %s signed by %q: Accept-Signature %q on %d", af, tt.name, signer, acc, rec.Code)
				}
			}
		}
	}
}

// TestAuthorizedFetchKeyDocument checks the actor document that authorized fetch serves to a
// request without a verified signature.
func TestAuthorizedFetchKeyDocument(t *testing.T) {
	s := newServer(t, accountDomain)
	s.objects.authorizedFetch = true
	rec := s.getSigned(t, "/ap/actors/"+aliceID, acceptMastodon, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	checkGolden(t, "actor-key-document.json", rec.Body.Bytes())

	// The HTML redirect does not depend on the signature.
	if rec := s.get(t, "/ap/actors/"+aliceID, acceptBrowser); rec.Code != http.StatusSeeOther {
		t.Errorf("GET as HTML = %d, want 303", rec.Code)
	}
	if rec := s.get(t, "/ap/objects/"+publicStatusID, acceptBrowser); rec.Code != http.StatusSeeOther {
		t.Errorf("GET of a note as HTML = %d, want 303", rec.Code)
	}
}

// answer is the part of a response that TestAuthorizedFetch compares.
type answer struct {
	status int
	cache  string
	// body is the type of the document, "full" or "key" for an actor document.
	body string
}

// kindOf names a document: its type, or for an actor whether it is the full document.
func kindOf(t *testing.T, body []byte) string {
	t.Helper()
	doc := decode(t, body)
	if _, ok := doc["publicKey"]; ok {
		if _, ok := doc["outbox"]; ok {
			return "full"
		}
		return "key"
	}
	typ, _ := doc["type"].(string)
	return typ
}
