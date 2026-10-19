// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"
)

// The audit log (FR-704, ADR-0037): one append-only entry per operation of a moderator or an
// administrator, written in the transaction of the operation. An entry names who did what to
// which target and copies the target's display name, so that it stays readable after the
// target is erased; reasons and notes stay in the source event.

// AuditActorKind is who performed an audited operation (audit_log_entries.actor_kind).
type AuditActorKind string

// Kinds of actors.
const (
	// AuditActorUser is a moderator or an administrator signed in; the entry names the user.
	AuditActorUser AuditActorKind = "user"
	// AuditActorCLI is the command line, which acts as no user.
	AuditActorCLI AuditActorKind = "cli"
	// AuditActorSystem is the server itself, such as a scheduled job.
	AuditActorSystem AuditActorKind = "system"
	// AuditActorRemote is another server.
	AuditActorRemote AuditActorKind = "remote"
)

// Valid reports whether k is one of the kinds of actors.
func (k AuditActorKind) Valid() bool {
	switch k {
	case AuditActorUser, AuditActorCLI, AuditActorSystem, AuditActorRemote:
		return true
	}
	return false
}

// AuditTargetKind is the kind of what an audited operation acted on
// (audit_log_entries.target_kind).
type AuditTargetKind string

// Kinds of targets.
const (
	AuditTargetAccount     AuditTargetKind = "account"
	AuditTargetStatus      AuditTargetKind = "status"
	AuditTargetDomain      AuditTargetKind = "domain"
	AuditTargetReport      AuditTargetKind = "report"
	AuditTargetAppeal      AuditTargetKind = "appeal"
	AuditTargetUser        AuditTargetKind = "user"
	AuditTargetRole        AuditTargetKind = "role"
	AuditTargetSetting     AuditTargetKind = "setting"
	AuditTargetRule        AuditTargetKind = "rule"
	AuditTargetSignupBlock AuditTargetKind = "signup_block"
)

var auditTargetKinds = []AuditTargetKind{
	AuditTargetAccount, AuditTargetStatus, AuditTargetDomain, AuditTargetReport, AuditTargetAppeal,
	AuditTargetUser, AuditTargetRole, AuditTargetSetting, AuditTargetRule, AuditTargetSignupBlock,
}

// AuditTargetKinds returns every kind of target, in the order of the schema's CHECK.
func AuditTargetKinds() []AuditTargetKind { return slices.Clone(auditTargetKinds) }

// Valid reports whether k is one of the kinds of targets.
func (k AuditTargetKind) Valid() bool { return slices.Contains(auditTargetKinds, k) }

// AuditAction is the kind of an audited operation (audit_log_entries.action). Each action has
// one kind of target (AuditAction.Target).
type AuditAction string

