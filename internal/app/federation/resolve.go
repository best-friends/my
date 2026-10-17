// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/federation"
)

// ErrUnresolvable reports a handle or URL that does not lead to a remote actor with a confirmed
// handle: WebFinger does not name an actor, the actor cannot be fetched or stored, its server is
// refused or unavailable, or WebFinger does not confirm its handle.
var ErrUnresolvable = errors.New("federation: the account could not be resolved")

// ResolveHandle returns the account with the handle username@domain (FR-305, federation.md
// §6.2): the stored one, or the actor that WebFinger of acct:username@domain on https://domain
// names as self, fetched and stored with Resolve. The actor must hold this handle, confirmed by
// WebFinger; a stored actor that holds another one is fetched again to check it anew. It fails
// with ErrUnresolvable; other errors come from the stores. It does network I/O, so callers call
// it outside transactions and bound it with ctx.
func (r *RemoteActors) ResolveHandle(ctx context.Context, username, domain string) (appaccount.Actor, error) {
	host, err := CanonicalHost(domain)
	if err != nil || username == "" || strings.ContainsAny(username, "@/?#: ") {
		return appaccount.Actor{}, fmt.Errorf("%w: %q@%q is not a handle", ErrUnresolvable, username, domain)
	}
	a, err := r.d.Accounts.FindByHandle(ctx, username, host)
	switch {
	case err == nil:
		if !a.Account.IsLocal() {
			r.refreshIfStale(ctx, a.Account.ID)
		}
		return a, nil
	case !errors.Is(err, appaccount.ErrNotFound):
		return appaccount.Actor{}, err
	}
	// The rules of fetching apply to WebFinger too: never this instance, a suspended server, or
	// an unavailable one.
	origin := federation.IRI("https://" + host + "/")
	if err := r.d.Fetcher.check(ctx, origin); err != nil {
		if isFetchError(err) {
			return appaccount.Actor{}, fmt.Errorf("%w: %w", ErrUnresolvable, err)
		}
		return appaccount.Actor{}, err
	}
	answer, err := r.webFinger(ctx, "https://"+host, "acct:"+username+"@"+host)
	if err != nil {
		return appaccount.Actor{}, fmt.Errorf("%w: %w", ErrUnresolvable, err)
	}
	self := answer.self()
	if self == "" {
		return appaccount.Actor{}, fmt.Errorf("%w: WebFinger of %s@%s names no actor", ErrUnresolvable, username, host)
	}
	a, err = r.ResolveURL(ctx, string(self))
	if err != nil || holdsHandle(a, username, host) {
		return a, err
	}
	// A stored actor may hold another handle until its handle is checked again, as when the
	// handle is on an account domain that WebFinger of its server names.
	a, err = r.Refresh(ctx, string(self))
	switch {
	case isFetchError(err):
		return appaccount.Actor{}, fmt.Errorf("%w: %w", ErrUnresolvable, err)
	case err != nil:
		return appaccount.Actor{}, err
	case !holdsHandle(a, username, host):
		return appaccount.Actor{}, fmt.Errorf("%w: %s does not hold the handle %s@%s", ErrUnresolvable, a.Account.URI, username, host)
	}
	return a, nil
}

// holdsHandle reports whether a is a remote actor whose confirmed handle is username@host: the
// username compared case-insensitively, host canonical.
func holdsHandle(a appaccount.Actor, username, host string) bool {
	if a.Account.IsLocal() || a.Handle == nil || !strings.EqualFold(a.Handle.Username, username) {
		return false
	}
	h, err := CanonicalHost(a.Handle.Domain)
	return err == nil && h == host
}

// ResolveURL returns the account whose actor is at uri (its id, or a URL that serves it, such as
// a profile page): the stored one, or the fetched one when it is unknown (Resolve). The actor
// must have a confirmed handle. It fails with ErrUnresolvable; other errors come from the
// stores.
func (r *RemoteActors) ResolveURL(ctx context.Context, uri string) (appaccount.Actor, error) {
	a, err := r.Resolve(ctx, uri)
	switch {
	case isFetchError(err):
		return appaccount.Actor{}, fmt.Errorf("%w: %w", ErrUnresolvable, err)
	case err != nil:
		return appaccount.Actor{}, err
	case a.Handle == nil:
		return appaccount.Actor{}, fmt.Errorf("%w: WebFinger does not confirm the handle of %s", ErrUnresolvable, a.Account.URI)
	}
	return a, nil
}
