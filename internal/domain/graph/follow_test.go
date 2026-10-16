// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"errors"
	"testing"
	"time"
)

func TestFollowURI(t *testing.T) {
	got := FollowURI("social.example", "0192f3c1-0000-7000-8000-000000000000")
	if want := "https://social.example/ap/activities/0192f3c1-0000-7000-8000-000000000000"; got != want {
		t.Errorf("FollowURI = %q, want %q", got, want)
	}
}

func TestFollowValidate(t *testing.T) {
	if err := (Follow{FollowerAccountID: "a", FolloweeAccountID: "a"}).Validate(); !errors.Is(err, ErrSelfFollow) {
		t.Errorf("self follow: %v, want ErrSelfFollow", err)
	}
	if err := (Follow{FollowerAccountID: "a"}).Validate(); err == nil {
		t.Error("missing followee: want an error")
	}
	if err := (Follow{FollowerAccountID: "a", FolloweeAccountID: "b"}).Validate(); err != nil {
		t.Errorf("valid follow: %v", err)
	}
}

func TestHistoryEdge(t *testing.T) {
	requested := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	accepted := requested.Add(time.Minute)
	f := Follow{ID: "f", FollowerAccountID: "a", FolloweeAccountID: "b", RequestedAt: requested}
	acc := &Acceptance{FollowID: "f", AcceptedBy: AcceptedByFollowee, AcceptedAt: accepted}
	cases := []struct {
		name   string
		h      History
		want   Edge
		wantOK bool
	}{
		{"pending", History{Follow: f}, Edge{"a", "b", "f", StatePending, requested}, true},
		{"accepted", History{Follow: f, Acceptance: acc}, Edge{"a", "b", "f", StateActive, accepted}, true},
		{"rejected", History{Follow: f, Rejection: &Rejection{FollowID: "f"}}, Edge{}, false},
		{"terminated", History{Follow: f, Acceptance: acc, Termination: &Termination{FollowID: "f"}}, Edge{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.h.Edge()
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("Edge() = %+v, %v; want %+v, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestCountEdges(t *testing.T) {
	got := CountEdges([]Edge{
		{FollowerAccountID: "a", FolloweeAccountID: "b", State: StateActive},
		{FollowerAccountID: "c", FolloweeAccountID: "b", State: StateActive},
		{FollowerAccountID: "b", FolloweeAccountID: "a", State: StatePending},
	})
	want := map[string]Counters{
		"a": {AccountID: "a", FollowingCount: 1},
		"b": {AccountID: "b", FollowersCount: 2},
		"c": {AccountID: "c", FollowingCount: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("CountEdges = %+v, want %+v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("CountEdges[%s] = %+v, want %+v", k, got[k], w)
		}
	}
}
