// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/content"
)

// Bounds of a thread (FR-106).
const (
	// MaxThreadAncestors is the number of statuses Thread walks up from a status.
	MaxThreadAncestors = 40
	// MaxThreadDescendants is the number of replies Thread returns below a status.
	MaxThreadDescendants = 60
)

// ReaderDeps are the ports a Reader uses.
type ReaderDeps struct {
	Statuses      Repository
	Interactions  InteractionRepository
	Accounts      Accounts
	Relationships Relationships
	// Restrictions are the authors' moderation restrictions; nil means NoRestrictions.
	Restrictions RestrictionPolicy
	// Media reads the attached media; nil presents no attachments.
	Media *MediaService
}

// Reader presents statuses to a viewer: one status, its thread, its edit history, and a local
// account's bookmarks, with everything a client shows of them. Every status goes through
// content.CanView in ContextStatus; a status that the viewer may not see fails with
// ErrNotFound, the same as a missing or deleted one, and is left out of lists, so that its
// existence does not leak (docs/architecture/visibility.md).
//
// The viewer is anonymous or a local account (content.ViewerAnonymous, content.ViewerLocal).
type Reader struct {
	d   ReaderDeps
	aud audience
}

// NewReader returns a Reader on d.
func NewReader(d ReaderDeps) *Reader {
	if d.Restrictions == nil {
		d.Restrictions = NoRestrictions{}
	}
	return &Reader{d: d, aud: audience{
		statuses: d.Statuses, interactions: d.Interactions, accounts: d.Accounts,
		relationships: d.Relationships, restrictions: d.Restrictions,
	}}
}

// StatusView is a status as a viewer sees it.
type StatusView struct {
	Status   content.Status
	Revision content.Revision
	// Author is the author's account.
	Author appaccount.Actor
	// InReplyTo is the status replied to, nil when there is none or the viewer may not see it.
	InReplyTo *content.Status
	// Attachments are the attached media that finished processing, in order.
	Attachments []MediaView
	// Mentions are the mentioned accounts that exist, in the revision's order.
	Mentions []appaccount.Actor
	Tags     []content.Tag
	Counters content.Counters
	// Relation is the viewer's relation to the status, nil for an anonymous viewer.
	Relation *ViewerRelation
}

// Edited reports whether the status has a revision after its first.
func (v StatusView) Edited() bool { return v.Revision.Seq > 1 }

// ViewerRelation is what a local viewer did with a status.
type ViewerRelation struct {
	Favourited, Boosted, Bookmarked bool
	// Author is whether the viewer is the author, who alone can pin the status.
	Author bool
	// Pinned is whether the author pinned the status; always false for other viewers.
	Pinned bool
}

// MediaView is an attached media file that can be shown: an upload that finished processing, or
// a remote file (Media.Remote) served through the media proxy.
type MediaView struct {
	Media Media
	// Description is the alt text of the attachment, "" for none.
	Description string
	// URL is the full variant, PreviewURL the thumbnail (the full variant without one). For a
	// remote file both are its media proxy URL.
	URL, PreviewURL string
}

// RevisionParts are what a revision refers to, resolved: the attached media that finished
// processing, in order; the mentioned accounts that exist, in the revision's order; and the
// hashtags, in the revision's order.
type RevisionParts struct {
	Attachments []MediaView
	Mentions    []appaccount.Actor
	Tags        []content.Tag
}

// Full returns the variant that URL serves.
func (v MediaView) Full() content.MediaVariant {
	full, _ := v.Media.Result.Variant(content.VariantFull)
	return full
}

// RevisionView is one revision of a status, for its edit history.
type RevisionView struct {
	Revision    content.Revision
	Attachments []MediaView
}

// Thread is the statuses around one that a viewer may see.
type Thread struct {
	// Ancestors are the statuses replied to, oldest first, up to the first that is missing or
	// that the viewer may not see.
	Ancestors []StatusView
	// Descendants are the replies and their replies, depth first in posting order, without
	// the replies that the viewer may not see and their replies.
	Descendants []StatusView
}

// BookmarkPage is a page of bookmarked statuses.
type BookmarkPage struct {
	Statuses []StatusView
	// Next is the cursor of the next page, "" on the last page.
	Next string
}

// View returns a status as viewer sees it.
func (r *Reader) View(ctx context.Context, viewer Viewer, statusID string) (StatusView, error) {
	st, author, err := r.aud.visibleStatus(ctx, viewer, statusID)
	if err != nil {
		return StatusView{}, err
	}
	return r.present(ctx, viewer, st, author)
}

