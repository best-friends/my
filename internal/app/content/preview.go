// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/domain/content"
)

// This file holds the use cases behind link previews (ADR-0028): what an anonymous visitor may
// see of a local profile and of a local status, for the meta tags of their HTML pages. Every
// decision about a status goes through content.CanView with an anonymous viewer; anything an
// anonymous visitor may not see fails with ErrNotFound, whatever the reason, so that the pages
// do not reveal it.

// PreviewDeps are the ports Previews uses.
type PreviewDeps struct {
	Repository   Repository
	Accounts     Accounts
	Restrictions RestrictionPolicy
	// Media reads the attached images and the profile images; nil previews no images.
	Media *MediaService
	// DefaultLanguage returns the default post language of a user (FR-110), the language of
	// a profile and of a status whose language is undetermined; nil leaves them undetermined.
	DefaultLanguage func(ctx context.Context, userID string) (string, error)
}

// Previews reads profiles and statuses as an anonymous visitor sees them.
type Previews struct {
	d PreviewDeps
}

// NewPreviews returns Previews on d.
func NewPreviews(d PreviewDeps) *Previews {
	if d.Restrictions == nil {
		d.Restrictions = NoRestrictions{}
	}
	return &Previews{d: d}
}

// PreviewImage is a processed image with what a preview needs of it. Alt is "" when the image
// has no alt text (profile images never have one).
type PreviewImage struct {
	MediaID string
	// ResultID identifies the processing result, so that a reprocessed image is a new version.
	ResultID      string
	URL           string
	ContentType   string
	Width, Height int
	Alt           string
}

// ProfilePreview is a local profile that an anonymous visitor may see.
type ProfilePreview struct {
	Actor appaccount.Actor
	// Language is the author's default post language, "" when undetermined.
	Language string
	// Header and Avatar are the profile images, nil when unset or not processed.
	Header, Avatar *PreviewImage
}

// Username returns the local username.
func (p ProfilePreview) Username() string { return p.Actor.Local.Username }

// StatusPreview is a local status that an anonymous visitor may see.
type StatusPreview struct {
	Post   Post
	Author ProfilePreview
	// Visibility is the effective visibility (public or unlisted).
	Visibility content.Visibility
	// Language is the status's language, or the author's default post language when it is
	// undetermined; "" when both are.
	Language string
	// Image is the first attached image with its alt text, nil when there is none or the
	// status is sensitive or behind a content warning.
	Image *PreviewImage
}

// Profile returns the profile of a local username as an anonymous visitor sees it. A username
// that names no local, active account with a profile page, or an account whose profile is
// restricted, fails with ErrNotFound.
func (p *Previews) Profile(ctx context.Context, username string) (ProfilePreview, error) {
	// "" is account.LocalDomain, the domain of local handles.
	a, err := p.d.Accounts.FindByHandle(ctx, username, "")
	if errors.Is(err, appaccount.ErrNotFound) {
		return ProfilePreview{}, fmt.Errorf("profile %s: %w", username, ErrNotFound)
	}
	if err != nil {
		return ProfilePreview{}, err
	}
	return p.profile(ctx, a)
}

func (p *Previews) profile(ctx context.Context, a appaccount.Actor) (ProfilePreview, error) {
	hidden := fmt.Errorf("account %s: %w", a.Account.ID, ErrNotFound)
	if !a.Account.IsLocal() || a.Local == nil || !a.Lifecycle.Active() {
		return ProfilePreview{}, hidden
	}
	r, err := p.d.Restrictions.Restrictions(ctx, a.Account.ID)
	if err != nil {
		return ProfilePreview{}, err
	}
	if r.Author.ProfileView {
		return ProfilePreview{}, hidden
	}
	out := ProfilePreview{Actor: a}
	if out.Language, err = p.defaultLanguage(ctx, a.Local.UserID); err != nil {
		return ProfilePreview{}, err
	}
	if prof := a.Profile; prof != nil {
		if out.Header, err = p.Image(ctx, prof.HeaderMediaID, ""); err != nil {
			return ProfilePreview{}, err
		}
		if out.Avatar, err = p.Image(ctx, prof.AvatarMediaID, ""); err != nil {
			return ProfilePreview{}, err
		}
	}
	return out, nil
}

