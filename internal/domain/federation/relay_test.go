// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/best-friends/my/internal/domain/federation"
)

func TestRelayStatusNext(t *testing.T) {
	at := time.Unix(1700000000, 0).UTC()
	const f1, f2 federation.IRI = "https://my.example/ap/activities/1", "https://my.example/ap/activities/2"
	ev := func(k federation.RelayEventKind, follow federation.IRI) federation.RelayEvent {
		e := federation.RelayEvent{ID: testID(2), RelayID: testID(1), Kind: k, FollowActivityURI: follow, OccurredAt: at}
		if k == federation.RelayAcceptReceived {
			e.RelayActorURI = "https://relay.example/actor"
		}
		return e
	}
	st := func(s federation.RelayState, follow federation.IRI) federation.RelayStatus {
		return federation.RelayStatus{RelayID: testID(1), State: s, FollowActivityURI: follow}
	}
	cases := []struct {
		name string
		from federation.RelayStatus
		e    federation.RelayEvent
		want federation.RelayState
		err  error
	}{
		{"the first Follow", federation.RelayStatus{}, ev(federation.RelayFollowSent, f1), federation.RelayPending, nil},
		{"Follow again after a rejection", st(federation.RelayRejected, f1), ev(federation.RelayFollowSent, f2), federation.RelayPending, nil},
		{"Follow again after leaving", st(federation.RelayLeft, f1), ev(federation.RelayFollowSent, f2), federation.RelayPending, nil},
		{"Follow of a pending relay", st(federation.RelayPending, f1), ev(federation.RelayFollowSent, f2), "", federation.ErrRelayNotJoinable},
		{"Follow of an accepted relay", st(federation.RelayAccepted, f1), ev(federation.RelayFollowSent, f2), "", federation.ErrRelayNotJoinable},
		{"Accept", st(federation.RelayPending, f1), ev(federation.RelayAcceptReceived, f1), federation.RelayAccepted, nil},
		{"Accept of another Follow", st(federation.RelayPending, f1), ev(federation.RelayAcceptReceived, f2), "", federation.ErrRelayNotAnswerable},
		{"Accept again", st(federation.RelayAccepted, f1), ev(federation.RelayAcceptReceived, f1), "", federation.ErrRelayNotAnswerable},
		{"Accept after leaving", st(federation.RelayLeft, f1), ev(federation.RelayAcceptReceived, f1), "", federation.ErrRelayNotAnswerable},
		{"Reject", st(federation.RelayPending, f1), ev(federation.RelayRejectReceived, f1), federation.RelayRejected, nil},
		{"Reject after the Accept", st(federation.RelayAccepted, f1), ev(federation.RelayRejectReceived, f1), federation.RelayRejected, nil},
		{"Reject of another Follow", st(federation.RelayAccepted, f1), ev(federation.RelayRejectReceived, f2), "", federation.ErrRelayNotAnswerable},
		{"leave a pending relay", st(federation.RelayPending, f1), ev(federation.RelayLeftSent, f1), federation.RelayLeft, nil},
		{"leave an accepted relay", st(federation.RelayAccepted, f1), ev(federation.RelayLeftSent, f1), federation.RelayLeft, nil},
		{"leave a left relay", st(federation.RelayLeft, f1), ev(federation.RelayLeftSent, f1), "", federation.ErrRelayNotJoined},
		{"leave a rejected relay", st(federation.RelayRejected, f1), ev(federation.RelayLeftSent, f1), "", federation.ErrRelayNotJoined},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.Next(tc.e)
			if !errors.Is(err, tc.err) || got.State != tc.want {
				t.Fatalf("Next = %+v, %v; want %s, %v", got, err, tc.want, tc.err)
			}
			if err == nil && (got.FollowActivityURI != tc.e.FollowActivityURI || got.RelayActorURI != tc.e.RelayActorURI || !got.Since.Equal(at)) {
				t.Errorf("Next = %+v, want the event's Follow, actor, and time", got)
			}
		})
	}
	bad := ev(federation.RelayFollowSent, f1)
	bad.RelayActorURI = "https://relay.example/actor"
	if _, err := (federation.RelayStatus{}).Next(bad); !errors.Is(err, federation.ErrInvalidRelay) {
		t.Errorf("Next of a follow_sent with an actor = %v, want ErrInvalidRelay", err)
	}
}