// Thread returns the ancestors and descendants of a status that viewer may see (FR-106).
func (r *Reader) Thread(ctx context.Context, viewer Viewer, statusID string) (Thread, error) {
	st, _, err := r.aud.visibleStatus(ctx, viewer, statusID)
	if err != nil {
		return Thread{}, err
	}
	var out Thread
	seen := map[string]bool{st.ID: true}
	for cur := st; len(out.Ancestors) < MaxThreadAncestors && cur.InReplyToURI != ""; {
		parent, view, ok, err := r.viewByURI(ctx, viewer, cur.InReplyToURI)
		if err != nil {
			return Thread{}, err
		}
		if !ok || seen[parent.ID] {
			break
		}
		seen[parent.ID] = true
		out.Ancestors = append([]StatusView{view}, out.Ancestors...)
		cur = parent
	}
	if err := r.descend(ctx, viewer, st, seen, &out.Descendants); err != nil {
		return Thread{}, err
	}
	return out, nil
}

// descend appends the visible replies to st, each followed by its own replies.
func (r *Reader) descend(ctx context.Context, viewer Viewer, st content.Status, seen map[string]bool, out *[]StatusView) error {
	replies, err := r.d.Statuses.ListReplies(ctx, st.URI)
	if err != nil {
		return err
	}
	for _, reply := range replies {
		if len(*out) >= MaxThreadDescendants {
			return nil
		}
		if seen[reply.ID] {
			continue
		}
		seen[reply.ID] = true
		ok, author, err := r.aud.visible(ctx, viewer, reply)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		view, err := r.present(ctx, viewer, reply, author)
		if errors.Is(err, ErrNotFound) {
			continue // deleted meanwhile
		}
		if err != nil {
			return err
		}
		*out = append(*out, view)
		if err := r.descend(ctx, viewer, reply, seen, out); err != nil {
			return err
		}
	}
	return nil
}

// viewByURI returns the live status with uri as viewer sees it; ok is false when it is
// missing, deleted, or hidden from the viewer.
func (r *Reader) viewByURI(ctx context.Context, viewer Viewer, uri string) (content.Status, StatusView, bool, error) {
	st, err := r.d.Statuses.FindStatusByURI(ctx, uri)
	if errors.Is(err, ErrNotFound) {
		return content.Status{}, StatusView{}, false, nil
	}
	if err != nil {
		return content.Status{}, StatusView{}, false, err
	}
	view, err := r.View(ctx, viewer, st.ID)
	if errors.Is(err, ErrNotFound) {
		return content.Status{}, StatusView{}, false, nil
	}
	if err != nil {
		return content.Status{}, StatusView{}, false, err
	}
	return st, view, true, nil
}

// History returns the revisions of a status that viewer may see, oldest first (FR-109).
func (r *Reader) History(ctx context.Context, viewer Viewer, statusID string) ([]RevisionView, error) {
	if _, _, err := r.aud.visibleStatus(ctx, viewer, statusID); err != nil {
		return nil, err
	}
	revs, err := r.d.Statuses.ListRevisions(ctx, statusID)
	if err != nil {
		return nil, err
	}
	out := make([]RevisionView, 0, len(revs))
	for _, head := range revs {
		rev, err := r.d.Statuses.FindRevision(ctx, head.ID)
		if err != nil {
			return nil, err
		}
		media, err := r.attachments(ctx, rev.Attachments)
		if err != nil {
			return nil, err
		}
		out = append(out, RevisionView{Revision: rev, Attachments: media})
	}
	return out, nil
}

// Source returns the current revision of a local account's own status, whose Source holds
// what the author typed. Another account's status fails with ErrNotFound, like a missing one.
func (r *Reader) Source(ctx context.Context, accountID, statusID string) (content.Revision, error) {
	view, err := r.View(ctx, Viewer{Kind: ViewerLocal, AccountID: accountID}, statusID)
	if err != nil {
		return content.Revision{}, err
	}
	if view.Status.AuthorAccountID != accountID || view.Revision.Source == nil {
		return content.Revision{}, fmt.Errorf("source of %s: %w", statusID, ErrNotFound)
	}
	return view.Revision, nil
}

// Bookmarks returns a page of a local account's bookmarked statuses, newest bookmark first
// (FR-113): up to limit bookmarks older than the cursor before ("" for the newest). Statuses
// that the account may no longer see are left out, so a page may be shorter than limit and
// still have a next page.
func (r *Reader) Bookmarks(ctx context.Context, accountID, before string, limit int) (BookmarkPage, error) {
	marks, err := r.d.Interactions.ListBookmarks(ctx, accountID, before, limit)
	if err != nil {
		return BookmarkPage{}, err
	}
	viewer := Viewer{Kind: ViewerLocal, AccountID: accountID}
	var out BookmarkPage
	for _, b := range marks {
		view, err := r.View(ctx, viewer, b.StatusID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return BookmarkPage{}, err
		}
		out.Statuses = append(out.Statuses, view)
	}
	if len(marks) == limit && limit > 0 {
		out.Next = marks[len(marks)-1].ID
	}
	return out, nil
}

