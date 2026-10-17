// SPDX-License-Identifier: AGPL-3.0-only

package activitypub_test

import (
	"context"
	"net/http"
	"testing"
)

// fakeSuspensions is an activitypub.Suspensions that suspends the accounts it holds.
type fakeSuspensions map[string]bool

func (f fakeSuspensions) AccountFederationSuspended(_ context.Context, accountID string) (bool, error) {
	return f[accountID], nil
}

// TestSuspendedActor checks GET of a local actor that moderation suspends (federation.md §11):
// the emptied actor marked suspended, with its keys, also while its lifecycle hides it (a ban
// schedules the deletion), and 404 once it is deleted.
func TestSuspendedActor(t *testing.T) {
	s := newServer(t, accountDomain)
	for _, id := range []string{aliceID, carolID, daveID} {
		s.suspended[id] = true
	}
	rec := s.get(t, "/ap/actors/"+aliceID, acceptMastodon)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/activity+json" {
		t.Fatalf("GET of the suspended alice = %d %v", rec.Code, rec.Header())
	}
	checkGolden(t, "actor-suspended.json", rec.Body.Bytes())

	rec = s.get(t, "/ap/actors/"+carolID, acceptMastodon)
	if doc := decode(t, rec.Body.Bytes()); rec.Code != http.StatusOK || doc["suspended"] != true || doc["publicKey"] == nil {
		t.Errorf("GET of the suspended, deactivated carol = %d %s, want her suspended actor", rec.Code, rec.Body)
	}
	if rec := s.get(t, "/ap/actors/"+daveID, acceptMastodon); rec.Code != http.StatusNotFound {
		t.Errorf("GET of the suspended, deleted dave = %d, want 404", rec.Code)
	}

	// The suspension ends: the profile is back.
	s.suspended[aliceID] = false
	rec = s.get(t, "/ap/actors/"+aliceID, acceptMastodon)
	if doc := decode(t, rec.Body.Bytes()); rec.Code != http.StatusOK || doc["suspended"] != nil || doc["name"] != "Alice" {
		t.Errorf("GET of alice after the suspension = %d %s, want her profile", rec.Code, rec.Body)
	}
}
