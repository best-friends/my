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
	if err := (Follow{FollowerAccountID: idA, FolloweeAccountID: idA}).Validate(); !errors.Is(err, ErrSelfFollow) {
		t.Errorf("self follow: %v, want ErrSelfFollow", err)
	}
	if err := (Follow{FollowerAccountID: idA}).Validate(); err == nil {
		t.Error("missing followee: want an error")
	}
	if err := (Follow{FollowerAccountID: idA, FolloweeAccountID: idB}).Validate(); err != nil {
		t.Errorf("valid follow: %v", err)
	}
}

func TestHistoryEdge(t *testing.T) {
	requested := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	accepted := requested.Add(time.Minute)
	f := Follow{ID: idF, FollowerAccountID: idA, FolloweeAccountID: idB, RequestedAt: requested}
	acc := &Acceptance{FollowID: idF, AcceptedBy: AcceptedByFollowee, AcceptedAt: accepted}
	cases := []struct {
		name   string
		h      History
		want   Edge
		wantOK bool
	}{
		{"pending", History{Follow: f}, Edge{idA, idB, idF, StatePending, requested}, true},
		{"accepted", History{Follow: f, Acceptance: acc}, Edge{idA, idB, idF, StateActive, accepted}, true},
		{"rejected", History{Follow: f, Rejection: &Rejection{FollowID: idF}}, Edge{}, false},
		{"terminated", History{Follow: f, Acceptance: acc, Termination: &Termination{FollowID: idF}}, Edge{}, false},
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
		{FollowerAccountID: idA, FolloweeAccountID: idB, State: StateActive},
		{FollowerAccountID: idC, FolloweeAccountID: idB, State: StateActive},
		{FollowerAccountID: idB, FolloweeAccountID: idA, State: StatePending},
	})
	want := map[string]Counters{
		idA: {AccountID: idA, FollowingCount: 1},
		idB: {AccountID: idB, FollowersCount: 2},
		idC: {AccountID: idC, FollowingCount: 1},
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
