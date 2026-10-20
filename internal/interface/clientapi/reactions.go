// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/notification"
)

// The emoji reactions to statuses (FR-116, ADR-0052 §5): their counts on Status, the list of
// the accounts that reacted, and the emoji of a reaction notification.

// newReactionEmoji presents the emoji of a reaction as the ReactionEmoji schema.
func newReactionEmoji(v appcontent.EmojiView) ReactionEmoji {
	out := ReactionEmoji{Name: v.Name()}
	if c := v.Custom; c != nil {
		shortcode := c.Shortcode
		out.Shortcode = &shortcode
		if c.Domain != "" {
			domain := c.Domain
			out.Domain = &domain
		}
		if c.URL != "" {
			u := c.URL
			out.Url = &u
		}
	}
	return out
}

// newStatusReactions presents the reaction counts of a status; me is set for a signed-in viewer.
func newStatusReactions(in []appcontent.ReactionView, signedIn bool) []StatusReaction {
	out := make([]StatusReaction, 0, len(in))
	for _, r := range in {
		e := newReactionEmoji(r.EmojiView)
		item := StatusReaction{Name: e.Name, Shortcode: e.Shortcode, Domain: e.Domain, Url: e.Url, Count: int(r.Count)}
		if signedIn {
			me := r.Me
			item.Me = &me
		}
		out = append(out, item)
	}
	return out
}

// reactionEmojiOf presents the emoji of a reaction notification, nil for another type or when
// the emoji cannot be read.
func (s *Server) reactionEmojiOf(ctx context.Context, n notification.Notification) (*ReactionEmoji, error) {
	if n.Type != notification.TypeReaction {
		return nil, nil
	}
	if n.ReactionCustomEmojiID == "" {
		return &ReactionEmoji{Name: n.ReactionEmoji}, nil
	}
	if s.o.StatusReader == nil {
		return nil, nil
	}
	v, err := s.o.StatusReader.ReactionEmoji(ctx, appcontent.ReactionEmoji{CustomEmojiID: n.ReactionCustomEmojiID})
	if err != nil {
		return nil, fmt.Errorf("clientapi: the emoji of notification %s: %w", n.ID, err)
	}
	e := newReactionEmoji(v)
	return &e, nil
}

// ListStatusReactions serves GET /api/v1/statuses/{id}/reactions.
func (s *Server) ListStatusReactions(ctx context.Context, req ListStatusReactionsRequestObject) (ListStatusReactionsResponseObject, error) {
	v := viewerOf(ctx)
	q := s.parsePage(ctx, req.Params.Cursor, req.Params.Limit)
	if q.problem != nil {
		return ListStatusReactionsdefaultApplicationProblemPlusJSONResponse(*q.problem), nil
	}
	page, err := s.o.StatusReader.Reactions(ctx, contentViewer(ctx), req.Id, deref(req.Params.Emoji), q.before, q.limit+1)
	if err != nil {
		if p, ok := s.statusProblem(ctx, err); ok {
			return ListStatusReactionsdefaultApplicationProblemPlusJSONResponse(p), nil
		}
		return nil, err
	}
	reactions, last := trim(page.Reactions, q.limit)
	var actors []appaccount.Actor
	var kept []appcontent.ReactedView
	for _, r := range reactions {
		a, ok, err := s.presentableByID(ctx, v, r.Reaction.AccountID, true)
		if err != nil {
			return nil, err
		}
		if ok {
			actors = append(actors, a)
			kept = append(kept, r)
		}
	}
	accounts, err := s.newAccounts(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := StatusReactionPage{Items: make([]StatusReactor, 0, len(kept))}
	for i, r := range kept {
		out.Items = append(out.Items, StatusReactor{Account: accounts[i], Emoji: newReactionEmoji(r.Emoji), CreatedAt: r.Reaction.CreatedAt})
	}
	if last >= 0 {
		out.NextCursor = encodeCursor(reactions[last].Reaction.ID)
	}
	return ListStatusReactions200JSONResponse{Body: out, Headers: ListStatusReactions200ResponseHeaders{CacheControl: statusCache(ctx)}}, nil
}
