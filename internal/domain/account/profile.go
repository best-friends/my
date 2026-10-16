// SPDX-License-Identifier: AGPL-3.0-only

package account

import "time"

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
}

// ProfileField is one metadata field of a profile revision (FR-006), ordered by Position.
type ProfileField struct {
	Position  int
	Name      string
	ValueHTML string
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
