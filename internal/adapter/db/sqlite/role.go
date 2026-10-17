// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/sqlite/sqlitegen"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// Granting and revoking roles (FR-720).

// ListRoles implements identity.RoleRepository.
func (r *IdentityRepository) ListRoles(ctx context.Context) ([]appidentity.RoleWithPermissions, error) {
	q := r.db.reader()
	roles, err := q.ListRoles(ctx)
	if err != nil {
		return nil, wrap(err, "list roles")
	}
	perms, err := q.ListAllRolePermissions(ctx)
	if err != nil {
		return nil, wrap(err, "list role permissions")
	}
	granted := map[string]map[identity.Permission]bool{}
	for _, p := range perms {
		if granted[p.RoleID] == nil {
			granted[p.RoleID] = map[identity.Permission]bool{}
		}
		granted[p.RoleID][identity.Permission(p.Permission)] = true
	}
	out := make([]appidentity.RoleWithPermissions, len(roles))
	for i, row := range roles {
		out[i] = appidentity.RoleWithPermissions{Role: roleFrom(row), Permissions: []identity.Permission{}}
		for _, p := range identity.AllPermissions() {
			if granted[row.ID][p] {
				out[i].Permissions = append(out[i].Permissions, p)
			}
		}
	}
	return out, nil
}

// LockRole implements identity.RoleRepository. SQLite serializes write transactions, so it
// needs no lock.
func (r *IdentityRepository) LockRole(ctx context.Context, tx uow.Tx, key string) (identity.Role, error) {
	q, err := queries(tx)
	if err != nil {
		return identity.Role{}, err
	}
	row, err := q.GetRoleByKey(ctx, key)
	if err != nil {
		return identity.Role{}, notFound(err, appidentity.ErrNotFound, "role "+key)
	}
	return roleFrom(row), nil
}

// FindRoleGrantID implements identity.RoleRepository.
func (r *IdentityRepository) FindRoleGrantID(ctx context.Context, tx uow.Tx, userID, roleID string) (string, error) {
	q, err := queries(tx)
	if err != nil {
		return "", err
	}
	id, err := q.GetUserRoleGrantID(ctx, sqlitegen.GetUserRoleGrantIDParams{UserID: userID, RoleID: roleID})
	if err != nil {
		return "", notFound(err, appidentity.ErrNotFound, "role "+roleID+" of user "+userID)
	}
	return id, nil
}

// CountRoleHolders implements identity.RoleRepository.
func (r *IdentityRepository) CountRoleHolders(ctx context.Context, tx uow.Tx, roleID string) (int64, error) {
	q, err := queries(tx)
	if err != nil {
		return 0, err
	}
	n, err := q.CountRoleHolders(ctx, roleID)
	return n, wrap(err, "count role holders")
}

// InsertRoleRevocation implements identity.RoleRepository.
func (r *IdentityRepository) InsertRoleRevocation(ctx context.Context, tx uow.Tx, rev identity.RoleRevocation) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertUserRoleRevocation(ctx, sqlitegen.InsertUserRoleRevocationParams{
		GrantID: rev.GrantID, RevokedByUserID: toNullString(rev.RevokedByUserID), RevokedAt: toMillis(rev.RevokedAt),
	})
	if err != nil {
		return wrap(err, "insert role revocation")
	}
	return wrap(q.DeleteUserRoleByGrant(ctx, rev.GrantID), "delete user role")
}

// ListRoleHolderIDs implements identity.RoleRepository.
func (r *IdentityRepository) ListRoleHolderIDs(ctx context.Context, roleID string) ([]string, error) {
	ids, err := r.db.reader().ListRoleHolderIDs(ctx, roleID)
	return ids, wrap(err, "list role holders")
}
