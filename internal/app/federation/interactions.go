// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"errors"
	"fmt"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// Boosts and favourites across servers (federation.md §3.4, §7.1): the outbound projectors that
// turn a local account's favourite of a remote status into Like and Undo Like, their fan-out to
// the status's author, and the inbox handlers of the Announce, Like, EmojiReact, and their Undo
// that remote servers send. The Announce and Undo Announce of local boosts are recorded with the
// other content activities (RegisterOutboundProjectors).

// registerFavouriteProjectors records the Like and Undo Like of a favourite of a remote status
// (federation.md §7.1). Remote accounts favourite only local statuses, so the favourite of a
// remote status is a local account's. A favourite is a current value whose row goes when it is
// taken back, so the activity names the status (object_id) and its actor the account, the pair
// that identifies the favourite, and its document is built here from the event and stored with
// it: the Undo Like embeds the Like by the favourite's URI after the row is gone.
func registerFavouriteProjectors(r *uow.Projectors, rec recorder) {
	record := func(ctx context.Context, tx uow.Tx, t federation.ActivityType, f appcontent.Favourite, st appcontent.StatusRef, at time.Time) error {
		if st.IsLocal || st.URI == "" {
			return nil
		}
		a := federation.OutboundActivity{
			Type: t, ActorAccountID: f.AccountID, ObjectKind: federation.OutboundFavourite, ObjectID: st.ID, CreatedAt: at,
		}
		if t == federation.ActivityLike {
			a.ActivityURI = federation.IRI(f.URI)
		}
		act, err := rec.insert(ctx, tx, a)
		if err != nil {
			return err
		}
		doc, err := likeDocument(act, rec.d.Host, f, st)
		if err != nil {
			return err
		}
		if _, err := rec.d.Repository.InsertOutboundDocument(ctx, tx, act.ID, doc, act.CreatedAt); err != nil {
			return err
		}
		return rec.enqueue(ctx, tx, act, fanOutPayload{})
	}
	r.Register(appcontent.EventFavourited, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.Favourited)
		if !ok {
			return nil
		}
		return record(ctx, tx, federation.ActivityLike, ev.Favourite, ev.Status, ev.Favourite.CreatedAt)
	})
	r.Register(appcontent.EventUnfavourited, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.Unfavourited)
		if !ok {
			return nil
		}
		return record(ctx, tx, federation.ActivityUndo, ev.Favourite, ev.Status, ev.RemovedAt)
	})
}

// likeDocument returns the Like of favourite f of the remote status st, or for an Undo the Undo
// that embeds it. Like Mastodon's, they carry no addressing: they go to the status's author
// alone, and the Like's id is the favourite's URI.
func likeDocument(act federation.OutboundActivity, host string, f appcontent.Favourite, st appcontent.StatusRef) ([]byte, error) {
	actor := federation.IRI(appaccount.ActorURI(host, f.AccountID))
	like := federation.NewActivityDocument(federation.IRI(f.URI), federation.ActivityLike, actor, f.CreatedAt,
		federation.Audience{}, federation.IRI(st.URI))
	if act.Type == federation.ActivityLike {
		return like.Marshal()
	}
	return federation.NewActivityDocument(act.ActivityURI, act.Type, actor, act.CreatedAt, federation.Audience{}, like.Embedded()).Marshal()
}

// favouriteSubject loads the remote status of a Like or Undo Like, whose author receives it.
func (f *FanOut) favouriteSubject(ctx context.Context, s subject) (subject, error) {
	act := s.act
	if act.Type != federation.ActivityLike && act.Type != federation.ActivityUndo {
		return subject{}, fmt.Errorf("%w: %s of a favourite", federation.ErrInvalidOutbound, act.Type)
	}
	st, err := f.d.Statuses.FindStatus(ctx, act.ObjectID)
	switch {
	case errors.Is(err, appcontent.ErrNotFound):
		return subject{}, fmt.Errorf("%w: status %s", errGone, act.ObjectID)
	case err != nil:
		return subject{}, err
	case st.IsLocal:
		return subject{}, fmt.Errorf("%w: %s of the favourite of a local status %s", federation.ErrInvalidOutbound, act.Type, st.ID)
	}
	s.status = st
	s.mentioned = []string{st.AuthorAccountID}
	return s, nil
}

// statusAuthorTargets returns the inbox of the author of a favourited status: its shared inbox,
// or its own inbox without one.
func (f *FanOut) statusAuthorTargets(ctx context.Context, s subject) ([]target, error) {
	inbox, err := f.d.Accounts.RemoteInbox(ctx, s.status.AuthorAccountID)
	if errors.Is(err, appaccount.ErrNotFound) {
		return nil, nil // a remote author never fetched
	}
	if err != nil {
		return nil, err
	}
	if t, ok := targetOf(federation.InboxFor(federation.IRI(inbox.InboxURL), federation.IRI(inbox.SharedInboxURL))); ok {
		return []target{t}, nil
	}
	return nil, nil
}

