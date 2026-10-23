// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"errors"
	"testing"
	"time"
)

func TestBlockURI(t *testing.T) {
	got := BlockURI("social.example", "0192f3c1-0000-7000-8000-000000000000")
	if want := "https://social.example/ap/activities/0192f3c1-0000-7000-8000-000000000000"; got != want {
		t.Errorf("BlockURI = %q, want %q", got, want)
	}
}

func TestBlockValidate(t *testing.T) {
	if err := (Block{BlockerAccountID: idA, BlockedAccountID: idA}).Validate(); !errors.Is(err, ErrSelfBlock) {
		t.Errorf("self block: %v, want ErrSelfBlock", err)
	}
	if err := (Block{BlockerAccountID: idA}).Validate(); err == nil {
		t.Error("missing blocked account: want an error")
	}
	if err := (Block{BlockerAccountID: idA, BlockedAccountID: idB}).Validate(); err != nil {
		t.Errorf("valid block: %v", err)
	}
}

func TestBlockHistoryEdge(t *testing.T) {
	b := Block{ID: idC, BlockerAccountID: idX, BlockedAccountID: idY}
	if e, ok := (BlockHistory{Block: b}).Edge(); !ok || e != (BlockEdge{BlockerAccountID: idX, BlockedAccountID: idY, BlockID: idC}) {
		t.Errorf("Edge() = %+v, %v; want the block's edge", e, ok)
	}
	if _, ok := (BlockHistory{Block: b, Retraction: &BlockRetraction{BlockID: idC}}).Edge(); ok {
		t.Error("Edge() of a retracted block: want none")
	}
}

func TestMute(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	m := Mute{AccountID: idA, TargetAccountID: idB, CreatedAt: now}
	if err := m.Validate(); err != nil {
		t.Errorf("valid mute: %v", err)
	}
	if !m.Active(later.Add(24 * time.Hour)) {
		t.Error("a mute without expiry: want active forever")
	}
	m.ExpiresAt = &later
	if err := m.Validate(); err != nil {
		t.Errorf("valid expiring mute: %v", err)
	}
	if !m.Active(later.Add(-time.Millisecond)) || m.Active(later) || m.Active(later.Add(time.Second)) {
		t.Error("an expiring mute: want active before ExpiresAt only")
	}
	m.ExpiresAt = &now
	if err := m.Validate(); !errors.Is(err, ErrMuteExpiry) {
		t.Errorf("expiry at creation: %v, want ErrMuteExpiry", err)
	}
	if err := (Mute{AccountID: idA, TargetAccountID: idA}).Validate(); !errors.Is(err, ErrSelfMute) {
		t.Errorf("self mute: %v, want ErrSelfMute", err)
	}
	if err := (Mute{AccountID: idA}).Validate(); err == nil {
		t.Error("missing target: want an error")
	}
}