// Audited operations. A new administrative operation adds its action here, to the schema's
// CHECK, and to the API's enum, and emits it from its use case.
const (
	// AuditAccountActionIssued: an action on an account (moderation_actions).
	AuditAccountActionIssued AuditAction = "account_action_issued"
	// AuditAccountActionLifted: the lift of an action on an account (moderation_action_lifts).
	AuditAccountActionLifted AuditAction = "account_action_lifted"
	// AuditDomainActionIssued: an action on a domain (domain_moderation_actions).
	AuditDomainActionIssued AuditAction = "domain_action_issued"
	// AuditDomainActionLifted: the lift of an action on a domain.
	AuditDomainActionLifted AuditAction = "domain_action_lifted"
	// AuditReportAssigned: a moderator took a report, or was given it (report_assignments).
	AuditReportAssigned AuditAction = "report_assigned"
	// AuditReportUnassigned: a report was left to nobody (report_assignments).
	AuditReportUnassigned AuditAction = "report_unassigned"
	// AuditReportResolved: a report was resolved (report_resolutions).
	AuditReportResolved AuditAction = "report_resolved"
	// AuditReportReopened: a resolved report was reopened (report_reopenings).
	AuditReportReopened AuditAction = "report_reopened"
	// AuditReportNoteAdded: a moderator wrote an internal note on a report (report_notes).
	AuditReportNoteAdded AuditAction = "report_note_added"
	// AuditRegistrationApproved: a pending registration was approved (registration_decisions).
	AuditRegistrationApproved AuditAction = "registration_approved"
	// AuditRegistrationRejected: a pending registration was rejected (registration_decisions).
	AuditRegistrationRejected AuditAction = "registration_rejected"
	// AuditSettingChanged: an instance setting got a new value (instance_setting_revisions).
	AuditSettingChanged AuditAction = "setting_changed"
	// AuditTextChanged: an instance text got a new text in one locale (instance_text_revisions).
	AuditTextChanged AuditAction = "text_changed"
	// AuditRuleAdded: a new instance rule (instance_rule_revisions).
	AuditRuleAdded AuditAction = "rule_added"
	// AuditRuleRevised: new texts of a rule (instance_rule_revisions).
	AuditRuleRevised AuditAction = "rule_revised"
	// AuditRuleMoved: a rule moved among the rules in effect (instance_rule_revisions).
	AuditRuleMoved AuditAction = "rule_moved"
	// AuditRuleRetired: a rule was retired (instance_rule_retirements).
	AuditRuleRetired AuditAction = "rule_retired"
	// AuditRoleGranted: a role was granted to a user (user_role_grants).
	AuditRoleGranted AuditAction = "role_granted"
	// AuditRoleRevoked: a role was revoked from a user (user_role_revocations).
	AuditRoleRevoked AuditAction = "role_revoked"
	// AuditAppealUpheld: an appeal was decided and its action kept (moderation_appeal_decisions).
	AuditAppealUpheld AuditAction = "appeal_upheld"
	// AuditAppealOverturned: an appeal was decided and its action lifted
	// (moderation_appeal_decisions); the lift has its own account_action_lifted entry.
	AuditAppealOverturned AuditAction = "appeal_overturned"
	// AuditAccountDeletionCancelled: an overturned appeal of a ban cancelled the scheduled
	// deletion of the account (account_lifecycle_events, deletion_cancelled).
	AuditAccountDeletionCancelled AuditAction = "account_deletion_cancelled"
	// AuditSignupBlockAdded: a new sign-up block (signup_blocks).
	AuditSignupBlockAdded AuditAction = "signup_block_added"
	// AuditSignupBlockLifted: a sign-up block was lifted (signup_block_lifts).
	AuditSignupBlockLifted AuditAction = "signup_block_lifted"
)

// auditActions maps each action to the kind of its target, in the order of the schema's CHECK.
var auditActions = []struct {
	action AuditAction
	target AuditTargetKind
}{
	{AuditAccountActionIssued, AuditTargetAccount},
	{AuditAccountActionLifted, AuditTargetAccount},
	{AuditDomainActionIssued, AuditTargetDomain},
	{AuditDomainActionLifted, AuditTargetDomain},
	{AuditReportAssigned, AuditTargetReport},
	{AuditReportUnassigned, AuditTargetReport},
	{AuditReportResolved, AuditTargetReport},
	{AuditReportReopened, AuditTargetReport},
	{AuditReportNoteAdded, AuditTargetReport},
	{AuditRegistrationApproved, AuditTargetUser},
	{AuditRegistrationRejected, AuditTargetUser},
	{AuditSettingChanged, AuditTargetSetting},
	{AuditTextChanged, AuditTargetSetting},
	{AuditRuleAdded, AuditTargetRule},
	{AuditRuleRevised, AuditTargetRule},
	{AuditRuleMoved, AuditTargetRule},
	{AuditRuleRetired, AuditTargetRule},
	{AuditRoleGranted, AuditTargetRole},
	{AuditRoleRevoked, AuditTargetRole},
	{AuditAppealUpheld, AuditTargetAppeal},
	{AuditAppealOverturned, AuditTargetAppeal},
	{AuditAccountDeletionCancelled, AuditTargetAccount},
	{AuditSignupBlockAdded, AuditTargetSignupBlock},
	{AuditSignupBlockLifted, AuditTargetSignupBlock},
}

