// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ActionKind is the kind of a moderation action on an account (moderation_actions.kind).
type ActionKind string

// Kinds of actions on an account (data-model §5.2).
const (
	ActionWarn                  ActionKind = "warn"
	ActionMarkStatusesSensitive ActionKind = "mark_statuses_sensitive"
	ActionDeleteStatuses        ActionKind = "delete_statuses"
	ActionForceSensitive        ActionKind = "force_sensitive"
	ActionLimit                 ActionKind = "limit"
	ActionRestrictPosting       ActionKind = "restrict_posting"
	ActionRestrictInteraction   ActionKind = "restrict_interaction"
	ActionDisableLogin          ActionKind = "disable_login"
	ActionSuspend               ActionKind = "suspend"
	ActionBan                   ActionKind = "ban"
)

// accountRestrictions is the table of data-model §5.2: the restrictions each kind of action
// imposes on its account. Changing it requires rebuilding account_restrictions.
var accountRestrictions = map[ActionKind][]Restriction{
	ActionWarn:                  nil,
	ActionMarkStatusesSensitive: nil,
	ActionDeleteStatuses:        nil,
	ActionForceSensitive:        {RestrictMediaSensitive},
	ActionLimit:                 {RestrictVisibilityCeilingPrivate, RestrictPublicDiscovery, RestrictUnsolicitedReach},
	ActionRestrictPosting:       {RestrictPost},
	ActionRestrictInteraction:   {RestrictInteract},
	ActionDisableLogin:          {RestrictLogin},
	ActionSuspend:               {RestrictLogin, RestrictPost, RestrictInteract, RestrictProfileView, RestrictFederation},
	ActionBan:                   {RestrictLogin, RestrictPost, RestrictInteract, RestrictProfileView, RestrictFederation},
}

// Valid reports whether k is one of the kinds of actions on an account.
func (k ActionKind) Valid() bool {
	_, ok := accountRestrictions[k]
	return ok
}

// Restrictions returns the restrictions an action of kind k imposes (data-model §5.2).
func (k ActionKind) Restrictions() []Restriction { return accountRestrictions[k] }

// Category is the reason category of an action or a report.
type Category string

// Categories.
const (
	CategorySpam      Category = "spam"
	CategoryLegal     Category = "legal"
	CategoryViolation Category = "violation"
	CategoryOther     Category = "other"
)

// Valid reports whether c is one of the categories.
func (c Category) Valid() bool {
	switch c {
	case CategorySpam, CategoryLegal, CategoryViolation, CategoryOther:
		return true
	}
	return false
}

// Issuer is who issued an action on an account.
type Issuer string

// Issuers.
const (
	IssuerModerator Issuer = "moderator"
	IssuerSystem    Issuer = "system"
	IssuerRemote    Issuer = "remote"
)

// Valid reports whether i is one of the issuers.
func (i Issuer) Valid() bool {
	switch i {
	case IssuerModerator, IssuerSystem, IssuerRemote:
		return true
	}
	return false
}

// ErrInvalidAction reports an action with a missing or invalid field.
var ErrInvalidAction = errors.New("moderation: invalid action")

// Action is one moderation action on an account (moderation_actions, an event). It is
// immutable; lifting it is a separate Lift event, and an expired action is not lifted: its
// restrictions end at ExpiresAt when they are read.
type Action struct {
	ID              string
	TargetAccountID string
	Kind            ActionKind
	ReasonCategory  Category
	PublicReason    string // shown to the target; "" for none
	PrivateNote     string // "" for none
	ReportID        string // "" when the action does not follow from a report
	Issuer          Issuer
	IssuedByUserID  string // "" for the system and the CLI
	NotifyTarget    bool
	ExpiresAt       *time.Time // nil: until lifted
	IssuedAt        time.Time
}

// Validate checks the invariants of a new action: a known kind, category, and issuer, an end
// after its start, and no end for a ban.
func (a Action) Validate() error {
	switch {
	case a.ID == "" || a.TargetAccountID == "" || a.IssuedAt.IsZero():
		return fmt.Errorf("%w: a field is empty", ErrInvalidAction)
	case !a.Kind.Valid():
		return fmt.Errorf("%w: kind %q", ErrInvalidAction, a.Kind)
	case !a.ReasonCategory.Valid():
		return fmt.Errorf("%w: category %q", ErrInvalidAction, a.ReasonCategory)
	case !a.Issuer.Valid():
		return fmt.Errorf("%w: issuer %q", ErrInvalidAction, a.Issuer)
	case a.Kind == ActionBan && a.ExpiresAt != nil:
		return fmt.Errorf("%w: a ban never expires", ErrInvalidAction)
	case a.ExpiresAt != nil && !a.ExpiresAt.After(a.IssuedAt):
		return fmt.Errorf("%w: it must expire after it is issued", ErrInvalidAction)
	}
	return nil
}