// RemoteInteractions is the part of the content module's public API that records the boosts and
// favourites of remote accounts (federation.md §1.1); *appcontent.Interactions implements it.
type RemoteInteractions interface {
	RecordRemoteBoost(ctx context.Context, in appcontent.RemoteBoostInput) (appcontent.Boost, error)
	RetractRemoteBoost(ctx context.Context, uri, boosterAccountID string) error
	RecordRemoteFavourite(ctx context.Context, in appcontent.RemoteFavouriteInput) (appcontent.Favourite, error)
	RemoveRemoteFavourite(ctx context.Context, uri, accountID, statusID string) error
}

// InteractionHandlersDeps are the ports of the boost and favourite inbox handlers.
type InteractionHandlersDeps struct {
	Interactions RemoteInteractions
	// Statuses finds the posts that activities are about, and resolves and stores the remote
	// post that an Announce boosts.
	Statuses *StatusHandlers
}

// RegisterInteractionHandlers sets the inbox handlers of boosts and favourites (federation.md
// §3.4):
//
//   - Announce of a post: content.RecordRemoteBoost. The post is found by its id; a remote post
//     that is not stored is taken from the Announce when it is embedded from the actor's origin,
//     fetched from its origin otherwise, and stored only when a local account follows the
//     booster (ADR-0034). A local post must be one the booster may see and boost.
//   - Like and EmojiReact of a local post: content.RecordRemoteFavourite. A Misskey reaction (a
//     Like with _misskey_reaction) and an Akkoma EmojiReact are favourites too.
//   - Undo of an Announce, Like, or EmojiReact: content.RetractRemoteBoost and
//     content.RemoveRemoteFavourite, found by the URI of the activity taken back; an unknown one
//     leaves a tombstone (federation.md §3.3).
func RegisterInteractionHandlers(p *InboxProcessor, d InteractionHandlersDeps) {
	h := interactionHandlers{d: d}
	p.Handle(federation.ActivityAnnounce, h.announce)
	p.Handle(federation.ActivityLike, h.like)
	p.Handle(federation.ActivityEmojiReact, h.like)
	p.HandleUndo(federation.ActivityAnnounce, h.undoAnnounce)
	p.HandleUndo(federation.ActivityLike, h.undoLike)
	p.HandleUndo(federation.ActivityEmojiReact, h.undoLike)
}

type interactionHandlers struct{ d InteractionHandlersDeps }

// interactionError turns the refusals of the content module's remote interaction commands into
// what the processor makes of them: a post that is missing, deleted, or hidden from the actor,
// or that the actor may not boost, has nothing to do (federation.md §3.3).
func interactionError(err error) error {
	switch {
	case errors.Is(err, appcontent.ErrNotFound), errors.Is(err, appcontent.ErrNotBoostable),
		errors.Is(err, appcontent.ErrActorUnavailable), errors.Is(err, appcontent.ErrNotAuthor):
		return fmt.Errorf("%w: %w", ErrDropActivity, err)
	}
	return err
}

// activityURI returns the IRI of a received activity: its id, or the digest URI of one without.
func activityURI(in *InboxActivity) string {
	if in.Activity.ID != "" {
		return string(in.Activity.ID)
	}
	return string(in.Received.ActivityURI)
}

func (h interactionHandlers) announce(ctx context.Context, in *InboxActivity) error {
	st, err := h.d.Statuses.boosted(ctx, in)
	if err != nil {
		return err
	}
	followers, err := h.d.Statuses.followers(ctx, in.Actor)
	if err != nil {
		return err
	}
	_, err = h.d.Interactions.RecordRemoteBoost(ctx, appcontent.RemoteBoostInput{
		URI: activityURI(in), BoosterAccountID: in.Actor.Account.ID, StatusID: st.ID,
		Visibility: appcontent.Visibility(in.Activity.Audience.Visibility(followers)), BoostedAt: in.Activity.Published,
	})
	return interactionError(err)
}

func (h interactionHandlers) like(ctx context.Context, in *InboxActivity) error {
	id := in.Activity.Object().ID
	if id == "" || !h.d.Statuses.isLocal(id) {
		return fmt.Errorf("%w: %s of %q, not of a local post", ErrDropActivity, in.Activity.RawType, id)
	}
	st, err := h.d.Statuses.d.Statuses.FindStatusByURI(ctx, string(id))
	if err != nil {
		return interactionError(err)
	}
	_, err = h.d.Interactions.RecordRemoteFavourite(ctx, appcontent.RemoteFavouriteInput{
		URI: activityURI(in), AccountID: in.Actor.Account.ID, StatusID: st.ID, CreatedAt: in.Activity.Published,
	})
	return interactionError(err)
}

// undone returns the id of the activity that an Undo takes back, and the activity when the Undo
// embeds it. The embedded activity is a hint only: the content module matches it against the
// actor's own boosts and favourites.
func undone(in *InboxActivity) (federation.IRI, *federation.Activity) {
	ref := in.Activity.Object()
	if doc := ref.Untrusted(); doc != nil && doc.Activity != nil {
		return ref.ID, doc.Activity
	}
	return ref.ID, nil
}

