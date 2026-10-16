// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/content"
)

// StatusViews reads statuses as the client API shows them (appcontent.Views).
type StatusViews interface {
	// Statuses returns the statuses of ids that are not deleted, in order, with what
	// viewerAccountID ("" for none) has done with them. It does not decide who may see them.
	Statuses(ctx context.Context, viewerAccountID string, ids []string) ([]appcontent.StatusView, error)
	// Boosts returns the boosts of ids, in order, leaving out missing ones.
	Boosts(ctx context.Context, ids []string) ([]content.Boost, error)
	// Tag returns a hashtag by name, failing with appcontent.ErrNotFound when no status used it.
	Tag(ctx context.Context, name string) (content.Tag, error)
	// Pinned returns the IDs of the statuses that accountID pins and viewer may see on its
	// profile, newest pin first.
	Pinned(ctx context.Context, viewer appcontent.Viewer, accountID string) ([]string, error)
}

// Media reads media files (appcontent.MediaService).
type Media interface {
	// Media returns a media file with its current state, failing with appcontent.ErrNotFound
	// when there is none.
	Media(ctx context.Context, id string) (appcontent.Media, error)
	// URL returns the public URL of a variant.
	URL(v content.MediaVariant) string
}

// statusItem is one item of a page of statuses: a post (BoostID "") or a boost of StatusID.
type statusItem struct {
	StatusID string
	BoostID  string
}

// statusRenderer presents the statuses of one response, reading each account once.
type statusRenderer struct {
	s        *Server
	viewer   string
	accounts map[string]*Account
}

func (s *Server) newStatusRenderer(viewerAccountID string) *statusRenderer {
	return &statusRenderer{s: s, viewer: viewerAccountID, accounts: map[string]*Account{}}
}

// account returns the Account of an ID, nil when there is no such account.
func (r *statusRenderer) account(ctx context.Context, id string) (*Account, error) {
	if a, ok := r.accounts[id]; ok {
		return a, nil
	}
	actor, err := r.s.o.Accounts.FindByID(ctx, id)
	var out *Account
	switch {
	case err == nil:
		a := r.s.newAccount(actor)
		out = &a
	case !errors.Is(err, appaccount.ErrNotFound):
		return nil, err
	}
	r.accounts[id] = out
	return out, nil
}

// statuses presents items in order. An item whose status, boost, or account is gone (deleted
// after the page was read) is left out.
func (r *statusRenderer) statuses(ctx context.Context, items []statusItem) ([]Status, error) {
	var statusIDs, boostIDs []string
	for _, it := range items {
		statusIDs = append(statusIDs, it.StatusID)
		if it.BoostID != "" {
			boostIDs = append(boostIDs, it.BoostID)
		}
	}
	views, err := r.s.o.StatusViews.Statuses(ctx, r.viewer, statusIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]appcontent.StatusView, len(views))
	for _, v := range views {
		byID[v.Status.ID] = v
	}
	boosts := map[string]content.Boost{}
	if len(boostIDs) > 0 {
		bs, err := r.s.o.StatusViews.Boosts(ctx, boostIDs)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			boosts[b.ID] = b
		}
	}
	out := make([]Status, 0, len(items))
	for _, it := range items {
		v, ok := byID[it.StatusID]
		if !ok {
			continue
		}
		st, ok, err := r.status(ctx, v)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if it.BoostID == "" {
			out = append(out, st)
			continue
		}
		b, ok := boosts[it.BoostID]
		if !ok {
			continue
		}
		booster, err := r.account(ctx, b.BoosterAccountID)
		if err != nil {
			return nil, err
		}
		if booster == nil {
			continue
		}
		out = append(out, Status{
			Id: b.ID, Uri: b.URI, CreatedAt: b.BoostedAt, Account: *booster,
			Visibility: st.Visibility, Language: st.Language, Boost: &st,
			MediaAttachments: []MediaAttachment{}, Mentions: []StatusMention{}, Tags: []StatusTag{},
		})
	}
	return out, nil
}

