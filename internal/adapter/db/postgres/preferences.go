// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// InsertPreferences implements identity.PreferencesRepository.
func (r *IdentityRepository) InsertPreferences(ctx context.Context, tx uow.Tx, p identity.Preferences) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertUserPreferences(ctx, postgresgen.InsertUserPreferencesParams{
		UserID:                  p.UserID,
		Locale:                  p.Locale,
		TimeZone:                p.TimeZone,
		DefaultVisibility:       string(p.DefaultVisibility),
		DefaultPostLanguage:     p.DefaultPostLanguage,
		RequireMediaDescription: p.RequireMediaDescription,
		UpdatedAt:               p.UpdatedAt,
	})
	return wrap(err, "insert preferences")
}

// UpdatePreferences implements identity.PreferencesRepository.
func (r *IdentityRepository) UpdatePreferences(ctx context.Context, tx uow.Tx, p identity.Preferences) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	n, err := q.UpdateUserPreferences(ctx, postgresgen.UpdateUserPreferencesParams{
		UserID:                  p.UserID,
		Locale:                  p.Locale,
		TimeZone:                p.TimeZone,
		DefaultVisibility:       string(p.DefaultVisibility),
		DefaultPostLanguage:     p.DefaultPostLanguage,
		RequireMediaDescription: p.RequireMediaDescription,
		UpdatedAt:               p.UpdatedAt,
	})
	if err != nil {
		return wrap(err, "update preferences")
	}
	if n == 0 {
		return fmt.Errorf("preferences of %s: %w", p.UserID, appidentity.ErrNotFound)
	}
	return nil
}

// ReplaceTimelineLanguages implements identity.PreferencesRepository.
func (r *IdentityRepository) ReplaceTimelineLanguages(ctx context.Context, tx uow.Tx, userID string, languages []string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	if err := q.DeleteUserTimelineLanguages(ctx, userID); err != nil {
		return wrap(err, "delete timeline languages")
	}
	for _, l := range languages {
		err := q.InsertUserTimelineLanguage(ctx, postgresgen.InsertUserTimelineLanguageParams{UserID: userID, Language: l})
		if err != nil {
			return wrap(err, "insert timeline language")
		}
	}
	return nil
}

// FindPreferences implements identity.PreferencesRepository.
func (r *IdentityRepository) FindPreferences(ctx context.Context, userID string) (identity.Preferences, error) {
	row, err := r.db.reader().GetUserPreferences(ctx, userID)
	if err != nil {
		return identity.Preferences{}, notFound(err, appidentity.ErrNotFound, "preferences of "+userID)
	}
	return identity.Preferences{
		UserID:                  row.UserID,
		Locale:                  row.Locale,
		TimeZone:                row.TimeZone,
		DefaultVisibility:       identity.Visibility(row.DefaultVisibility),
		DefaultPostLanguage:     row.DefaultPostLanguage,
		RequireMediaDescription: row.RequireMediaDescription,
		UpdatedAt:               utc(row.UpdatedAt),
	}, nil
}

// ListTimelineLanguages implements identity.PreferencesRepository.
func (r *IdentityRepository) ListTimelineLanguages(ctx context.Context, userID string) ([]string, error) {
	langs, err := r.db.reader().ListUserTimelineLanguages(ctx, userID)
	return langs, wrap(err, "list timeline languages")
}
