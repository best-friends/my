// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"context"
	"errors"
	"fmt"
	"html"

	"github.com/best-friends/my/internal/domain/account"
)

// ErrNotLocal reports a use case for local accounts called with a remote account or the
// instance actor.
var ErrNotLocal = errors.New("account: not a local account")

// ProfileUpdate is a change of a local profile (FR-006). Nil fields keep their current value.
type ProfileUpdate struct {
	DisplayName *string
	// Note is the profile text as typed; the server renders it to HTML.
	Note *string
	// Fields replace the metadata fields; their values are rendered to HTML.
	Fields *[]account.FieldSource
	// AvatarMediaID and HeaderMediaID name processed images that the account uploaded; ""
	// removes the image.
	AvatarMediaID *string
	HeaderMediaID *string
	Locked        *bool
	Bot           *bool
	Discoverable  *bool
	Indexable     *bool
}

// UpdateProfile records a new revision of a local account's profile: the current revision
// with the changes of in. The typed note and field values are kept as the revision's source
// and rendered to HTML like the text of a post. It fails with an error wrapping
// account.ErrInvalidProfile (a *account.ProfileError) for invalid content or an image that
// the account cannot use, with ErrNotLocal for an account that is not a local user's, and
// with ErrNotFound for an unknown account.
func (s *Service) UpdateProfile(ctx context.Context, accountID string, in ProfileUpdate) (Actor, error) {
	a, err := s.FindByID(ctx, accountID)
	if err != nil {
		return Actor{}, err
	}
	if a.Local == nil {
		return Actor{}, fmt.Errorf("account %s: %w", accountID, ErrNotLocal)
	}
	cur := account.ProfileRevision{}
	if a.Profile != nil {
		cur = *a.Profile
	}
	src := account.ProfileSource{FieldValues: make([]string, len(cur.Fields))}
	if cur.Source != nil {
		src.Note = cur.Source.Note
		copy(src.FieldValues, cur.Source.FieldValues)
	}

	text := account.ProfileText{DisplayName: cur.DisplayName, Note: src.Note}
	for i, f := range cur.Fields {
		text.Fields = append(text.Fields, account.FieldSource{Name: f.Name, Value: src.FieldValues[i]})
	}
	if in.DisplayName != nil {
		text.DisplayName = *in.DisplayName
	}
	if in.Note != nil {
		text.Note = *in.Note
	}
	if in.Fields != nil {
		text.Fields = *in.Fields
	}
	if err := account.ValidateProfileText(text); err != nil {
		return Actor{}, err
	}

	next := ProfileInput{
		DisplayName: text.DisplayName, NoteHTML: cur.NoteHTML,
		Locked: cur.Locked, Bot: cur.Bot, Discoverable: cur.Discoverable, Indexable: cur.Indexable,
		AvatarMediaID: cur.AvatarMediaID, HeaderMediaID: cur.HeaderMediaID,
		Fields: cur.Fields, Source: &account.ProfileSource{Note: text.Note, FieldValues: src.FieldValues},
	}
	if in.Note != nil {
		if next.NoteHTML, err = s.renderProfileText(ctx, text.Note); err != nil {
			return Actor{}, err
		}
	}
	if in.Fields != nil {
		next.Fields, next.Source.FieldValues = nil, nil
		for _, f := range text.Fields {
			value, err := s.renderProfileText(ctx, f.Value)
			if err != nil {
				return Actor{}, err
			}
			next.Fields = append(next.Fields, account.ProfileField{Name: f.Name, ValueHTML: value})
			next.Source.FieldValues = append(next.Source.FieldValues, f.Value)
		}
	}
	if next.AvatarMediaID, err = s.profileImage(ctx, accountID, account.PropertyAvatar, cur.AvatarMediaID, in.AvatarMediaID); err != nil {
		return Actor{}, err
	}
	if next.HeaderMediaID, err = s.profileImage(ctx, accountID, account.PropertyHeader, cur.HeaderMediaID, in.HeaderMediaID); err != nil {
		return Actor{}, err
	}
	for _, b := range []struct {
		dst *bool
		src *bool
	}{{&next.Locked, in.Locked}, {&next.Bot, in.Bot}, {&next.Discoverable, in.Discoverable}, {&next.Indexable, in.Indexable}} {
		if b.src != nil {
			*b.dst = *b.src
		}
	}

	rev, err := s.ReviseProfile(ctx, accountID, next)
	if err != nil {
		return Actor{}, err
	}
	a.Profile = &rev
	return a, nil
}

func (s *Service) renderProfileText(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}
	if s.d.RenderProfileText == nil {
		return "<p>" + html.EscapeString(text) + "</p>", nil
	}
	out, err := s.d.RenderProfileText(ctx, text)
	if err != nil {
		return "", fmt.Errorf("account: render profile text: %w", err)
	}
	return out, nil
}

// profileImage returns the image of a new revision: the current one unless requested is set,
// and then requested after checking it, or "" to remove the image.
func (s *Service) profileImage(ctx context.Context, accountID string, prop account.ProfileProperty, current string, requested *string) (string, error) {
	if requested == nil {
		return current, nil
	}
	if *requested == "" || *requested == current {
		return *requested, nil
	}
	err := ErrUnusableImage
	if s.d.CheckProfileImage != nil {
		err = s.d.CheckProfileImage(ctx, accountID, *requested, prop)
	}
	if errors.Is(err, ErrUnusableImage) {
		return "", &account.ProfileError{Property: prop, Problem: account.ProblemInvalid}
	}
	if err != nil {
		return "", fmt.Errorf("account: check %s %s: %w", prop, *requested, err)
	}
	return *requested, nil
}