// status presents a post; ok is false when its author is gone.
func (r *statusRenderer) status(ctx context.Context, v appcontent.StatusView) (Status, bool, error) {
	st, rev := v.Status, v.Revision
	author, err := r.account(ctx, st.AuthorAccountID)
	if err != nil || author == nil {
		return Status{}, false, err
	}
	out := Status{
		Id: st.ID, Uri: st.URI, CreatedAt: st.PublishedAt, Account: *author,
		Visibility: Visibility(st.Visibility), Language: rev.Language, Content: body(rev),
		SpoilerText: rev.SpoilerText, Sensitive: rev.Sensitive,
		MediaAttachments: make([]MediaAttachment, 0, len(rev.Attachments)),
		Mentions:         make([]StatusMention, 0, len(rev.MentionedAccountIDs)),
		Tags:             make([]StatusTag, 0, len(v.Tags)),
		RepliesCount:     int(v.Counters.RepliesCount), BoostsCount: int(v.Counters.BoostsCount),
		FavouritesCount: int(v.Counters.FavouritesCount),
	}
	if st.URL != "" {
		out.Url = &st.URL
	}
	if rev.Seq > 1 {
		out.EditedAt = &rev.RevisedAt
	}
	if out.Language == "" {
		out.Language = appcontent.UndeterminedLanguage
	}
	if l := v.InReplyTo; l != nil {
		out.InReplyToId, out.InReplyToAccountId = &l.ParentStatusID, &l.ParentAuthorAccountID
	}
	for _, at := range rev.Attachments {
		m, err := r.s.mediaAttachment(ctx, at)
		if err != nil {
			return Status{}, false, err
		}
		out.MediaAttachments = append(out.MediaAttachments, m)
	}
	for _, id := range rev.MentionedAccountIDs {
		a, err := r.account(ctx, id)
		if err != nil {
			return Status{}, false, err
		}
		if a != nil {
			out.Mentions = append(out.Mentions, StatusMention{Id: a.Id, Username: a.Username, Acct: a.Acct, Url: a.Url})
		}
	}
	for _, t := range v.Tags {
		out.Tags = append(out.Tags, StatusTag{Name: t.Name, Url: appcontent.TagURL(r.s.o.Host, t.NameNormalized)})
	}
	if vs := v.Viewer; vs != nil {
		out.Favourited, out.Boosted, out.Bookmarked, out.Pinned = &vs.Favourited, &vs.Boosted, &vs.Bookmarked, &vs.Pinned
	}
	return out, true, nil
}

// body returns the content of a revision in its language, or its first body.
func body(rev content.Revision) string {
	for _, b := range rev.Bodies {
		if b.Language == rev.Language {
			return b.ContentHTML
		}
	}
	if len(rev.Bodies) > 0 {
		return rev.Bodies[0].ContentHTML
	}
	return ""
}

// mediaAttachment presents an attachment; a file that is missing or not processed is unknown
// and has no URL.
func (s *Server) mediaAttachment(ctx context.Context, at content.Attachment) (MediaAttachment, error) {
	out := MediaAttachment{Id: at.MediaID, Type: MediaAttachmentTypeUnknown}
	if at.Description != "" {
		out.Description = &at.Description
	}
	if s.o.Media == nil {
		return out, nil
	}
	m, err := s.o.Media.Media(ctx, at.MediaID)
	if errors.Is(err, appcontent.ErrNotFound) {
		s.o.Logger.WarnContext(ctx, "attachment without a media file", slog.String("media_id", at.MediaID))
		return out, nil
	}
	if err != nil {
		return MediaAttachment{}, fmt.Errorf("clientapi: media %s: %w", at.MediaID, err)
	}
	if !m.Ready() {
		return out, nil
	}
	res := m.Result
	out.Type = MediaAttachmentType(res.MediaType)
	if v, ok := res.Variant(content.VariantFull); ok {
		u := s.o.Media.URL(v)
		out.Url = &u
	}
	if v, ok := res.Variant(content.VariantThumbnail); ok {
		u := s.o.Media.URL(v)
		out.PreviewUrl = &u
	}
	if res.Blurhash != "" {
		out.Blurhash = &res.Blurhash
	}
	if res.Width > 0 && res.Height > 0 {
		out.Width, out.Height = &res.Width, &res.Height
	}
	if f := m.Focus; f != nil && (f.X != 0 || f.Y != 0) {
		out.Focus = &MediaFocus{X: float32(f.X), Y: float32(f.Y)}
	}
	return out, nil
}