// Status returns a local status as an anonymous visitor sees it. A status that does not exist,
// is deleted, is remote, or that content.CanView hides from an anonymous visitor (local,
// private, and direct statuses, and the statuses of restricted or inactive authors) fails with
// ErrNotFound.
func (p *Previews) Status(ctx context.Context, statusID string) (StatusPreview, error) {
	hidden := fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	st, err := p.d.Repository.FindStatus(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return StatusPreview{}, hidden
	}
	if err != nil {
		return StatusPreview{}, err
	}
	if !st.IsLocal {
		return StatusPreview{}, hidden
	}
	switch _, err := p.d.Repository.FindDeletion(ctx, statusID); {
	case err == nil:
		return StatusPreview{}, hidden
	case !errors.Is(err, ErrNotFound):
		return StatusPreview{}, err
	}
	rev, err := p.d.Repository.FindCurrentRevision(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return StatusPreview{}, hidden
	}
	if err != nil {
		return StatusPreview{}, err
	}

	r, err := p.d.Restrictions.Restrictions(ctx, st.AuthorAccountID)
	if err != nil {
		return StatusPreview{}, err
	}
	restrictions := r.Author
	author, err := p.d.Accounts.FindByID(ctx, st.AuthorAccountID)
	switch {
	case errors.Is(err, appaccount.ErrNotFound):
		restrictions.ProfileView = true
	case err != nil:
		return StatusPreview{}, err
	default:
		// A suspended or deleted author is shown to no one else (data-model §5.1).
		restrictions.ProfileView = restrictions.ProfileView || !author.Lifecycle.Active() || author.Local == nil
	}
	facts := content.StatusFacts{AuthorAccountID: st.AuthorAccountID, AuthorIsLocal: st.IsLocal, Visibility: st.Visibility}
	if !content.CanView(content.ContextStatus, content.Viewer{Kind: content.ViewerAnonymous}, facts, restrictions, content.Relation{}) {
		return StatusPreview{}, hidden
	}
	prof, err := p.profile(ctx, author)
	if errors.Is(err, ErrNotFound) {
		return StatusPreview{}, hidden
	}
	if err != nil {
		return StatusPreview{}, err
	}

	out := StatusPreview{
		Post: Post{Status: st, Revision: rev}, Author: prof,
		Visibility: content.EffectiveVisibility(st.Visibility, restrictions), Language: rev.Language,
	}
	if out.Language == "" || out.Language == UndeterminedLanguage {
		out.Language = prof.Language
	}
	// Moderation may show the attachments as sensitive (data-model §5.2): the preview shows none.
	sensitive := rev.Sensitive || r.MediaSensitive
	if !sensitive && len(rev.Attachments) > 0 {
		if sensitive, err = p.d.Repository.SensitivityOverridden(ctx, st.ID); err != nil {
			return StatusPreview{}, err
		}
	}
	if !sensitive && rev.SpoilerText == "" {
		for _, at := range rev.Attachments {
			img, err := p.Image(ctx, at.MediaID, at.Description)
			if err != nil {
				return StatusPreview{}, err
			}
			if img != nil {
				out.Image = img
				break
			}
		}
	}
	return out, nil
}

// defaultLanguage returns a user's default post language, "" when undetermined.
func (p *Previews) defaultLanguage(ctx context.Context, userID string) (string, error) {
	if p.d.DefaultLanguage == nil || userID == "" {
		return "", nil
	}
	lang, err := p.d.DefaultLanguage(ctx, userID)
	if err != nil {
		return "", err
	}
	if lang == UndeterminedLanguage {
		return "", nil
	}
	return lang, nil
}

// Image returns the large still variant of a processed image with alt text alt, nil for no
// media, a file that is missing, not processed, or not an image. Besides the images of profiles
// and statuses, it reads the instance's preview image.
func (p *Previews) Image(ctx context.Context, mediaID, alt string) (*PreviewImage, error) {
	if mediaID == "" || p.d.Media == nil {
		return nil, nil
	}
	m, err := p.d.Media.Media(ctx, mediaID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !m.Ready() || m.Result.MediaType != content.MediaTypeImage {
		return nil, nil
	}
	v, ok := m.Result.Variant(content.SizeLarge, content.MotionStill)
	if !ok {
		return nil, nil
	}
	return &PreviewImage{
		MediaID: mediaID, ResultID: m.Result.ID, URL: p.d.Media.URL(v), ContentType: v.ContentType,
		Width: v.Width, Height: v.Height, Alt: alt,
	}, nil
}
