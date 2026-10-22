// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"errors"
	"slices"
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
	resolved := ok
	resolved.SourceAccountID, resolved.Type, resolved.ReportID = "", TypeReportResolved, "r"
	if err := resolved.Validate(); err != nil {
		t.Errorf("Validate(%+v) = %v", resolved, err)
	}
	noReport := resolved
	noReport.ReportID = ""
	strayReport := ok
	strayReport.ReportID = "r"
	for _, n := range []Notification{noReport, strayReport} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error for the report", n)
		}
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
	severed := ok
	severed.SourceAccountID, severed.Type, severed.SeveranceID = "", TypeRelationshipsSevered, "s"
	if err := severed.Validate(); err != nil {
		t.Errorf("Validate(%+v) = %v", severed, err)
	}
	noSeverance := severed
	noSeverance.SeveranceID = ""
	straySeverance := ok
	straySeverance.SeveranceID = "s"
	severedBySomeone := severed
	severedBySomeone.SourceAccountID = "b"
	for _, n := range []Notification{noSeverance, straySeverance, severedBySomeone} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error for the severance", n)
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
		AdminReportKey("r1"):                      "admin_report:r1",
		RelationshipsSeveredKey("s1"):             "relationships_severed:s1",
		AdminSignUpKey("u1"):                      "admin_sign_up:u1",
		ReportResolvedKey("r1"):                   "report_resolved:r1",
		ModerationNoticeKey("m1"):                 "moderation_notice:m1",
		AppealDecidedKey("p1"):                    "appeal_decided:p1",
		AdminAppealKey("p1"):                      "admin_appeal:p1",
		PollClosedKey("p1"):                       "poll_closed:p1",
	} {
		if got != want {
			t.Errorf("key = %q, want %q", got, want)
		}
	}
}

func TestValidateSupporterNotifications(t *testing.T) {
	for _, n := range []Notification{
		{ID: "n", RecipientAccountID: "a", Type: TypeSupporterPaymentFailed, SupporterSubscriptionID: "s", DedupeKey: SupporterTransitionKey(TypeSupporterPaymentFailed, "t")},
		{ID: "n", RecipientAccountID: "a", Type: TypeSupporterSubscriptionEnded, SupporterSubscriptionID: "s", DedupeKey: SupporterTransitionKey(TypeSupporterSubscriptionEnded, "t")},
		{ID: "n", RecipientAccountID: "a", Type: TypeSupporterPlanRetired, SupporterSubscriptionID: "s", DedupeKey: SupporterPlanRetiredKey("p", "s")},
	} {
		if !n.Type.Valid() || !n.Type.Supporter() {
			t.Errorf("%s: Valid %v, Supporter %v", n.Type, n.Type.Valid(), n.Type.Supporter())
		}
		if err := n.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", n, err)
		}
	}
	for _, n := range []Notification{
		{ID: "n", RecipientAccountID: "a", Type: TypeSupporterPaymentFailed, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeSupporterPlanRetired, SupporterSubscriptionID: "s", SourceAccountID: "b", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeFollow, SourceAccountID: "b", SupporterSubscriptionID: "s", DedupeKey: "k"},
	} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error", n)
		}
	}
	if TypeFollow.Supporter() {
		t.Error("follow is a supporter notification")
	}
}

func TestValidateModeratorNotifications(t *testing.T) {
	for _, n := range []Notification{
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminReport, ReportID: "r", DedupeKey: AdminReportKey("r")},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminSignUp, SubjectUserID: "u", DedupeKey: AdminSignUpKey("u")},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminAppeal, AppealID: "p", DedupeKey: AdminAppealKey("p")},
		{ID: "n", RecipientAccountID: "a", Type: TypeAppealDecided, AppealID: "p", DedupeKey: AppealDecidedKey("p")},
	} {
		if err := n.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", n, err)
		}
	}
	for _, n := range []Notification{
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminReport, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminReport, ReportID: "r", SourceAccountID: "b", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminSignUp, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminAppeal, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminAppeal, AppealID: "p", SourceAccountID: "b", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAppealDecided, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminWebhookDisabled, SourceAccountID: "b", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminCustomEmojiRequest, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeAdminCustomEmojiRequest, CustomEmojiRequestID: "r", SourceAccountID: "b", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeCustomEmojiRequestDecided, DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeFollow, SourceAccountID: "b", CustomEmojiRequestID: "r", DedupeKey: "k"},
		{ID: "n", RecipientAccountID: "a", Type: TypeFollow, SourceAccountID: "b", AppealID: "p", DedupeKey: "k"},
	} {
		if err := n.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error", n)
		}
	}
}

func TestHiddenTypes(t *testing.T) {
	for _, tc := range []struct {
		granted []Permission
		want    []Type
	}{
		{nil, []Type{TypeAdminReport, TypeAdminSignUp, TypeAdminAppeal, TypeAdminWebhookDisabled, TypeAdminCustomEmojiRequest}},
		{[]Permission{PermissionManageReports}, []Type{TypeAdminSignUp, TypeAdminAppeal, TypeAdminWebhookDisabled, TypeAdminCustomEmojiRequest}},
		{[]Permission{PermissionManageAccounts}, []Type{TypeAdminReport, TypeAdminWebhookDisabled, TypeAdminCustomEmojiRequest}},
		{[]Permission{PermissionManageWebhooks}, []Type{TypeAdminReport, TypeAdminSignUp, TypeAdminAppeal, TypeAdminCustomEmojiRequest}},
		{[]Permission{PermissionManageCustomEmojis}, []Type{TypeAdminReport, TypeAdminSignUp, TypeAdminAppeal, TypeAdminWebhookDisabled}},
		{[]Permission{PermissionManageAccounts, PermissionManageReports, PermissionManageWebhooks, PermissionManageCustomEmojis, "view_audit"}, nil},
	} {
		if got := HiddenTypes(tc.granted); !slices.Equal(got, tc.want) {
			t.Errorf("HiddenTypes(%v) = %v, want %v", tc.granted, got, tc.want)
		}
	}
	if _, ok := ModeratorPermission(TypeFollow); ok {
		t.Error("ModeratorPermission(follow) needs a permission")
	}
}
