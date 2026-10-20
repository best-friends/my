// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"strings"

	"github.com/best-friends/my/internal/domain/content"
)

// This file holds what the Reader presents of the emoji reactions to a status (ADR-0052 §5): the
// counts per emoji on the status, and the list of the accounts that reacted.

// ReactionView is the count of the reactions to a status with one emoji, as a viewer sees it.
type ReactionView struct {
	EmojiView
	Count int64
	// Me is whether the viewer reacted with the emoji; always false for an anonymous viewer.
	Me bool
}

// EmojiView is the emoji of a reaction with what a client shows of it.
type EmojiView struct {
	Emoji content.ReactionEmoji
	// Custom is the custom emoji, nil for a Unicode emoji.
	Custom *CustomEmojiView
}

// Name returns the name that clients show and filter the reactions with: the Unicode emoji,
// the shortcode of a custom emoji of this instance, or "shortcode@domain" for one of another
// server.
func (v EmojiView) Name() string {
	switch c := v.Custom; {
	case c == nil:
		return v.Emoji.Unicode
	case c.Domain == "":
		return c.Shortcode
	default:
		return c.Shortcode + "@" + c.Domain
	}
}

// CustomEmojiView is a custom emoji with the URL of its current image: its upload's full
// variant, or the media proxy URL of a remote image; "" when it has none to show.
type CustomEmojiView struct {
	ID        string
	Shortcode string
	// Domain is the server of the emoji, "" for this instance.
	Domain string
	URL    string
}

// ReactedView is one reaction to a status, for the list of the accounts that reacted.
type ReactedView struct {
	Reaction content.Reaction
	Emoji    EmojiView
}

// ReactedPage is a page of the reactions to a status.
type ReactedPage struct {
	Reactions []ReactedView
	// Next is the cursor of the next page, "" on the last page.
	Next string
}

// Reactions returns a page of the reactions to a status that viewer may see, newest first: up to
// limit reactions older than the cursor before ("" for the newest), only those with the emoji
// whose name (EmojiView.Name) is emoji unless it is "". An emoji that no reaction can have
// gives an empty page. A status that the viewer may not see fails with ErrNotFound.
func (r *Reader) Reactions(ctx context.Context, viewer Viewer, statusID, emoji, before string, limit int) (ReactedPage, error) {
	if _, _, err := r.aud.visibleStatus(ctx, viewer, statusID); err != nil {
		return ReactedPage{}, err
	}
	var key string
	if emoji != "" {
		var ok bool
		var err error
		if key, ok, err = r.emojiKey(ctx, emoji); err != nil || !ok {
			return ReactedPage{}, err
		}
	}
	rs, err := r.d.Interactions.ListStatusReactions(ctx, statusID, key, before, limit)
	if err != nil {
		return ReactedPage{}, err
	}
	emojis := emojiCache{r: r, seen: map[string]*CustomEmojiView{}}
	out := ReactedPage{Reactions: make([]ReactedView, 0, len(rs))}
	for _, re := range rs {
		v, err := emojis.view(ctx, re.Emoji)
		if err != nil {
			return ReactedPage{}, err
		}
		out.Reactions = append(out.Reactions, ReactedView{Reaction: re, Emoji: v})
	}
	if len(rs) == limit && limit > 0 {
		out.Next = rs[len(rs)-1].ID
	}
	return out, nil
}

// emojiKey returns the key (content.ReactionEmoji.Key) of the emoji whose name is name, and
// false when no stored emoji has it.
func (r *Reader) emojiKey(ctx context.Context, name string) (string, bool, error) {
	if u, ok := content.NormalizeUnicodeEmoji(name); ok {
		return u, true, nil
	}
	shortcode, domain, _ := strings.Cut(name, "@")
	e, err := r.d.Statuses.FindCustomEmojiByShortcode(ctx, shortcode, domain)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return e.ID, true, nil
}

// reactions returns the counts of the reactions to st, the most used content.MaxStatusReactions
// emoji, with whether accountID ("" for an anonymous viewer) reacted with each.
func (r *Reader) reactions(ctx context.Context, st content.Status, accountID string) ([]ReactionView, error) {
	counts, err := r.d.Interactions.ListReactionCounts(ctx, st.ID, content.MaxStatusReactions)
	if err != nil || len(counts) == 0 {
		return nil, err
	}
	mine := map[string]bool{}
	if accountID != "" {
		rs, err := r.d.Interactions.ListAccountStatusReactions(ctx, accountID, st.ID)
		if err != nil {
			return nil, err
		}
		for _, re := range rs {
			mine[re.Emoji.Key()] = true
		}
	}
	emojis := emojiCache{r: r, seen: map[string]*CustomEmojiView{}}
	out := make([]ReactionView, 0, len(counts))
	for _, c := range counts {
		v, err := emojis.view(ctx, c.Emoji)
		if err != nil {
			return nil, err
		}
		out = append(out, ReactionView{EmojiView: v, Count: c.Count, Me: mine[c.Emoji.Key()]})
	}
	return out, nil
}

// emojiCache presents the emoji of reactions, reading each custom emoji once.
type emojiCache struct {
	r    *Reader
	seen map[string]*CustomEmojiView
}

func (c emojiCache) view(ctx context.Context, e content.ReactionEmoji) (EmojiView, error) {
	if e.CustomEmojiID == "" {
		return EmojiView{Emoji: e}, nil
	}
	if v, ok := c.seen[e.CustomEmojiID]; ok {
		return EmojiView{Emoji: e, Custom: v}, nil
	}
	emoji, err := c.r.d.Statuses.FindCustomEmoji(ctx, e.CustomEmojiID)
	if err != nil {
		return EmojiView{}, err
	}
	v := &CustomEmojiView{ID: emoji.ID, Shortcode: emoji.Shortcode, Domain: emoji.Domain}
	if v.URL, err = c.r.emojiImageURL(ctx, emoji.ID); err != nil {
		return EmojiView{}, err
	}
	c.seen[e.CustomEmojiID] = v
	return EmojiView{Emoji: e, Custom: v}, nil
}

// emojiImageURL returns the URL of the current image of a custom emoji, "" when it has none to
// show (no image, an upload that did not finish processing, or no media proxy).
func (r *Reader) emojiImageURL(ctx context.Context, emojiID string) (string, error) {
	if r.d.Media == nil {
		return "", nil
	}
	img, err := r.d.Statuses.FindCustomEmojiImage(ctx, emojiID)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	views, err := r.attachments(ctx, []content.Attachment{{MediaID: img.MediaID}})
	if err != nil || len(views) == 0 {
		return "", err
	}
	return views[0].URL, nil
}

// ReactionEmoji returns the emoji of a reaction with what a client shows of it, for the
// notifications of reactions.
func (r *Reader) ReactionEmoji(ctx context.Context, e content.ReactionEmoji) (EmojiView, error) {
	return emojiCache{r: r, seen: map[string]*CustomEmojiView{}}.view(ctx, e)
}
