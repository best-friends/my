// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/account"
)

// Accounts reads accounts for the client API.
type Accounts interface {
	// FindByID returns the actor with an account ID, failing with appaccount.ErrNotFound when
	// there is none.
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
}

// cacheNoStore is the Cache-Control of responses that depend on the viewer (ADR-0016).
const cacheNoStore = "no-store"

// newAccount presents an actor as the Account schema.
func (s *Server) newAccount(a appaccount.Actor) Account {
	out := Account{Id: a.Account.ID, CreatedAt: a.Account.CreatedAt, Url: a.Account.URI}
	switch {
	case a.Local != nil:
		out.Username, out.Acct = a.Local.Username, a.Local.Username
		out.Url = "https://" + s.o.Host + "/@" + a.Local.Username
	case a.Handle != nil:
		out.Username = a.Handle.Username
		out.Acct = a.Handle.Username
		if a.Handle.Domain != account.LocalDomain {
			out.Acct += "@" + a.Handle.Domain
		}
	}
	if p := a.Profile; p != nil {
		out.DisplayName, out.Note, out.Locked, out.Bot = p.DisplayName, p.NoteHTML, p.Locked, p.Bot
	}
	return out
}

// VerifyAccountCredentials serves GET /api/v1/accounts/verify_credentials: the viewer's own
// account with the source of its profile.
func (s *Server) VerifyAccountCredentials(ctx context.Context, _ VerifyAccountCredentialsRequestObject) (VerifyAccountCredentialsResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.o.Accounts.FindByID(ctx, v.AccountID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the viewer's account %s: %w", v.AccountID, err)
	}
	body, err := s.newCredentialAccount(ctx, a)
	if err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return VerifyAccountCredentials200JSONResponse{
		Body:    body,
		Headers: VerifyAccountCredentials200ResponseHeaders{CacheControl: &cache},
	}, nil
}
