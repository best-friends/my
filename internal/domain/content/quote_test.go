// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"testing"

	"github.com/best-friends/my/internal/domain/content"
)

func TestQuoteStateOf(t *testing.T) {
	fep := content.Quote{StatusID: "s", QuotedURI: "https://b.example/n/1", Form: content.QuoteFormFEP044f}
	legacy := content.Quote{StatusID: "s", QuotedURI: "https://b.example/n/1", Form: content.QuoteFormLegacy}
	accepted := func(uri string) content.QuoteEvent {
		return content.QuoteEvent{Kind: content.QuoteEventAccepted, AuthorizationURI: uri}
	}
	revoked := func(uri string) content.QuoteEvent {
		return content.QuoteEvent{Kind: content.QuoteEventRevoked, AuthorizationURI: uri}
	}
	detached := content.QuoteEvent{Kind: content.QuoteEventDetached}
	rejected := content.QuoteEvent{Kind: content.QuoteEventRejected}
	for _, tc := range []struct {
		name   string
		q      content.Quote
		events []content.QuoteEvent
		want   content.QuoteState
	}{
		{"FEP-044f starts pending", fep, nil, content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalPending}},
		{"legacy starts legacy", legacy, nil, content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalLegacy}},
		{"accepted", fep, []content.QuoteEvent{accepted("a1")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalAccepted, AuthorizationURI: "a1"}},
		{"accepted without an authorization (own status)", fep, []content.QuoteEvent{accepted("")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalAccepted}},
		{"revoked", fep, []content.QuoteEvent{accepted("a1"), revoked("a1")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalRevoked}},
		{"revoking another authorization", fep, []content.QuoteEvent{accepted("a1"), revoked("a0")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalAccepted, AuthorizationURI: "a1"}},
		{"approved again after a revocation", fep, []content.QuoteEvent{accepted("a1"), revoked("a1"), accepted("a2")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalAccepted, AuthorizationURI: "a2"}},
		{"rejected", fep, []content.QuoteEvent{rejected}, content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalRejected}},
		{"detached stays detached", fep, []content.QuoteEvent{accepted("a1"), detached, accepted("a2")},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalDetached}},
		{"legacy ignores approvals", legacy, []content.QuoteEvent{accepted("a1"), rejected},
			content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalLegacy}},
		{"legacy detached", legacy, []content.QuoteEvent{detached}, content.QuoteState{StatusID: "s", Approval: content.QuoteApprovalDetached}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := content.QuoteStateOf(tc.q, tc.events); got != tc.want {
				t.Errorf("QuoteStateOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestQuoteApprovalShown(t *testing.T) {
	for a, want := range map[content.QuoteApproval]bool{
		content.QuoteApprovalAccepted: true, content.QuoteApprovalLegacy: true, content.QuoteApprovalPending: false,
		content.QuoteApprovalRejected: false, content.QuoteApprovalRevoked: false, content.QuoteApprovalDetached: false,
	} {
		if got := a.Shown(); got != want {
			t.Errorf("%s.Shown() = %v, want %v", a, got, want)
		}
	}
}
