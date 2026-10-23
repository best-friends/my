// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"errors"
	"testing"
	"time"
)

func TestSeveranceValidate(t *testing.T) {
	at := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	ok := Severance{ID: testID(31), DomainActionID: testID(20), AccountID: testID(11), FollowersLost: 1, OccurredAt: at}
	ofAccount := ok
	ofAccount.DomainActionID, ofAccount.ActionID, ofAccount.FollowersLost, ofAccount.FollowingLost = "", testID(1), 0, 1
	for _, s := range []Severance{ok, ofAccount} {
		if err := s.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", s, err)
		}
	}
	both := ok
	both.ActionID = testID(1)
	neither := ok
	neither.DomainActionID = ""
	nothingLost := ok
	nothingLost.FollowersLost = 0
	negative := ok
	negative.FollowingLost = -1
	noID := ok
	noID.ID = ""
	noAccount := ok
	noAccount.AccountID = ""
	for _, s := range []Severance{both, neither, nothingLost, negative, noID, noAccount} {
		if err := s.Validate(); !errors.Is(err, ErrInvalidSeverance) {
			t.Errorf("Validate(%+v) = %v, want ErrInvalidSeverance", s, err)
		}
	}
}
