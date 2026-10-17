// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"errors"
	"testing"
)

func TestTypes(t *testing.T) {
	for _, typ := range Types {
		if !typ.Valid() {
			t.Errorf("%q is not valid", typ)
		}
	}
	for _, typ := range []Type{"", "poke", "Follow"} {
		if typ.Valid() {
			t.Errorf("%q is valid", typ)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Notification{ID: "n", RecipientAccountID: "a", SourceAccountID: "b", Type: TypeFollow, DedupeKey: FollowKey(TypeFollow, "f")}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate(%+v) = %v", ok, err)
	}
	fromInstance := ok
	fromInstance.SourceAccountID, fromInstance.Type, fromInstance.ModerationActionID = "", TypeModerationNotice, "m"
	if err := fromInstance.Validate(); err != nil {
		t.Errorf("Validate(%+v) = %v", fromInstance, err)
	}
	noAction := fromInstance
	noAction.ModerationActionID = ""
	strayAction := ok
	strayAction.ModerationActionID = "m"
	for _, n := range []Notification{noAction, strayAction} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error for the moderation action", n)
		}
	}
	self := ok
	self.SourceAccountID = "a"
	if err := self.Validate(); !errors.Is(err, ErrSelfNotification) {
		t.Errorf("Validate(self) = %v, want ErrSelfNotification", err)
	}
	unknown := ok
	unknown.Type = "poke"
	if err := unknown.Validate(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("Validate(poke) = %v, want ErrInvalidType", err)
	}
	for _, n := range []Notification{{RecipientAccountID: "a", Type: TypeFollow, DedupeKey: "k"}, {ID: "n", Type: TypeFollow, DedupeKey: "k"}, {ID: "n", RecipientAccountID: "a", Type: TypeFollow}} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error", n)
		}
	}
}

func TestKeys(t *testing.T) {
	for got, want := range map[string]string{
		FollowKey(TypeFollowRequest, "f1"):        "follow_request:f1",
		MentionKey("s1"):                          "mention:s1",
		InteractionKey(TypeFavourite, "s1", "a1"): "favourite:s1:a1",
		InteractionKey(TypeBoost, "s1", "a1"):     "boost:s1:a1",
		ModerationNoticeKey("m1"):                 "moderation_notice:m1",
	} {
		if got != want {
			t.Errorf("key = %q, want %q", got, want)
		}
	}
}
