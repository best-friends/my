// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/best-friends/my/internal/domain/content"
)

// CustomEmojiView is a custom emoji as clients show it (ADR-0051 §4): its shortcode without
// colons, the URL of its image, and the URL of a still image of it.
type CustomEmojiView struct {
	ID        string
	Shortcode string
	URL       string
	StaticURL string
}

// CustomEmojis returns the custom emoji with an ID in ids that can be shown, keyed by ID, reading
// them together so that a page of statuses and accounts presents its emoji without a lookup per
// emoji. A remote emoji is shown through the media proxy; one whose domain or image host is under
// a reject_media (the media_fetch restriction, FR-702) is left out, as is every remote emoji when
// no proxy is configured, and so is an emoji of this instance until local emoji can be
// registered. An emoji that is left out stays its :shortcode: text in the clients.
//
// The proxy serves remote files as they are, so URL and StaticURL are the same proxy URL of the
// image; they part when the proxy serves still variants (ADR-0054 §7).
func (s *MediaService) CustomEmojis(ctx context.Context, ids []string) (map[string]CustomEmojiView, error) {
	out := map[string]CustomEmojiView{}
	ids = sortedSet(slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return id == "" }))
	if len(ids) == 0 || s.d.Proxy == nil {
		return out, nil
	}
	emojis, err := s.d.Repository.ListCurrentCustomEmojis(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("content: the custom emoji %v: %w", ids, err)
	}
	mediaIDs := make([]string, 0, len(emojis))
	for _, e := range emojis {
		if e.Emoji.Domain != "" {
			mediaIDs = append(mediaIDs, e.Image.MediaID)
		}
	}
	remote, err := s.d.Repository.ListRemoteMedia(ctx, sortedSet(mediaIDs))
	if err != nil {
		return nil, fmt.Errorf("content: the images of the custom emoji %v: %w", ids, err)
	}
	byID := make(map[string]content.RemoteMedia, len(remote))
	for _, m := range remote {
		byID[m.MediaID] = m
	}
	seen := map[string]bool{}
	for _, e := range emojis {
		m, ok := byID[e.Image.MediaID]
		if e.Emoji.Domain == "" || !ok {
			continue
		}
		u, err := url.Parse(m.RemoteURL)
		if err != nil || u.Hostname() == "" {
			continue
		}
		rejected, err := s.d.Proxy.rejects(ctx, seen, e.Emoji.Domain, u.Hostname())
		if err != nil {
			return nil, err
		}
		if rejected != "" {
			continue
		}
		proxied := s.d.Proxy.URL(m.MediaID, content.VariantFull)
		out[e.Emoji.ID] = CustomEmojiView{ID: e.Emoji.ID, Shortcode: e.Emoji.Shortcode, URL: proxied, StaticURL: proxied}
	}
	return out, nil
}
