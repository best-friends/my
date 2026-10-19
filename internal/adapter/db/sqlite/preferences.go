// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
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
	err = q.InsertUserPreferences(ctx, sqlitegen.InsertUserPreferencesParams{
		UserID:                      p.UserID,
		Locale:                      p.Locale,
		DefaultVisibility:           string(p.DefaultVisibility),
		DefaultPostLanguage:         p.DefaultPostLanguage,
		RequireMediaDescription:     boolToInt(p.RequireMediaDescription),
		EmailModeratorNotifications: boolToInt(p.EmailModeratorNotifications),
		UpdatedAt:                   toMillis(p.UpdatedAt),
	})
	if err != nil {
		return wrap(err, "insert preferences")
	}
	return setTimeZone(ctx, q, p)
}

// UpdatePreferences implements identity.PreferencesRepository.
func (r *IdentityRepository) UpdatePreferences(ctx context.Context, tx uow.Tx, p identity.Preferences) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	n, err := q.UpdateUserPreferences(ctx, sqlitegen.UpdateUserPreferencesParams{
		Locale:                      p.Locale,
		DefaultVisibility:           string(p.DefaultVisibility),
		DefaultPostLanguage:         p.DefaultPostLanguage,
		RequireMediaDescription:     boolToInt(p.RequireMediaDescription),
		EmailModeratorNotifications: boolToInt(p.EmailModeratorNotifications),
		UpdatedAt:                   toMillis(p.UpdatedAt),
		UserID:                      p.UserID,
	})
	if err != nil {
		return wrap(err, "update preferences")
	}
	if n == 0 {
		return fmt.Errorf("preferences of %s: %w", p.UserID, appidentity.ErrNotFound)
	}
	return setTimeZone(ctx, q, p)
}

// setTimeZone stores the time zone of p when the user chose one (user_time_zones).
func setTimeZone(ctx context.Context, q *sqlitegen.Queries, p identity.Preferences) error {
	if !p.TimeZoneChosen {
		return nil
	}
	err := q.UpsertUserTimeZone(ctx, sqlitegen.UpsertUserTimeZoneParams{UserID: p.UserID, TimeZone: p.TimeZone})
	return wrap(err, "set time zone")
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
		err := q.InsertUserTimelineLanguage(ctx, sqlitegen.InsertUserTimelineLanguageParams{UserID: userID, Language: l})
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
	tz, chosen, err := r.timeZone(ctx, userID)
	if err != nil {
		return identity.Preferences{}, err
	}
	return identity.Preferences{
		UserID:                      row.UserID,
		TimeZone:                    tz,
		TimeZoneChosen:              chosen,
		Locale:                      row.Locale,
		DefaultVisibility:           identity.Visibility(row.DefaultVisibility),
		DefaultPostLanguage:         row.DefaultPostLanguage,
		RequireMediaDescription:     row.RequireMediaDescription != 0,
		EmailModeratorNotifications: row.EmailModeratorNotifications != 0,
		UpdatedAt:                   fromMillis(row.UpdatedAt),
	}, nil
}

// timeZone returns the time zone the user chose, or identity.DefaultTimeZone and false when they
// have chosen none.
func (r *IdentityRepository) timeZone(ctx context.Context, userID string) (string, bool, error) {
	tz, err := r.db.reader().GetUserTimeZone(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.DefaultTimeZone, false, nil
	}
	if err != nil {
		return "", false, wrap(err, "get time zone")
	}
	return tz, true, nil
}

// ListTimelineLanguages implements identity.PreferencesRepository.
func (r *IdentityRepository) ListTimelineLanguages(ctx context.Context, userID string) ([]string, error) {
	langs, err := r.db.reader().ListUserTimelineLanguages(ctx, userID)
	return langs, wrap(err, "list timeline languages")
}
