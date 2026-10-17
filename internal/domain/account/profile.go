// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"errors"
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"
)

// ProfileRevision is one version of an account's profile. Seq starts at 1 and grows by one
// per revision of the same account; the revision with the highest Seq is current.
type ProfileRevision struct {
	ID            string
	AccountID     string
	Seq           int
	DisplayName   string
	NoteHTML      string
	Locked        bool
	Bot           bool
	Discoverable  bool
	Indexable     bool
	AvatarMediaID string // "" when there is no avatar
	HeaderMediaID string // "" when there is no header image
	RevisedAt     time.Time
	Fields        []ProfileField
	// Source is what the owner typed, nil for a revision without one (remote profiles).
	Source *ProfileSource
	// Remote is where a remote revision came from, nil for a local one.
	Remote *RemoteProfileSource
}

// SameContent reports whether r and o present the same profile: everything but their
// identity, Seq, time, and sources.
func (r ProfileRevision) SameContent(o ProfileRevision) bool {
	if r.DisplayName != o.DisplayName || r.NoteHTML != o.NoteHTML || r.Locked != o.Locked || r.Bot != o.Bot ||
		r.Discoverable != o.Discoverable || r.Indexable != o.Indexable ||
		r.AvatarMediaID != o.AvatarMediaID || r.HeaderMediaID != o.HeaderMediaID || len(r.Fields) != len(o.Fields) {
		return false
	}
	for i, f := range r.Fields {
		g := o.Fields[i]
		if f.Position != g.Position || f.Name != g.Name || f.ValueHTML != g.ValueHTML {
			return false
		}
	}
	return true
}

// ProfileField is one metadata field of a profile revision (FR-006), ordered by Position.
type ProfileField struct {
	Position  int
	Name      string
	ValueHTML string
}

// ProfileSource is what the owner of a local profile typed, from which the server rendered
// NoteHTML and the ValueHTML of the fields (account_profile_revision_sources and
// account_profile_field_sources).
type ProfileSource struct {
	Note string
	// FieldValues are the typed values of the revision's Fields, by position.
	FieldValues []string
}

// ProfileHead points at the current revision of an account's profile
// (the account_profile_heads projection).
type ProfileHead struct {
	AccountID  string
	RevisionID string
	Seq        int
}

// Supersedes reports whether revision r should replace the current head h,
// which is the zero value when the account has no head yet.
func (r ProfileRevision) Supersedes(h ProfileHead) bool {
	return h.RevisionID == "" || r.Seq > h.Seq
}

// Limits of a local profile, in characters (the same limits as Mastodon).
const (
	MaxDisplayNameLength = 30
	MaxNoteLength        = 500
	MaxProfileFields     = 4
	MaxFieldNameLength   = 255
	MaxFieldValueLength  = 255
)

// ErrInvalidProfile reports profile content that a local profile may not have; errors that
// wrap it are *ProfileError.
var ErrInvalidProfile = errors.New("invalid profile")

// ProfileProperty names the part of a profile that a ProfileError is about.
type ProfileProperty string

// Profile properties.
const (
	PropertyDisplayName ProfileProperty = "display_name"
	PropertyNote        ProfileProperty = "note"
	PropertyFields      ProfileProperty = "fields"
	PropertyFieldName   ProfileProperty = "field_name"
	PropertyFieldValue  ProfileProperty = "field_value"
	PropertyAvatar      ProfileProperty = "avatar"
	PropertyHeader      ProfileProperty = "header"
)

// ProfileProblem says what is wrong with a profile property.
type ProfileProblem string

// Profile problems.
const (
	// ProblemTooLong is text longer than Max characters.
	ProblemTooLong ProfileProblem = "too_long"
	// ProblemTooMany is more than Max fields.
	ProblemTooMany ProfileProblem = "too_many"
	// ProblemInvalid is text that is not valid UTF-8 or contains a control character that the
	// property does not allow, or an image that the account cannot use.
	ProblemInvalid ProfileProblem = "invalid"
)

// ProfileError is an invalid property of a new local profile revision. It wraps
// ErrInvalidProfile.
type ProfileError struct {
	Property ProfileProperty
	Problem  ProfileProblem
	// Max is the limit of ProblemTooLong and ProblemTooMany.
	Max int
}

func (e *ProfileError) Error() string {
	if e.Max > 0 {
		return fmt.Sprintf("%s: %s is %s (at most %d)", ErrInvalidProfile, e.Property, e.Problem, e.Max)
	}
	return fmt.Sprintf("%s: %s is %s", ErrInvalidProfile, e.Property, e.Problem)
}

// Unwrap returns ErrInvalidProfile.
func (e *ProfileError) Unwrap() error { return ErrInvalidProfile }

// ProfileText is the text of a local profile as its owner typed it.
type ProfileText struct {
	DisplayName string
	Note        string
	Fields      []FieldSource
}

// FieldSource is a metadata field as its owner typed it.
type FieldSource struct {
	Name  string
	Value string
}

// ValidateProfileText checks the typed text of a local profile against the limits above. The
// display name and field names are single lines; the note and field values may have line
// breaks and tabs. No text may contain other control characters.
func ValidateProfileText(p ProfileText) error {
	if err := checkProfileText(PropertyDisplayName, p.DisplayName, MaxDisplayNameLength, false); err != nil {
		return err
	}
	if err := checkProfileText(PropertyNote, p.Note, MaxNoteLength, true); err != nil {
		return err
	}
	if len(p.Fields) > MaxProfileFields {
		return &ProfileError{Property: PropertyFields, Problem: ProblemTooMany, Max: MaxProfileFields}
	}
	for _, f := range p.Fields {
		if err := checkProfileText(PropertyFieldName, f.Name, MaxFieldNameLength, false); err != nil {
			return err
		}
		if err := checkProfileText(PropertyFieldValue, f.Value, MaxFieldValueLength, true); err != nil {
			return err
		}
	}
	return nil
}

func checkProfileText(prop ProfileProperty, s string, maxLen int, multiline bool) error {
	if !utf8.ValidString(s) {
		return &ProfileError{Property: prop, Problem: ProblemInvalid}
	}
	for _, r := range s {
		if unicode.IsControl(r) && (!multiline || (r != '\n' && r != '\r' && r != '\t')) {
			return &ProfileError{Property: prop, Problem: ProblemInvalid}
		}
	}
	if utf8.RuneCountInString(s) > maxLen {
		return &ProfileError{Property: prop, Problem: ProblemTooLong, Max: maxLen}
	}
	return nil
}
