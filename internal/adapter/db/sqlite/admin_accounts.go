// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"
	"time"

	"github.com/mycroblog/my/internal/adapter/db/sqlite/sqlitegen"
	appaccount "github.com/mycroblog/my/internal/app/account"
	appidentity "github.com/mycroblog/my/internal/app/identity"
	appmoderation "github.com/mycroblog/my/internal/app/moderation"
	"github.com/mycroblog/my/internal/domain/account"
	"github.com/mycroblog/my/internal/domain/identity"
	"github.com/mycroblog/my/internal/domain/moderation"
)

// The reads of the moderators' account management (FR-714) in the account, identity, and
// moderation repositories.

var (
	_ appaccount.SearchRepository          = (*AccountRepository)(nil)
	_ appidentity.AccountAdminRepository   = (*IdentityRepository)(nil)
	_ appmoderation.AccountAdminRepository = (*ModerationRepository)(nil)
)

// SearchAccountIDs implements account.SearchRepository.
func (r *AccountRepository) SearchAccountIDs(ctx context.Context, q appaccount.SearchQuery, limit int) ([]string, error) {
	ids, err := r.db.reader().SearchAccountIDs(ctx, sqlitegen.SearchAccountIDsParams{
		Domain: q.Domain, Origin: q.Origin, UsernamePrefix: q.UsernamePrefix, Before: q.Before, Lim: int64(limit),
	})
	return ids, wrap(err, "search accounts")
}

// ListAccountIDsInLifecycleState implements account.SearchRepository.
func (r *AccountRepository) ListAccountIDsInLifecycleState(ctx context.Context, state account.LifecycleStateName, before string, limit int) ([]string, error) {
	ids, err := r.db.reader().ListAccountIDsInLifecycleState(ctx, sqlitegen.ListAccountIDsInLifecycleStateParams{
		State: string(state), Before: before, Lim: int64(limit),
	})
	return ids, wrap(err, "list accounts in a lifecycle state")
}

// ListUserIDsByEmailDomain implements identity.AccountAdminRepository.
func (r *IdentityRepository) ListUserIDsByEmailDomain(ctx context.Context, domain, before string, limit int) ([]string, error) {
	ids, err := r.db.reader().ListUserIDsByEmailDomain(ctx, sqlitegen.ListUserIDsByEmailDomainParams{
		Suffix: "@" + domain, Before: before, Lim: int64(limit),
	})
	return ids, wrap(err, "list users by email domain")
}

// ListUserIDsByIP implements identity.AccountAdminRepository.
func (r *IdentityRepository) ListUserIDsByIP(ctx context.Context, ip, before string, limit int) ([]string, error) {
	ids, err := r.db.reader().ListUserIDsByIP(ctx, sqlitegen.ListUserIDsByIPParams{Ip: ip, Before: before, Lim: int64(limit)})
	return ids, wrap(err, "list users by IP address")
}

// ListUserIDsInRegistrationState implements identity.AccountAdminRepository.
func (r *IdentityRepository) ListUserIDsInRegistrationState(ctx context.Context, state identity.RegistrationState, before string, limit int) ([]string, error) {
	ids, err := r.db.reader().ListUserIDsInRegistrationState(ctx, sqlitegen.ListUserIDsInRegistrationStateParams{
		State: string(state), Before: before, Lim: int64(limit),
	})
	return ids, wrap(err, "list users in a registration state")
}

// ListSessionOriginsOfUser implements identity.AccountAdminRepository.
func (r *IdentityRepository) ListSessionOriginsOfUser(ctx context.Context, userID string, limit int) ([]identity.SessionOrigin, error) {
	rows, err := r.db.reader().ListSessionOriginsOfUser(ctx, sqlitegen.ListSessionOriginsOfUserParams{UserID: userID, Lim: int64(limit)})
	if err != nil {
		return nil, wrap(err, "list session origins of a user")
	}
	out := make([]identity.SessionOrigin, len(rows))
	for i, row := range rows {
		out[i] = identity.SessionOrigin{
			SessionID: row.SessionID, UserID: row.UserID, IP: row.Ip.String, UserAgent: row.UserAgent.String,
			CreatedAt: fromMillis(row.CreatedAt),
		}
	}
	return out, nil
}

// ListRestrictedAccountIDs implements moderation.AccountAdminRepository.
func (r *ModerationRepository) ListRestrictedAccountIDs(ctx context.Context, restriction moderation.Restriction, now time.Time, before string, limit int) ([]string, error) {
	ids, err := r.db.reader().ListRestrictedAccountIDs(ctx, sqlitegen.ListRestrictedAccountIDsParams{
		Restriction: string(restriction), Now: toMillis(now), Before: before, Lim: int64(limit),
	})
	return ids, wrap(err, "list restricted accounts")
}
