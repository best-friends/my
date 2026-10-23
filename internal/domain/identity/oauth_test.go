// SPDX-License-Identifier: AGPL-3.0-only

package identity_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
)

func TestKnownScopesAreSortedAndParse(t *testing.T) {
	known := identity.KnownScopes()
	if !slices.IsSorted(known) {
		t.Errorf("KnownScopes = %v, want sorted", known)
	}
	for _, s := range known {
		if got, err := identity.ParseScope(s); err != nil || string(got) != s {
			t.Errorf("ParseScope(%q) = %q, %v", s, got, err)
		}
	}
	if _, err := identity.ParseScope("read:everything"); !errors.Is(err, identity.ErrInvalidScope) {
		t.Errorf("ParseScope(read:everything) = %v, want ErrInvalidScope", err)
	}
}

func TestScopeGrants(t *testing.T) {
	tests := []struct {
		scope identity.Scope
		want  string
		ok    bool
	}{
		{"read", identity.ScopeReadAccounts, true},
		{"read", identity.ScopeRead, true},
		{"read", identity.ScopeWriteStatuses, false},
		{"write", identity.ScopeWriteMedia, true},
		{"write", identity.ScopeReadStatuses, false},
		{"follow", identity.ScopeReadFollows, true},
		{"follow", identity.ScopeWriteMutes, true},
		{"follow", identity.ScopeWriteStatuses, false},
		{"follow", identity.ScopeReadAccounts, false},
		{"read:accounts", identity.ScopeReadAccounts, true},
		{"read:accounts", identity.ScopeRead, false},
		{"read:accounts", identity.ScopeReadStatuses, false},
		{"push", identity.ScopeReadNotifications, false},
		{"", identity.ScopeReadAccounts, false},
	}
	for _, tt := range tests {
		if got := tt.scope.Grants(tt.want); got != tt.ok {
			t.Errorf("Scope(%q).Grants(%q) = %v, want %v", tt.scope, tt.want, got, tt.ok)
		}
	}
}

func TestScopeWithinFollowsBroadScopes(t *testing.T) {
	if !identity.Scope("read:accounts write:statuses").Within(identity.AllScopes()) {
		t.Error("fine-grained scopes are not within AllScopes")
	}
	if identity.Scope("read write").Within("read:accounts write") {
		t.Error("read is within read:accounts")
	}
	for _, s := range identity.KnownScopes() {
		if !identity.Scope(s).Within(identity.AllScopes()) {
			t.Errorf("%s is not within AllScopes", s)
		}
	}
}
