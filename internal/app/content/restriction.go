// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"

	"github.com/best-friends/my/internal/domain/content"
)

// Restrictions are the current moderation restrictions of an account that bear on its statuses
// (data-model §5.2).
type Restrictions struct {
	// Post forbids posting, replying, and editing (the post restriction).
	Post bool
	// Author are the restrictions that bear on who sees the account's statuses, the visibility
	// ceiling among them.
	Author content.AuthorRestrictions
}

// CanPost reports whether the account may post, reply, and edit.
func (r Restrictions) CanPost() bool { return !r.Post }

// VisibilityCeiling returns the widest visibility the account may post with (data-model §5.4).
func (r Restrictions) VisibilityCeiling() content.Visibility { return r.Author.VisibilityCeiling() }

// PostingLimits are what an account's current restrictions let it post (data-model §5.4).
type PostingLimits struct {
	// VisibilityCeiling is the widest visibility a new status gets.
	VisibilityCeiling content.Visibility
	// Visibilities are the visibilities a new status keeps as asked for, widest first.
	Visibilities []content.Visibility
}

// RestrictionPolicy is the port through which the content module reads an account's effective
// restrictions (data-model §5.3). Use cases decide posting rights and the visibility ceiling
// only through it, never from the moderation tables.
type RestrictionPolicy interface {
	// Restrictions returns the restrictions of an account that are in effect now.
	Restrictions(ctx context.Context, accountID string) (Restrictions, error)
}

// NoRestrictions is the RestrictionPolicy of an instance whose accounts carry no moderation
// restrictions: it restricts nobody. It stands in for the policy that the moderation module
// derives from account_restrictions and domain_restrictions.
type NoRestrictions struct{}

// Restrictions implements RestrictionPolicy.
func (NoRestrictions) Restrictions(context.Context, string) (Restrictions, error) {
	return Restrictions{}, nil
}
