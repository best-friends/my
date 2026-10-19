// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"errors"
	"slices"
	"time"
)

// Type is the kind of a notification (notifications.type, FR-205).
type Type string

// Notification types. The notification module generates follow, follow_request,
// follow_accepted, mention, boost, and favourite from the graph and content events, and
// moderation_notice, appeal_decided, report_resolved, and relationships_severed from the
// moderation events; poll_closed belongs to the module that closes polls. admin_report,
// admin_sign_up, and admin_appeal go to the moderators whose roles grant the permission they
// need (ModeratorPermission).
const (
	TypeFollow           Type = "follow"
	TypeFollowRequest    Type = "follow_request"
	TypeFollowAccepted   Type = "follow_accepted"
	TypeMention          Type = "mention"
	TypeBoost            Type = "boost"
	TypeFavourite        Type = "favourite"
	TypePollClosed       Type = "poll_closed"
	TypeModerationNotice Type = "moderation_notice"
	TypeReportResolved   Type = "report_resolved"
	TypeAdminReport      Type = "admin_report"
	TypeAdminSignUp      Type = "admin_sign_up"
	// TypeRelationshipsSevered tells a local account of the follows that a suspension of a
	// domain or a remote account cut (FR-717).
	TypeRelationshipsSevered Type = "relationships_severed"
	TypeAppealDecided        Type = "appeal_decided"
	TypeAdminAppeal          Type = "admin_appeal"
)

// Types are every notification type, in a stable order.
var Types = []Type{
	TypeFollow, TypeFollowRequest, TypeFollowAccepted, TypeMention, TypeBoost, TypeFavourite,
	TypePollClosed, TypeModerationNotice, TypeReportResolved, TypeAdminReport, TypeAdminSignUp,
	TypeRelationshipsSevered,
	TypeAppealDecided, TypeAdminAppeal,
}

// Valid reports whether t is a known notification type.
func (t Type) Valid() bool {
	switch t {
	case TypeFollow, TypeFollowRequest, TypeFollowAccepted, TypeMention, TypeBoost, TypeFavourite,
		TypePollClosed, TypeModerationNotice, TypeReportResolved, TypeAdminReport, TypeAdminSignUp,
		TypeRelationshipsSevered, TypeAppealDecided, TypeAdminAppeal:
		return true
	}
	return false
}

// Notification is one notification to a local account: an event that happened to it
// (notifications, data-model §4.7). It never changes; dismissing it is a separate Dismissal.
//
// DedupeKey is unique per recipient, so that processing the same domain event again, or an
// action that the source repeats, inserts nothing: see the *Key functions.
type Notification struct {
	ID                 string
	RecipientAccountID string
	Type               Type
	SourceAccountID    string // "" for moderation_notice, report_resolved, relationships_severed, and the admin types, which come from the instance
	StatusID           string // "" unless the notification is about a status
	ModerationActionID string // "" unless the notification is a moderation_notice
	ReportID           string // "" unless the notification is a report_resolved or an admin_report
	SubjectUserID      string // "" unless the notification is an admin_sign_up: the user who signed up
	SeveranceID        string // "" unless the notification is a relationships_severed
	AppealID           string // "" unless the notification is an appeal_decided or an admin_appeal
	DedupeKey          string
	CreatedAt          time.Time
}

var (
	// ErrSelfNotification reports a notification whose source is its recipient: nobody is
	// notified of their own actions.
	ErrSelfNotification = errors.New("notification: an account is not notified of its own actions")
	// ErrInvalidType reports an unknown notification type.
	ErrInvalidType = errors.New("notification: unknown notification type")
)

// Validate checks the invariants of a new notification.
func (n Notification) Validate() error {
	if n.ID == "" || n.RecipientAccountID == "" || n.DedupeKey == "" {
		return errors.New("notification: a notification needs an ID, a recipient, and a dedupe key")
	}
	if !n.Type.Valid() {
		return ErrInvalidType
	}
	if n.SourceAccountID == n.RecipientAccountID {
		return ErrSelfNotification
	}
	if (n.Type == TypeReportResolved || n.Type == TypeAdminReport) != (n.ReportID != "") {
		return errors.New("notification: a report_resolved or an admin_report, and only they, name a report")
	}
	if (n.Type == TypeModerationNotice) != (n.ModerationActionID != "") {
		return errors.New("notification: a moderation_notice, and only it, names a moderation action")
	}
	if (n.Type == TypeRelationshipsSevered) != (n.SeveranceID != "") || (n.Type == TypeRelationshipsSevered && n.SourceAccountID != "") {
		return errors.New("notification: a relationships_severed, and only it, names a severance, and it has no source account")
	}
	if (n.Type == TypeAppealDecided || n.Type == TypeAdminAppeal) != (n.AppealID != "") {
		return errors.New("notification: an appeal_decided or an admin_appeal, and only they, name an appeal")
	}
	switch {
	case n.Type == TypeAdminReport && (n.ReportID == "" || n.SourceAccountID != ""),
		n.Type == TypeAdminAppeal && n.SourceAccountID != "",
		n.Type == TypeAdminSignUp && (n.SubjectUserID == "" || n.SourceAccountID != ""):
		return errors.New("notification: a moderator notification needs its subject and has no source account")
	}
	return nil
}

