// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/federation"
)

// document builds the body of act (federation.md §7.4): the activity envelope with its context
// and addressing around the object. The object of a Create or Update is the status's Note as of
// the revision the activity presents, with its identity, author, reply target, and addressing.
func (f *FanOut) document(ctx context.Context, act federation.OutboundActivity, s subject) ([]byte, error) {
	actor, err := f.actorIRI(ctx, act.ActorAccountID)
	if err != nil {
		return nil, err
	}
	mentioned, err := f.actorIRIs(ctx, s.mentioned)
	if err != nil {
		return nil, err
	}
	aud := federation.Address(federation.Visibility(s.visibility), federation.FollowersOf(actor), mentioned)

	var doc federation.ActivityDocument
	switch {
	case act.ObjectKind == federation.OutboundStatus && act.Type == federation.ActivityDelete:
		uri := s.status.URI
		if uri == "" {
			uri = appcontent.StatusURI(f.d.Host, act.ObjectID)
		}
		doc = federation.NewActivityDocument(act.ActivityURI, act.Type, actor, act.CreatedAt, aud, federation.TombstoneOf(federation.IRI(uri)))
	case act.ObjectKind == federation.OutboundStatus:
		note := federation.ObjectDocument{
			ID: federation.IRI(s.status.URI), Type: federation.ObjectNote,
			Published: federation.FormatTime(s.status.PublishedAt), URL: s.status.URL,
			AttributedTo: actor, To: aud.To, Cc: aud.Cc,
		}
		if s.status.InReplyToURI != "" {
			r := federation.IRI(s.status.InReplyToURI)
			note.InReplyTo = &r
		}
		published := s.status.PublishedAt
		if act.Type == federation.ActivityUpdate {
			note.Updated = federation.FormatTime(s.revision.RevisedAt)
			published = s.revision.RevisedAt
		}
		doc = federation.NewActivityDocument(act.ActivityURI, act.Type, actor, published, aud, note)
	case act.ObjectKind == federation.OutboundBoost:
		announce := federation.NewActivityDocument(federation.IRI(s.boost.URI), federation.ActivityAnnounce, actor,
			s.boost.BoostedAt, aud, federation.IRI(s.status.URI))
		doc = announce
		if act.Type == federation.ActivityUndo {
			doc = federation.NewActivityDocument(act.ActivityURI, act.Type, actor, act.CreatedAt, aud, announce.Embedded())
		}
	default:
		return nil, fmt.Errorf("%w: %s of a %s", federation.ErrInvalidOutbound, act.Type, act.ObjectKind)
	}
	return doc.Marshal()
}

// actorIRI returns the ActivityPub id of an account.
func (f *FanOut) actorIRI(ctx context.Context, accountID string) (federation.IRI, error) {
	a, err := f.d.Accounts.FindByID(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("actor %s: %w", accountID, err)
	}
	return federation.IRI(a.Account.URI), nil
}

// actorIRIs returns the ids of the accounts that exist among ids, in order.
func (f *FanOut) actorIRIs(ctx context.Context, ids []string) ([]federation.IRI, error) {
	var out []federation.IRI
	for _, id := range ids {
		iri, err := f.actorIRI(ctx, id)
		if errors.Is(err, appaccount.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, iri)
	}
	return out, nil
}