// present returns st, which viewer may see, with everything a client shows of it.
func (r *Reader) present(ctx context.Context, viewer Viewer, st content.Status, author appaccount.Actor) (StatusView, error) {
	rev, err := r.d.Statuses.FindCurrentRevision(ctx, st.ID)
	if err != nil {
		return StatusView{}, err // a deleted status has no current revision
	}
	out := StatusView{Status: st, Revision: rev, Author: author, Counters: content.Counters{StatusID: st.ID}}
	if st.InReplyToURI != "" {
		parent, err := r.d.Statuses.FindStatusByURI(ctx, st.InReplyToURI)
		switch {
		case errors.Is(err, ErrNotFound):
		case err != nil:
			return StatusView{}, err
		default:
			_, _, err := r.aud.visibleStatus(ctx, viewer, parent.ID)
			switch {
			case errors.Is(err, ErrNotFound):
			case err != nil:
				return StatusView{}, err
			default:
				out.InReplyTo = &parent
			}
		}
	}
	parts, err := r.Parts(ctx, rev)
	if err != nil {
		return StatusView{}, err
	}
	out.Attachments, out.Mentions, out.Tags = parts.Attachments, parts.Mentions, parts.Tags
	switch c, err := r.d.Statuses.FindCounters(ctx, st.ID); {
	case err == nil:
		out.Counters = c
	case !errors.Is(err, ErrNotFound):
		return StatusView{}, err
	}
	if viewer.Kind == ViewerLocal && viewer.AccountID != "" {
		if out.Relation, err = r.relation(ctx, viewer.AccountID, st); err != nil {
			return StatusView{}, err
		}
	}
	return out, nil
}

// Parts resolves what rev refers to. It does not decide who may see the revision; callers
// that present it to a viewer decide that first. ActivityPub documents of local statuses are
// built from it (federation.md §7.4).
func (r *Reader) Parts(ctx context.Context, rev content.Revision) (RevisionParts, error) {
	var out RevisionParts
	var err error
	if out.Attachments, err = r.attachments(ctx, rev.Attachments); err != nil {
		return RevisionParts{}, err
	}
	for _, id := range rev.MentionedAccountIDs {
		a, err := r.d.Accounts.FindByID(ctx, id)
		if errors.Is(err, appaccount.ErrNotFound) {
			continue
		}
		if err != nil {
			return RevisionParts{}, err
		}
		out.Mentions = append(out.Mentions, a)
	}
	for _, id := range rev.TagIDs {
		tag, err := r.d.Statuses.FindTag(ctx, id)
		if err != nil {
			return RevisionParts{}, err
		}
		out.Tags = append(out.Tags, tag)
	}
	return out, nil
}

// relation returns what accountID did with st.
func (r *Reader) relation(ctx context.Context, accountID string, st content.Status) (*ViewerRelation, error) {
	repo := r.d.Interactions
	var rel ViewerRelation
	var err error
	if rel.Favourited, err = found(repo.FindFavourite(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if rel.Boosted, err = found(repo.FindBoostEdge(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if rel.Bookmarked, err = found(repo.FindBookmark(ctx, accountID, st.ID)); err != nil {
		return nil, err
	}
	if st.AuthorAccountID == accountID {
		rel.Author = true
		if rel.Pinned, err = found(repo.FindPin(ctx, accountID, st.ID)); err != nil {
			return nil, err
		}
	}
	return &rel, nil
}

// found turns a lookup into whether it found a row.
func found[T any](_ T, err error) (bool, error) {
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrNotFound):
		return false, nil
	default:
		return false, err
	}
}

// attachments returns the attached media that can be shown, in order: the uploads that finished
// processing, and the remote files when the media proxy is configured.
func (r *Reader) attachments(ctx context.Context, in []content.Attachment) ([]MediaView, error) {
	if r.d.Media == nil {
		return nil, nil
	}
	out := make([]MediaView, 0, len(in))
	for _, at := range in {
		m, err := r.d.Media.Media(ctx, at.MediaID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if m.Remote != nil {
			if u, ok := r.d.Media.ProxyURL(m.File.ID); ok {
				out = append(out, MediaView{Media: m, Description: at.Description, URL: u, PreviewURL: u})
			}
			continue
		}
		if !m.Ready() {
			continue
		}
		full, ok := m.Result.Variant(content.VariantFull)
		if !ok {
			continue
		}
		v := MediaView{Media: m, Description: at.Description, URL: r.d.Media.URL(full)}
		v.PreviewURL = v.URL
		if thumb, ok := m.Result.Variant(content.VariantThumbnail); ok {
			v.PreviewURL = r.d.Media.URL(thumb)
		}
		out = append(out, v)
	}
	return out, nil
}