// FollowKey is the dedupe key of a follow, follow_request, or follow_accepted notification:
// one per follow and type. Following again after unfollowing is a new follow and notifies again.
func FollowKey(t Type, followID string) string { return string(t) + ":" + followID }

// MentionKey is the dedupe key of a mention notification: one per status, so that an edit
// notifies only the accounts it newly mentions.
func MentionKey(statusID string) string { return string(TypeMention) + ":" + statusID }

// InteractionKey is the dedupe key of a boost or favourite notification: one per status and
// source account, so that undoing and repeating the interaction does not notify again.
func InteractionKey(t Type, statusID, sourceAccountID string) string {
	return string(t) + ":" + statusID + ":" + sourceAccountID
}

// ReportResolvedKey is the dedupe key of a report_resolved notification: one per report, so
// that resolving a reopened report again does not notify the reporter again.
func ReportResolvedKey(reportID string) string { return string(TypeReportResolved) + ":" + reportID }

// ModerationNoticeKey is the dedupe key of a moderation_notice notification: one per action.
func ModerationNoticeKey(actionID string) string {
	return string(TypeModerationNotice) + ":" + actionID
}

// RelationshipsSeveredKey is the dedupe key of a relationships_severed notification: one per
// severance, so that processing the same suspension again does not notify again.
func RelationshipsSeveredKey(severanceID string) string {
	return string(TypeRelationshipsSevered) + ":" + severanceID
}

// Permission is a permission of the roles of the identity module (role_permissions.permission)
// that a moderator notification needs.
type Permission string

// Permissions of the moderator notifications.
const (
	PermissionManageReports  Permission = "manage_reports"
	PermissionManageAccounts Permission = "manage_accounts"
)

// ModeratorPermission returns the permission that the recipient of a notification of type t
// needs, when they get it and while they read it, and whether t needs one: admin_report needs
// manage_reports, and admin_sign_up and admin_appeal manage_accounts. The other types need none.
func ModeratorPermission(t Type) (Permission, bool) {
	switch t {
	case TypeAdminReport:
		return PermissionManageReports, true
	case TypeAdminSignUp, TypeAdminAppeal:
		return PermissionManageAccounts, true
	}
	return "", false
}

// HiddenTypes returns the types that need a permission that granted does not hold: the
// notifications of a recipient with the permissions granted that the list leaves out.
func HiddenTypes(granted []Permission) []Type {
	var out []Type
	for _, t := range Types {
		p, ok := ModeratorPermission(t)
		if !ok {
			continue
		}
		if !slices.Contains(granted, p) {
			out = append(out, t)
		}
	}
	return out
}

// AdminReportKey is the dedupe key of an admin_report notification: one per report.
func AdminReportKey(reportID string) string { return string(TypeAdminReport) + ":" + reportID }

// AdminSignUpKey is the dedupe key of an admin_sign_up notification: one per user who signed up.
func AdminSignUpKey(userID string) string { return string(TypeAdminSignUp) + ":" + userID }

// AppealDecidedKey is the dedupe key of an appeal_decided notification: one per appeal.
func AppealDecidedKey(appealID string) string { return string(TypeAppealDecided) + ":" + appealID }

// AdminAppealKey is the dedupe key of an admin_appeal notification: one per appeal.
func AdminAppealKey(appealID string) string { return string(TypeAdminAppeal) + ":" + appealID }

// Dismissal is the event that the recipient dismissed a notification (notification_dismissals).
// Dismissed notifications leave the list and the unread count.
type Dismissal struct {
	NotificationID string
	DismissedAt    time.Time
}

// ReadMarker is the newest notification an account has read (notification_read_markers, a
// current value). Notifications with a greater ID are unread.
type ReadMarker struct {
	AccountID  string
	LastReadID string
}

// ExclusionReason is why an exclusion hides a target's notifications from a viewer.
type ExclusionReason string

// Exclusion reasons.
const (
	// ExcludedBlock: the viewer blocks the target.
	ExcludedBlock ExclusionReason = "block"
	// ExcludedBlockedBy: the target blocks the viewer.
	ExcludedBlockedBy ExclusionReason = "blocked_by"
	// ExcludedMute: the viewer mutes the target with hide_notifications.
	ExcludedMute ExclusionReason = "mute"
)

// Exclusion hides the notifications whose source is TargetAccountID from ViewerAccountID while
// it is in force: until ExpiresAt, or for good when ExpiresAt is nil (notification_exclusions,
// a projection of the graph module's blocks and mutes). Reading the list and the unread count
// applies the exclusions in force at that time, so a block or mute also hides the notifications
// that came before it, and lifting it shows them again.
type Exclusion struct {
	ViewerAccountID string
	TargetAccountID string
	Reason          ExclusionReason
	ExpiresAt       *time.Time
}