func (h interactionHandlers) undoAnnounce(ctx context.Context, in *InboxActivity) error {
	id, _ := undone(in)
	if id == "" {
		return ErrUnknownObject
	}
	err := h.d.Interactions.RetractRemoteBoost(ctx, string(id), in.Actor.Account.ID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return fmt.Errorf("%w: boost %s", ErrUnknownObject, id)
	}
	return interactionError(err)
}

func (h interactionHandlers) undoLike(ctx context.Context, in *InboxActivity) error {
	id, like := undone(in)
	if id == "" && like == nil {
		return ErrUnknownObject
	}
	// Without a favourite of the Like's URI, the embedded Like names the post the actor took
	// its favourite back from.
	var statusID string
	if like != nil && like.Actor == in.Activity.Actor {
		if target := like.Object().ID; target != "" && h.d.Statuses.isLocal(target) {
			st, err := h.d.Statuses.d.Statuses.FindStatusByURI(ctx, string(target))
			switch {
			case err == nil:
				statusID = st.ID
			case !errors.Is(err, appcontent.ErrNotFound):
				return err
			}
		}
	}
	err := h.d.Interactions.RemoveRemoteFavourite(ctx, string(id), in.Actor.Account.ID, statusID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return fmt.Errorf("%w: favourite %s", ErrUnknownObject, id)
	}
	return interactionError(err)
}

// boosted returns the post that an Announce boosts (federation.md §3.4, ADR-0034). A local post
// and a stored remote post are found by the object's id. A remote post that is not stored is
// stored only when a local account follows the booster: from the Announce when it embeds the
// post from the booster's origin, fetched from the post's origin otherwise. Anything else drops
// the Announce.
func (h *StatusHandlers) boosted(ctx context.Context, in *InboxActivity) (appcontent.Status, error) {
	act := in.Activity
	ref := act.Object()
	if ref.ID == "" {
		return appcontent.Status{}, fmt.Errorf("%w: Announce without an object", ErrDropActivity)
	}
	st, err := h.d.Statuses.FindStatusByURI(ctx, string(ref.ID))
	switch {
	case err == nil:
		return st, nil
	case !errors.Is(err, appcontent.ErrNotFound):
		return appcontent.Status{}, err
	case h.isLocal(ref.ID):
		return appcontent.Status{}, fmt.Errorf("%w: Announce of an unknown local post %s", ErrDropActivity, ref.ID)
	}
	followed, err := h.followed(ctx, in.Actor)
	if err != nil {
		return appcontent.Status{}, err
	}
	if !followed {
		return appcontent.Status{}, fmt.Errorf("%w: Announce of %s by %s, whom no local account follows (ADR-0034)", ErrDropActivity, ref.ID, act.Actor)
	}
	tombstoned, err := h.d.Statuses.Tombstoned(ctx, string(ref.ID))
	if err != nil {
		return appcontent.Status{}, err
	}
	if tombstoned {
		return appcontent.Status{}, fmt.Errorf("%w: %s is deleted", ErrDropActivity, ref.ID)
	}

	var doc *federation.Document
	if federation.SameOrigin(ref.ID, act.Actor) {
		doc = ref.Trusted()
	}
	if doc == nil {
		got, err := h.d.Fetcher.Fetch(ctx, string(ref.ID))
		if err != nil {
			return appcontent.Status{}, err
		}
		doc = got.Document
	}
	obj := doc.Object
	switch {
	case obj == nil || obj.ID != ref.ID:
		return appcontent.Status{}, fmt.Errorf("%w: %s is not an object", ErrDropActivity, ref.ID)
	case !isPost(obj.Type):
		return appcontent.Status{}, fmt.Errorf("%w: %s is a %s, not a post", ErrDropActivity, obj.ID, obj.RawType)
	case obj.AttributedTo == "" || !federation.SameOrigin(obj.ID, obj.AttributedTo):
		return appcontent.Status{}, fmt.Errorf("%w: %s is not attributed to an actor of its origin", ErrDropActivity, obj.ID)
	}
	author, err := h.d.Actors.Resolve(ctx, string(obj.AttributedTo))
	if err != nil {
		return appcontent.Status{}, err
	}
	if author.Account.IsLocal() {
		return appcontent.Status{}, fmt.Errorf("%w: %s claims a local author", ErrDropActivity, obj.ID)
	}
	followers, err := h.followers(ctx, author)
	if err != nil {
		return appcontent.Status{}, err
	}
	mentions, err := h.mentions(ctx, obj, followers)
	if err != nil {
		return appcontent.Status{}, err
	}
	if err := h.resolveMentions(ctx, mentions); err != nil {
		return appcontent.Status{}, err
	}
	post := h.inputOf(author, federation.Audience{}, "", obj, followers, mentions)
	err = h.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		_, err := h.d.Statuses.IngestRemoteStatus(ctx, tx, post)
		return err
	})
	if err != nil && !errors.Is(err, appcontent.ErrStatusExists) {
		return appcontent.Status{}, err
	}
	return h.d.Statuses.FindStatusByURI(ctx, string(obj.ID))
}
