// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/federation"
)

func TestAddress(t *testing.T) {
	const followers federation.IRI = "https://social.example/ap/actors/a/followers"
	m1, m2 := federation.IRI("https://remote.example/users/m1"), federation.IRI("https://other.example/users/m2")
	public := federation.PublicIRI
	cases := []struct {
		v      federation.Visibility
		to, cc []federation.IRI
	}{
		{federation.VisibilityPublic, []federation.IRI{public}, []federation.IRI{followers, m1, m2}},
		{federation.VisibilityUnlisted, []federation.IRI{followers}, []federation.IRI{public, m1, m2}},
		{federation.VisibilityPrivate, []federation.IRI{followers}, []federation.IRI{m1, m2}},
		// A local post reaches the author's followers only (ADR-0017).
		{federation.VisibilityLocal, []federation.IRI{followers}, []federation.IRI{}},
		{federation.VisibilityDirect, []federation.IRI{m1, m2}, []federation.IRI{}},
		{"bogus", []federation.IRI{}, []federation.IRI{}},
	}
	for _, tc := range cases {
		t.Run(string(tc.v), func(t *testing.T) {
			// Duplicates and empty IRIs are dropped; a recipient in to is not repeated in cc.
			got := federation.Address(tc.v, followers, []federation.IRI{m1, "", m2, m1})
			if !slices.Equal(got.To, tc.to) || !slices.Equal(got.Cc, tc.cc) {
				t.Errorf("Address(%s) = to %v cc %v, want to %v cc %v", tc.v, got.To, got.Cc, tc.to, tc.cc)
			}
			if got.To == nil || got.Cc == nil {
				t.Errorf("Address(%s) returned a nil slice", tc.v)
			}
		})
	}
	if got := federation.Address(federation.VisibilityDirect, followers, []federation.IRI{followers}); !slices.Equal(got.To, []federation.IRI{followers}) {
		t.Errorf("Address(direct, followers mentioned) = %v", got)
	}
}

func TestDeliverable(t *testing.T) {
	available := federation.DomainStatus{State: federation.InstanceAvailable}
	unknown := federation.DomainStatus{}
	unavailable := federation.DomainStatus{State: federation.InstanceUnavailable}
	suspended := federation.DomainStatus{Suspended: true, State: federation.InstanceAvailable}
	for _, tc := range []struct {
		t    federation.ActivityType
		s    federation.DomainStatus
		want bool
	}{
		{federation.ActivityCreate, available, true},
		{federation.ActivityCreate, unknown, true},
		{federation.ActivityCreate, unavailable, false},
		{federation.ActivityAnnounce, unavailable, false},
		{federation.ActivityFollow, unavailable, true},
		{federation.ActivityAccept, unavailable, true},
		{federation.ActivityReject, unavailable, true},
		{federation.ActivityFollow, suspended, false},
		{federation.ActivityDelete, suspended, false},
	} {
		if got := federation.Deliverable(tc.t, tc.s); got != tc.want {
			t.Errorf("Deliverable(%s, %+v) = %v, want %v", tc.t, tc.s, got, tc.want)
		}
	}
}

func TestInboxFor(t *testing.T) {
	if got := federation.InboxFor("https://r.example/users/a/inbox", "https://r.example/inbox"); got != "https://r.example/inbox" {
		t.Errorf("InboxFor with a shared inbox = %s", got)
	}
	if got := federation.InboxFor("https://r.example/users/a/inbox", ""); got != "https://r.example/users/a/inbox" {
		t.Errorf("InboxFor without a shared inbox = %s", got)
	}
}

func TestOutboundActivityValidate(t *testing.T) {
	ok := federation.OutboundActivity{
		ID: testID(1), ActivityURI: federation.IRI("https://social.example/ap/activities/" + testID(1)), Type: federation.ActivityCreate,
		ActorAccountID: testID(2), ObjectKind: federation.OutboundStatus, ObjectID: testID(3), CreatedAt: time.Unix(1, 0),
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate(valid) = %v", err)
	}
	bad := ok
	bad.ObjectKind = "note"
	if err := bad.Validate(); !errors.Is(err, federation.ErrInvalidOutbound) {
		t.Errorf("Validate(unknown kind) = %v", err)
	}
	bad = ok
	bad.ActorAccountID = ""
	if err := bad.Validate(); !errors.Is(err, federation.ErrInvalidOutbound) {
		t.Errorf("Validate(no actor) = %v", err)
	}
}

func TestActivityDocument(t *testing.T) {
	aud := federation.Address(federation.VisibilityPublic, "https://social.example/ap/actors/a/followers", nil)
	published := time.Date(2026, 10, 17, 12, 0, 0, 999_000_000, time.FixedZone("JST", 9*3600))
	doc := federation.NewActivityDocument("https://social.example/ap/activities/1", federation.ActivityAnnounce,
		"https://social.example/ap/actors/a", published, aud, federation.IRI("https://remote.example/notes/1"))
	b, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["published"] != "2026-10-17T03:00:00Z" || got["object"] != "https://remote.example/notes/1" || got["@context"] == nil {
		t.Errorf("document = %s", b)
	}

	undo := federation.NewActivityDocument("https://social.example/ap/activities/2", federation.ActivityUndo,
		"https://social.example/ap/actors/a", time.Time{}, aud, doc.Embedded())
	b, err = undo.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	inner, _ := got["object"].(map[string]any)
	if _, has := got["published"]; has || inner["@context"] != nil || inner["type"] != "Announce" {
		t.Errorf("Undo document = %s, want no published and the Announce embedded without a context", b)
	}

	// Every document gets its own context value.
	a, c := federation.LDContext(), federation.LDContext()
	a[0] = "changed"
	if c[0] != "https://www.w3.org/ns/activitystreams" {
		t.Error("LDContext values are shared")
	}
}
