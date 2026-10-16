// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
)

// Profiles changes the viewer's profile. *appaccount.Service implements it.
type Profiles interface {
	// UpdateProfile records a new profile revision; it fails with an error wrapping
	// account.ErrInvalidProfile for invalid content.
	UpdateProfile(ctx context.Context, accountID string, in appaccount.ProfileUpdate) (appaccount.Actor, error)
}

// MediaFiles reads media files for presenting them. *appcontent.MediaService implements it.
type MediaFiles interface {
	Media(ctx context.Context, id string) (appcontent.Media, error)
	URL(v content.MediaVariant) string
}

// newCredentialAccount presents the viewer's own actor as the CredentialAccount schema: the
// account with its follow counts (newProfile), images, and the source of its profile.
func (s *Server) newCredentialAccount(ctx context.Context, a appaccount.Actor) (CredentialAccount, error) {
	base, err := s.newProfile(ctx, a)
	if err != nil {
		return CredentialAccount{}, err
	}
	out := CredentialAccount{
		Id: base.Id, Username: base.Username, Acct: base.Acct, DisplayName: base.DisplayName, Note: base.Note,
		Url: base.Url, Locked: base.Locked, Bot: base.Bot, CreatedAt: base.CreatedAt, Fields: base.Fields,
		FollowersCount: base.FollowersCount, FollowingCount: base.FollowingCount,
		Source: AccountSource{Fields: []AccountFieldSource{}},
	}
	p := a.Profile
	if p == nil {
		return out, nil
	}
	out.Discoverable, out.Indexable = p.Discoverable, p.Indexable
	for i, f := range p.Fields {
		src := AccountFieldSource{Name: f.Name}
		if p.Source != nil && i < len(p.Source.FieldValues) {
			src.Value = p.Source.FieldValues[i]
		}
		out.Source.Fields = append(out.Source.Fields, src)
	}
	if p.Source != nil {
		out.Source.Note = p.Source.Note
	}
	if p.AvatarMediaID != "" {
		out.Source.AvatarMediaId = &p.AvatarMediaID
		if out.Avatar, err = s.imageURL(ctx, p.AvatarMediaID); err != nil {
			return CredentialAccount{}, err
		}
	}
	if p.HeaderMediaID != "" {
		out.Source.HeaderMediaId = &p.HeaderMediaID
		if out.Header, err = s.imageURL(ctx, p.HeaderMediaID); err != nil {
			return CredentialAccount{}, err
		}
	}
	return out, nil
}

// imageURL returns the URL of the full variant of a processed image, or nil when there is
// none to show.
func (s *Server) imageURL(ctx context.Context, mediaID string) (*string, error) {
	if s.o.Media == nil {
		return nil, nil
	}
	m, err := s.o.Media.Media(ctx, mediaID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: media %s: %w", mediaID, err)
	}
	if !m.Ready() {
		return nil, nil
	}
	v, ok := m.Result.Variant(content.VariantFull)
	if !ok {
		return nil, nil
	}
	u := s.o.Media.URL(v)
	return &u, nil
}

// UpdateAccountCredentials serves PATCH /api/v1/accounts/update_credentials: a new revision of
// the viewer's profile (FR-006).
func (s *Server) UpdateAccountCredentials(ctx context.Context, req UpdateAccountCredentialsRequestObject) (UpdateAccountCredentialsResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	in := appaccount.ProfileUpdate{}
	if b := req.Body; b != nil {
		in = appaccount.ProfileUpdate{
			DisplayName: b.DisplayName, Note: b.Note, AvatarMediaID: b.AvatarMediaId, HeaderMediaID: b.HeaderMediaId,
			Locked: b.Locked, Bot: b.Bot, Discoverable: b.Discoverable, Indexable: b.Indexable,
		}
		if b.Fields != nil {
			fields := make([]account.FieldSource, len(*b.Fields))
			for i, f := range *b.Fields {
				fields[i] = account.FieldSource{Name: f.Name, Value: f.Value}
			}
			in.Fields = &fields
		}
	}
	a, err := s.o.Profiles.UpdateProfile(ctx, v.AccountID, in)
	var invalid *account.ProfileError
	if errors.As(err, &invalid) {
		body, headers := s.NewProblem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank,
			s.Localizer(ctx).ServerErrorsUnprocessable(), s.profileProblemDetail(ctx, invalid))
		return UpdateAccountCredentialsdefaultApplicationProblemPlusJSONResponse{
			Body: body, Headers: headers, StatusCode: http.StatusUnprocessableEntity,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: update the profile of %s: %w", v.AccountID, err)
	}
	body, err := s.newCredentialAccount(ctx, a)
	if err != nil {
		return nil, err
	}
	cache := cacheNoStore
	return UpdateAccountCredentials200JSONResponse{
		Body:    body,
		Headers: UpdateAccountCredentials200ResponseHeaders{CacheControl: &cache},
	}, nil
}

// profileProperties are the request properties of the parts of a profile.
var profileProperties = map[account.ProfileProperty]string{
	account.PropertyDisplayName: "display_name",
	account.PropertyNote:        "note",
	account.PropertyFields:      "fields",
	account.PropertyFieldName:   "fields[].name",
	account.PropertyFieldValue:  "fields[].value",
	account.PropertyAvatar:      "avatar_media_id",
	account.PropertyHeader:      "header_media_id",
}

func (s *Server) profileProblemDetail(ctx context.Context, e *account.ProfileError) string {
	l := s.Localizer(ctx)
	name := profileProperties[e.Property]
	switch {
	case e.Problem == account.ProblemTooLong:
		return l.ServerErrorsTooLong(e.Max, name)
	case e.Problem == account.ProblemTooMany:
		return l.ServerErrorsTooMany(e.Max, name)
	case e.Property == account.PropertyAvatar || e.Property == account.PropertyHeader:
		return l.ServerErrorsUnusableImage(name)
	}
	return l.ServerErrorsInvalidValue(name)
}
