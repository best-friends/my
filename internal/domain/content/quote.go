// SPDX-License-Identifier: AGPL-3.0-only

package content

import "time"

// This file holds the quotes of statuses (FR-114, ADR-0069): the status that a status quotes,
// the approvals of the quote, and their current state.

// QuoteForm is how a status names the status it quotes (ADR-0069 §2).
type QuoteForm string

// Quote forms (status_quotes.form).
const (
	// QuoteFormFEP044f is FEP-044f's quote, whose author must approve it.
	QuoteFormFEP044f QuoteForm = "fep_044f"
	// QuoteFormLegacy is any other spelling (Misskey's _misskey_quote, quoteUrl, Fedibird's
	// quoteUri, an FEP-e232 Link), which carries no approval.
	QuoteFormLegacy QuoteForm = "legacy"
)

// Valid reports whether f is a known form.
func (f QuoteForm) Valid() bool { return f == QuoteFormFEP044f || f == QuoteFormLegacy }

// Quote is the status that a status quotes (status_quotes, ADR-0069 §1). It is fixed when the
// status is created; the author can only take it away (QuoteDetached).
type Quote struct {
	StatusID string
	// QuotedURI is the URI of the quoted status, which may not be stored yet.
	QuotedURI string
	Form      QuoteForm
}

// QuoteEventKind is what happened to the approval of a quote.
type QuoteEventKind string

// Quote event kinds (status_quote_events.kind).
const (
	// QuoteEventAccepted approves the quote: a QuoteAuthorization verified, or a quote of the
	// author's own status.
	QuoteEventAccepted QuoteEventKind = "accepted"
	// QuoteEventRejected refuses it: the quoted author's Reject of a QuoteRequest.
	QuoteEventRejected QuoteEventKind = "rejected"
	// QuoteEventRevoked withdraws an approval: the Delete of its QuoteAuthorization.
	QuoteEventRevoked QuoteEventKind = "revoked"
	// QuoteEventDetached takes the quote away from the quoting status: an edit without it.
	QuoteEventDetached QuoteEventKind = "detached"
)

// Valid reports whether k is a known kind.
func (k QuoteEventKind) Valid() bool {
	switch k {
	case QuoteEventAccepted, QuoteEventRejected, QuoteEventRevoked, QuoteEventDetached:
		return true
	}
	return false
}

// QuoteEvent is one change of the approval of a quote (status_quote_events).
type QuoteEvent struct {
	ID       string
	StatusID string
	Kind     QuoteEventKind
	// AuthorizationURI is the QuoteAuthorization that an accepted event verified or a revoked
	// event deleted; "" for the other kinds and for the approval of a quote of the author's own
	// status, which needs none.
	AuthorizationURI string
	OccurredAt       time.Time
}

// QuoteApproval is the state of the approval of a quote, with the values of Mastodon's
// Quote.state (ADR-0069 §1).
type QuoteApproval string

// Quote approvals (status_quote_states.state).
const (
	QuoteApprovalPending  QuoteApproval = "pending"
	QuoteApprovalAccepted QuoteApproval = "accepted"
	QuoteApprovalRejected QuoteApproval = "rejected"
	QuoteApprovalRevoked  QuoteApproval = "revoked"
	// QuoteApprovalLegacy is a legacy quote, which no approval governs.
	QuoteApprovalLegacy QuoteApproval = "legacy"
	// QuoteApprovalDetached is a quote that its author took away: the status shows none.
	QuoteApprovalDetached QuoteApproval = "detached"
)

// Shown reports whether the quoted status may be embedded in the quoting one: the quote is
// approved or legacy. Who may see the quoted status is decided on top of it, per viewer.
func (a QuoteApproval) Shown() bool { return a == QuoteApprovalAccepted || a == QuoteApprovalLegacy }

// QuoteState is the current approval of a quote (the status_quote_states projection).
type QuoteState struct {
	StatusID string
	Approval QuoteApproval
	// AuthorizationURI is the QuoteAuthorization of an accepted quote, "" when it needs none.
	AuthorizationURI string
}

// InitialQuoteState returns the state of q before any event: legacy for a legacy quote, pending
// for an FEP-044f one.
func InitialQuoteState(q Quote) QuoteState {
	s := QuoteState{StatusID: q.StatusID, Approval: QuoteApprovalPending}
	if q.Form == QuoteFormLegacy {
		s.Approval = QuoteApprovalLegacy
	}
	return s
}

// Apply returns the state after e. A detached quote stays detached. A revocation withdraws only
// the approval of the QuoteAuthorization it deletes. The approval of a legacy quote changes
// only by being taken away.
func (s QuoteState) Apply(q Quote, e QuoteEvent) QuoteState {
	if s.Approval == QuoteApprovalDetached {
		return s
	}
	if e.Kind == QuoteEventDetached {
		return QuoteState{StatusID: s.StatusID, Approval: QuoteApprovalDetached}
	}
	if q.Form == QuoteFormLegacy {
		return s
	}
	switch e.Kind {
	case QuoteEventAccepted:
		return QuoteState{StatusID: s.StatusID, Approval: QuoteApprovalAccepted, AuthorizationURI: e.AuthorizationURI}
	case QuoteEventRejected:
		return QuoteState{StatusID: s.StatusID, Approval: QuoteApprovalRejected}
	case QuoteEventRevoked:
		if s.Approval == QuoteApprovalAccepted && s.AuthorizationURI != "" && s.AuthorizationURI == e.AuthorizationURI {
			return QuoteState{StatusID: s.StatusID, Approval: QuoteApprovalRevoked}
		}
	}
	return s
}

// QuoteStateOf derives the state of q from its events in the order they occurred.
func QuoteStateOf(q Quote, events []QuoteEvent) QuoteState {
	s := InitialQuoteState(q)
	for _, e := range events {
		s = s.Apply(q, e)
	}
	return s
}