// AuditActions returns every audited operation, in the order of the schema's CHECK.
func AuditActions() []AuditAction {
	out := make([]AuditAction, len(auditActions))
	for i, a := range auditActions {
		out[i] = a.action
	}
	return out
}

// Target returns the kind of target of the action, and false for an unknown action.
func (a AuditAction) Target() (AuditTargetKind, bool) {
	for _, x := range auditActions {
		if x.action == a {
			return x.target, true
		}
	}
	return "", false
}

// Valid reports whether a is one of the audited operations.
func (a AuditAction) Valid() bool {
	_, ok := a.Target()
	return ok
}

// MaxAuditTargetLabel is the number of characters of an entry's target label that are kept.
const MaxAuditTargetLabel = 512

// ErrInvalidAuditEntry reports an audit log entry with a missing or invalid field.
var ErrInvalidAuditEntry = errors.New("moderation: invalid audit log entry")

// AuditEntry is one entry of the audit log (audit_log_entries, an event). It is never updated;
// only the retention cleanup deletes it (ADR-0038).
type AuditEntry struct {
	ID        string
	ActorKind AuditActorKind
	// ActorUserID is the user who performed the operation when ActorKind is AuditActorUser, and
	// "" otherwise.
	ActorUserID string
	Action      AuditAction
	TargetKind  AuditTargetKind
	TargetID    string
	// TargetLabel is the target's display name when the operation happened: a handle such as
	// @alice@example.com, a domain, or a setting's key.
	TargetLabel string
	// SourceEventID is the ID of the row of the source event (the action, the lift, the
	// revision); "" when the operation has no row of its own.
	SourceEventID string
	OccurredAt    time.Time
}

// AuditActor returns the actor of an operation that a moderator performs, given the user ID
// that the use cases take: the user, or the CLI for "".
func AuditActor(userID string) (AuditActorKind, string) {
	if userID == "" {
		return AuditActorCLI, ""
	}
	return AuditActorUser, userID
}

// TruncateAuditLabel returns the first MaxAuditTargetLabel characters of label.
func TruncateAuditLabel(label string) string {
	if utf8.RuneCountInString(label) <= MaxAuditTargetLabel {
		return label
	}
	return string([]rune(label)[:MaxAuditTargetLabel])
}

// Validate checks the invariants of a new entry: a known actor, action, and target of the
// action's kind, a user exactly for a user actor, and texts within their limits.
func (e AuditEntry) Validate() error {
	target, known := e.Action.Target()
	switch {
	case e.ID == "" || e.TargetID == "" || e.OccurredAt.IsZero():
		return fmt.Errorf("%w: a field is empty", ErrInvalidAuditEntry)
	case !e.ActorKind.Valid():
		return fmt.Errorf("%w: actor kind %q", ErrInvalidAuditEntry, e.ActorKind)
	case (e.ActorKind == AuditActorUser) != (e.ActorUserID != ""):
		return fmt.Errorf("%w: a user actor needs a user, and only a user actor has one", ErrInvalidAuditEntry)
	case !known:
		return fmt.Errorf("%w: action %q", ErrInvalidAuditEntry, e.Action)
	case e.TargetKind != target:
		return fmt.Errorf("%w: %s acts on a %s, not a %s", ErrInvalidAuditEntry, e.Action, target, e.TargetKind)
	case utf8.RuneCountInString(e.TargetLabel) > MaxAuditTargetLabel:
		return fmt.Errorf("%w: the target label is longer than %d characters", ErrInvalidAuditEntry, MaxAuditTargetLabel)
	}
	for _, s := range []string{e.TargetID, e.TargetLabel, e.SourceEventID, e.ActorUserID} {
		if !validText(s) {
			return fmt.Errorf("%w: a text is not valid UTF-8 or contains NUL", ErrInvalidAuditEntry)
		}
	}
	return nil
}