// Rows returns the account_restrictions rows that a imposes.
func (a Action) Rows() []RestrictionRow {
	rs := a.Kind.Restrictions()
	out := make([]RestrictionRow, len(rs))
	for i, r := range rs {
		out[i] = RestrictionRow{Target: a.TargetAccountID, Restriction: r, SourceActionID: a.ID, EffectiveUntil: a.ExpiresAt}
	}
	return out
}

// Lift is the event that an action on an account was lifted (moderation_action_lifts).
type Lift struct {
	ActionID       string
	LiftedByUserID string // "" for the system
	Reason         string // "" for none
	LiftedAt       time.Time
}

// DomainActionKind is the kind of a moderation action on a domain
// (domain_moderation_actions.kind).
type DomainActionKind string

// Kinds of actions on a domain.
const (
	DomainLimit          DomainActionKind = "limit"
	DomainSuspend        DomainActionKind = "suspend"
	DomainRejectMedia    DomainActionKind = "reject_media"
	DomainRejectReports  DomainActionKind = "reject_reports"
	DomainForceSensitive DomainActionKind = "force_sensitive"
)

// domainRestrictions maps each kind of domain action to the restrictions it imposes
// (data-model §5.2).
var domainRestrictions = map[DomainActionKind][]Restriction{
	DomainLimit:          {RestrictPublicDiscovery, RestrictUnsolicitedReach},
	DomainSuspend:        {RestrictProfileView, RestrictFederation},
	DomainRejectMedia:    {RestrictMediaFetch},
	DomainRejectReports:  {RestrictReportIntake},
	DomainForceSensitive: {RestrictMediaSensitive},
}

// Valid reports whether k is one of the kinds of actions on a domain.
func (k DomainActionKind) Valid() bool {
	_, ok := domainRestrictions[k]
	return ok
}

// Restrictions returns the restrictions an action of kind k imposes on its domain.
func (k DomainActionKind) Restrictions() []Restriction { return domainRestrictions[k] }

// DomainAction is one moderation action on a domain and its subdomains
// (domain_moderation_actions, an event). Like Action, it is immutable and ends by a DomainLift
// or by expiring.
type DomainAction struct {
	ID string
	// Domain is a canonical host: lower-case IDNA A-labels without a trailing dot or a port.
	Domain        string
	Kind          DomainActionKind
	PublicComment string // "" for none
	PrivateNote   string // "" for none
	// Obfuscate hides part of the domain where the instance lists its domain actions publicly.
	Obfuscate      bool
	IssuedByUserID string     // "" for the CLI
	ExpiresAt      *time.Time // nil: until lifted
	IssuedAt       time.Time
}

// Validate checks the invariants of a new domain action.
func (a DomainAction) Validate() error {
	switch {
	case a.ID == "" || a.IssuedAt.IsZero():
		return fmt.Errorf("%w: a field is empty", ErrInvalidAction)
	case !ValidDomain(a.Domain):
		return fmt.Errorf("%w: domain %q", ErrInvalidAction, a.Domain)
	case !a.Kind.Valid():
		return fmt.Errorf("%w: kind %q", ErrInvalidAction, a.Kind)
	case a.ExpiresAt != nil && !a.ExpiresAt.After(a.IssuedAt):
		return fmt.Errorf("%w: it must expire after it is issued", ErrInvalidAction)
	}
	return nil
}

// Rows returns the domain_restrictions rows that a imposes.
func (a DomainAction) Rows() []RestrictionRow {
	rs := a.Kind.Restrictions()
	out := make([]RestrictionRow, len(rs))
	for i, r := range rs {
		out[i] = RestrictionRow{Target: a.Domain, Restriction: r, SourceActionID: a.ID, EffectiveUntil: a.ExpiresAt}
	}
	return out
}

// DomainLift is the event that a domain action was lifted (domain_moderation_action_lifts).
type DomainLift struct {
	ActionID       string
	LiftedByUserID string // "" for the CLI
	LiftedAt       time.Time
}

// ValidDomain reports whether d looks like a canonical host: non-empty, lower case, without a
// scheme, path, port, or empty label. Callers canonicalize hosts (IDNA A-labels) before.
func ValidDomain(d string) bool {
	if d == "" || len(d) > 253 || d != strings.ToLower(d) || strings.ContainsAny(d, "/:@ \t\n") {
		return false
	}
	for label := range strings.SplitSeq(d, ".") {
		if label == "" {
			return false
		}
	}
	return true
}