func TestRelayValidate(t *testing.T) {
	ok := federation.Relay{
		ID: testID(1), Protocol: federation.RelayMastodon, URL: "https://relay.example/inbox", InboxURL: "https://relay.example/inbox",
		CreatedAt: time.Unix(1, 0),
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	for name, mod := range map[string]func(*federation.Relay){
		"protocol":   func(r *federation.Relay) { r.Protocol = "pleroma" },
		"http":       func(r *federation.Relay) { r.URL = "http://relay.example/inbox" },
		"no host":    func(r *federation.Relay) { r.InboxURL = "https:///inbox" },
		"no created": func(r *federation.Relay) { r.CreatedAt = time.Time{} },
	} {
		r := ok
		mod(&r)
		if err := r.Validate(); !errors.Is(err, federation.ErrInvalidRelay) {
			t.Errorf("%s: Validate = %v, want ErrInvalidRelay", name, err)
		}
	}
}

func TestRelayDocuments(t *testing.T) {
	at := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	const instance = "https://my.example/ap/actors/instance"
	mastodon := federation.Relay{Protocol: federation.RelayMastodon, URL: "https://relay.example/inbox"}
	lite := federation.Relay{Protocol: federation.RelayLitePub, URL: "https://relay.example/actor"}
	doc := func(d federation.ActivityDocument) map[string]any {
		b, err := d.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	f := doc(federation.RelayFollowDocument("https://my.example/ap/activities/1", instance, mastodon, at))
	if f["type"] != "Follow" || f["object"] != string(federation.PublicIRI) || f["actor"] != instance || len(f["to"].([]any)) != 0 {
		t.Errorf("Mastodon Follow = %v, want a Follow of Public addressed to no one", f)
	}
	if f := doc(federation.RelayFollowDocument("https://my.example/ap/activities/1", instance, lite, at)); f["object"] != "https://relay.example/actor" ||
		len(f["to"].([]any)) != 1 || f["to"].([]any)[0] != "https://relay.example/actor" {
		t.Errorf("LitePub Follow = %v, want a Follow of the relay actor addressed to it", f)
	}
	u := doc(federation.RelayUndoDocument("https://my.example/ap/activities/2", "https://my.example/ap/activities/1", instance, mastodon, time.Time{}, at))
	inner, _ := u["object"].(map[string]any)
	if u["type"] != "Undo" || inner["id"] != "https://my.example/ap/activities/1" || inner["type"] != "Follow" || inner["@context"] != nil {
		t.Errorf("Undo = %v, want the embedded Follow", u)
	}
	a := doc(federation.FollowAnswerDocument("https://my.example/ap/activities/3", federation.ActivityAccept, instance, "https://relay.example/actor", "https://relay.example/follows/1", at))
	inner, _ = a["object"].(map[string]any)
	if a["type"] != "Accept" || inner["id"] != "https://relay.example/follows/1" || inner["actor"] != "https://relay.example/actor" || inner["object"] != instance {
		t.Errorf("Accept = %v, want the relay's Follow of the instance actor", a)
	}
}

func TestSignsLinkedData(t *testing.T) {
	public := federation.Audience{To: []federation.IRI{federation.PublicIRI}}
	unlisted := federation.Audience{To: []federation.IRI{"https://my.example/f"}, Cc: []federation.IRI{federation.PublicIRI}}
	cases := []struct {
		kind federation.OutboundObjectKind
		t    federation.ActivityType
		aud  federation.Audience
		want bool
	}{
		{federation.OutboundStatus, federation.ActivityCreate, public, true},
		{federation.OutboundStatus, federation.ActivityUpdate, public, true},
		{federation.OutboundStatus, federation.ActivityDelete, public, true},
		{federation.OutboundStatus, federation.ActivityCreate, unlisted, false},
		{federation.OutboundBoost, federation.ActivityAnnounce, public, false},
		{federation.OutboundAccount, federation.ActivityUpdate, public, false},
	}
	for _, tc := range cases {
		if got := federation.SignsLinkedData(federation.OutboundActivity{ObjectKind: tc.kind, Type: tc.t}, tc.aud); got != tc.want {
			t.Errorf("SignsLinkedData(%s %s, %v) = %v, want %v", tc.t, tc.kind, tc.aud, got, tc.want)
		}
	}
}
