// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// IdentityRepository implements identity.Repository on PostgreSQL.
type IdentityRepository struct {
	db *DB
}

var _ appidentity.Repository = (*IdentityRepository)(nil)

// IdentityRepository returns the identity module's repository on d.
func (d *DB) IdentityRepository() *IdentityRepository { return &IdentityRepository{db: d} }

// InsertUser implements identity.Repository.
func (r *IdentityRepository) InsertUser(ctx context.Context, tx uow.Tx, u identity.User) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.InsertUser(ctx, postgresgen.InsertUserParams{ID: u.ID, CreatedAt: u.CreatedAt}), "insert user")
}

// InsertPasswordCredential implements identity.Repository.
func (r *IdentityRepository) InsertPasswordCredential(ctx context.Context, tx uow.Tx, c identity.PasswordCredential) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertPasswordCredential(ctx, postgresgen.InsertPasswordCredentialParams{
		ID: c.ID, UserID: c.UserID, PasswordHash: c.Hash, SetAt: c.SetAt,
	})
	return wrap(err, "insert password credential")
}

// EnsureRole implements identity.Repository.
func (r *IdentityRepository) EnsureRole(ctx context.Context, tx uow.Tx, role identity.Role, perms []identity.Permission) (identity.Role, error) {
	q, err := queries(tx)
	if err != nil {
		return identity.Role{}, err
	}
	pos, err := toInt32(role.Position, "role position")
	if err != nil {
		return identity.Role{}, err
	}
	err = q.InsertRoleIfAbsent(ctx, postgresgen.InsertRoleIfAbsentParams{
		ID: role.ID, Key: role.Key, Position: pos, CreatedAt: role.CreatedAt,
	})
	if err != nil {
		return identity.Role{}, wrap(err, "insert role")
	}
	row, err := q.GetRoleByKey(ctx, role.Key)
	if err != nil {
		return identity.Role{}, wrap(err, "get role")
	}
	for _, p := range perms {
		err := q.InsertRolePermissionIfAbsent(ctx, postgresgen.InsertRolePermissionIfAbsentParams{RoleID: row.ID, Permission: string(p)})
		if err != nil {
			return identity.Role{}, wrap(err, "insert role permission")
		}
	}
	return roleFrom(row), nil
}

func roleFrom(row postgresgen.Role) identity.Role {
	return identity.Role{ID: row.ID, Key: row.Key, Position: int(row.Position), CreatedAt: utc(row.CreatedAt)}
}

// FindUser implements identity.Repository.
func (r *IdentityRepository) FindUser(ctx context.Context, id string) (identity.User, error) {
	row, err := r.db.reader().GetUser(ctx, id)
	if err != nil {
		return identity.User{}, notFound(err, appidentity.ErrNotFound, "user "+id)
	}
	return identity.User{ID: row.ID, CreatedAt: utc(row.CreatedAt)}, nil
}

// FindCurrentPasswordCredential implements identity.Repository.
func (r *IdentityRepository) FindCurrentPasswordCredential(ctx context.Context, userID string) (identity.PasswordCredential, error) {
	row, err := r.db.reader().GetCurrentPasswordCredential(ctx, userID)
	if err != nil {
		return identity.PasswordCredential{}, notFound(err, appidentity.ErrNotFound, "password of "+userID)
	}
	return identity.PasswordCredential{ID: row.ID, UserID: row.UserID, Hash: row.PasswordHash, SetAt: utc(row.SetAt)}, nil
}

// FindRole implements identity.Repository.
func (r *IdentityRepository) FindRole(ctx context.Context, key string) (identity.Role, []identity.Permission, error) {
	q := r.db.reader()
	row, err := q.GetRoleByKey(ctx, key)
	if err != nil {
		return identity.Role{}, nil, notFound(err, appidentity.ErrNotFound, "role "+key)
	}
	perms, err := q.ListRolePermissions(ctx, row.ID)
	if err != nil {
		return identity.Role{}, nil, wrap(err, "list role permissions")
	}
	out := make([]identity.Permission, len(perms))
	for i, p := range perms {
		out[i] = identity.Permission(p)
	}
	return roleFrom(row), out, nil
}
