// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/domain/content"
)

// DomainMutes tells whether a local account mutes the domain of a remote account (FR-304);
// *appgraph.Service implements it.
type DomainMutes interface {
	MutesDomain(ctx context.Context, accountID, domain string) (bool, error)
}

// QuoteViewState is the state of a quote as a viewer sees it (ADR-0069 §4, §5): the approval of
// the quote, or why the quoted status is not shown to the viewer. The values are those of
// Mastodon's Quote.state, and legacy.
type QuoteViewState string

// Quote view states.
const (
	// QuoteViewPending is a quote that awaits its approval, or whose quoted status is not
	// fetched yet.
	QuoteViewPending  QuoteViewState = "pending"
	QuoteViewAccepted QuoteViewState = "accepted"
	QuoteViewRejected QuoteViewState = "rejected"
	QuoteViewRevoked  QuoteViewState = "revoked"
	// QuoteViewLegacy is a legacy quote, which needs no approval.
	QuoteViewLegacy QuoteViewState = "legacy"
	// QuoteViewDeleted is a quote of a deleted status.
	QuoteViewDeleted QuoteViewState = "deleted"
	// QuoteViewUnauthorized is a quote of a status that the viewer may not see.
	QuoteViewUnauthorized QuoteViewState = "unauthorized"
	// QuoteViewBlockedAccount is a quote of a status of an account that blocks the viewer or
	// that the viewer blocks; the status is not shown.
	QuoteViewBlockedAccount QuoteViewState = "blocked_account"
	// QuoteViewBlockedDomain is a quote of a status of a domain that the viewer mutes; the status
	// is shown behind the reason.
	QuoteViewBlockedDomain QuoteViewState = "blocked_domain"
	// QuoteViewMutedAccount is a quote of a status of an account that the viewer mutes; the
	// status is shown behind the reason.
	QuoteViewMutedAccount QuoteViewState = "muted_account"
)

// QuoteView is the quote of a status as a viewer sees it.
type QuoteView struct {
	State QuoteViewState
	// Status is the quoted status, set for accepted and legacy quotes and behind a mute
	// (QuoteViewMutedAccount, QuoteViewBlockedDomain). It is presented without its own quote:
	// quotes are shown one level deep (ADR-0069 §5).
	Status *StatusView
}

// quote returns the quote of st as viewer sees it, nil when st quotes nothing or its quote was
// taken away (ADR-0069 §4).
func (r *Reader) quote(ctx context.Context, viewer Viewer, st content.Status) (*QuoteView, error) {
	if r.d.Quotes == nil {
		return nil, nil
	}
	q, err := r.d.Quotes.FindQuote(ctx, st.ID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state, err := r.d.Quotes.FindQuoteState(ctx, st.ID)
	switch {
	case errors.Is(err, ErrNotFound):
		state = content.InitialQuoteState(q)
	case err != nil:
		return nil, err
	}
	switch state.Approval {
	case content.QuoteApprovalDetached:
		return nil, nil
	case content.QuoteApprovalPending:
		return &QuoteView{State: QuoteViewPending}, nil
	case content.QuoteApprovalRejected:
		return &QuoteView{State: QuoteViewRejected}, nil
	case content.QuoteApprovalRevoked:
		return &QuoteView{State: QuoteViewRevoked}, nil
	}
	shown := QuoteViewAccepted
	if state.Approval == content.QuoteApprovalLegacy {
		shown = QuoteViewLegacy
	}

	quoted, err := r.d.Statuses.FindStatusByURI(ctx, q.QuotedURI)
	if errors.Is(err, ErrNotFound) {
		return &QuoteView{State: QuoteViewPending}, nil // not fetched yet
	}
	if err != nil {
		return nil, err
	}
	if _, err := r.aud.live(ctx, quoted.ID); errors.Is(err, ErrNotFound) {
		return &QuoteView{State: QuoteViewDeleted}, nil
	} else if err != nil {
		return nil, err
	}
	// A legacy quote of a local status is shown only when the quoting author may see the status
	// (ADR-0069 §2): no approval tells otherwise.
	if shown == QuoteViewLegacy && quoted.IsLocal && !st.IsLocal && quoted.AuthorAccountID != st.AuthorAccountID {
		ok, err := r.aud.remoteVisible(ctx, st.AuthorAccountID, quoted)
		if err != nil {
			return nil, err
		}
		if !ok {
			return &QuoteView{State: QuoteViewRejected}, nil
		}
	}

	var muted, domainMuted bool
	if viewer.Kind == ViewerLocal && viewer.AccountID != "" && viewer.AccountID != quoted.AuthorAccountID {
		rel, err := r.d.Relationships.Relationship(ctx, viewer.AccountID, quoted.AuthorAccountID)
		if err != nil {
			return nil, err
		}
		if rel.Blocking || rel.BlockedBy {
			return &QuoteView{State: QuoteViewBlockedAccount}, nil
		}
		muted = rel.Muting
	}
	ok, author, err := r.aud.visible(ctx, viewer, quoted)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &QuoteView{State: QuoteViewUnauthorized}, nil
	}
	if !muted && r.d.DomainMutes != nil && viewer.Kind == ViewerLocal && viewer.AccountID != "" && !quoted.IsLocal {
		if domainMuted, err = r.d.DomainMutes.MutesDomain(ctx, viewer.AccountID, author.Account.Domain); err != nil {
			return nil, err
		}
	}
	view, err := r.presentWith(ctx, viewer, quoted, author, false)
	if errors.Is(err, ErrNotFound) {
		return &QuoteView{State: QuoteViewDeleted}, nil // deleted meanwhile
	}
	if err != nil {
		return nil, err
	}
	switch {
	case muted:
		shown = QuoteViewMutedAccount
	case domainMuted:
		shown = QuoteViewBlockedDomain
	}
	return &QuoteView{State: shown, Status: &view}, nil
}
