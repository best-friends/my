// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appaccount "github.com/best-friends/my/internal/app/account"
)

// ErrHandleNotFound reports a handle that no remote actor holds: WebFinger of it does not name
// an actor, or the actor it names does not verify the handle (federation.md §6.2).
var ErrHandleNotFound = errors.New("federation: no remote actor holds the handle")

// ResolveHandle returns the remote actor with the handle username@domain (FR-305, federation.md
// §6.2): the stored one, or else the actor that WebFinger of acct:username@domain on domain
// names as self, fetched and stored, provided that its handle check verifies this handle. A
// stored actor that is stale is returned as it is and refreshed asynchronously.
//
// It fails with ErrHandleNotFound when no remote actor holds the handle, ErrFetchFailed when
// WebFinger did not answer, and the errors of Fetcher.Fetch. It does network I/O, so callers
// call it outside transactions and bound it with ctx.
func (r *RemoteActors) ResolveHandle(ctx context.Context, username, domain string) (appaccount.Actor, error) {
	notFound := fmt.Errorf("%w: %s@%s", ErrHandleNotFound, username, domain)
	host, err := CanonicalHost(domain)
	if err != nil || username == "" || strings.ContainsAny(username, "@/?#: ") {
		return appaccount.Actor{}, notFound
	}
	a, err := r.d.Accounts.FindByHandle(ctx, username, host)
	switch {
	case err == nil && a.Account.IsLocal():
		return appaccount.Actor{}, notFound
	case err == nil:
		r.refreshIfStale(ctx, a.Account.ID)
		return a, nil
	case !errors.Is(err, appaccount.ErrNotFound):
		return appaccount.Actor{}, err
	}

	answer, err := r.webFinger(ctx, "https://"+host, "acct:"+username+"@"+host)
	switch {
	case errors.Is(err, errWebFingerUnanswered):
		return appaccount.Actor{}, fmt.Errorf("%w: %w", ErrFetchFailed, err)
	case err != nil:
		return appaccount.Actor{}, fmt.Errorf("%w: %w", notFound, err)
	}
	self := answer.self()
	if self == "" {
		return appaccount.Actor{}, fmt.Errorf("%w: WebFinger names no actor", notFound)
	}
	a, err = r.Refresh(ctx, string(self))
	if err != nil {
		return appaccount.Actor{}, err
	}
	if a.Account.IsLocal() || a.Handle == nil ||
		!strings.EqualFold(a.Handle.Username, username) || a.Handle.Domain != host {
		return appaccount.Actor{}, fmt.Errorf("%w: %s does not verify it", notFound, a.Account.URI)
	}
	return a, nil
}
