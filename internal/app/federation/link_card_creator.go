// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"strings"

	appcontent "github.com/mycroblog/my/internal/app/content"
)

var _ appcontent.LinkCardCreators = (*RemoteActors)(nil)

// VerifyCreator implements content.LinkCardCreators (ADR-0062 §5): it resolves handle, the
// fediverse:creator of a page ("@user@domain" or "user@domain"), with ResolveHandle, fetches the
// actor's document again, and returns the account when the actor's attributionDomains hold host,
// the page's host, or one of its parents. A handle that does not resolve or verify returns ""
// with a nil error; local accounts never verify, since they declare no attributionDomains.
func (r *RemoteActors) VerifyCreator(ctx context.Context, handle, host string) (string, error) {
	username, domain, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(handle), "@"), "@")
	if !ok || username == "" || domain == "" {
		return "", nil
	}
	pageHost, err := CanonicalHost(host)
	if err != nil {
		return "", nil
	}
	a, err := r.ResolveHandle(ctx, username, domain)
	switch {
	case errors.Is(err, ErrUnresolvable):
		return "", nil
	case err != nil:
		return "", err
	case a.Account.IsLocal():
		return "", nil
	}
	actor, _, err := r.fetchActor(ctx, a.Account.URI)
	switch {
	case isFetchError(err):
		return "", nil
	case err != nil:
		return "", err
	}
	for _, d := range actor.AttributionDomains {
		d, err := CanonicalHost(d)
		if err == nil && (pageHost == d || strings.HasSuffix(pageHost, "."+d)) {
			return a.Account.ID, nil
		}
	}
	return "", nil
}
