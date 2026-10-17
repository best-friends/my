// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"errors"
	"time"
)

// Type is the kind of a notification (notifications.type, FR-205).
type Type string

// Notification types. The notification module generates follow, follow_request,
// follow_accepted, mention, boost, and favourite from the graph and content events, and
// report_resolved from the moderation events; poll_closed and moderation_notice belong to the
// modules that close polls and act on accounts.
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
)

// Types are every notification type, in a stable order.
var Types = []Type{
	TypeFollow, TypeFollowRequest, TypeFollowAccepted, TypeMention, TypeBoost, TypeFavourite,
	TypePollClosed, TypeModerationNotice, TypeReportResolved,
}

// Valid reports whether t is a known notification type.
func (t Type) Valid() bool {
	switch t {
	case TypeFollow, TypeFollowRequest, TypeFollowAccepted, TypeMention, TypeBoost, TypeFavourite,
		TypePollClosed, TypeModerationNotice, TypeReportResolved:
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
	SourceAccountID    string // "" for moderation_notice and report_resolved, which come from the instance
	StatusID           string // "" unless the notification is about a status
	ModerationActionID string // "" unless the notification is a moderation_notice
	ReportID           string // "" unless the notification is a report_resolved
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
	if (n.Type == TypeReportResolved) != (n.ReportID != "") {
		return errors.New("notification: a report_resolved, and only it, names a report")
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
