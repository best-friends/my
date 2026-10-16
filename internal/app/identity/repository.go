// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// ErrNotFound reports that no row matches a lookup.
var ErrNotFound = errors.New("identity: not found")

// Repository is the persistence port of the identity module (data-model §4.1). The database
// adapters implement it and pass testkit.AccountRepositoryContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type Repository interface {
	InsertUser(ctx context.Context, tx uow.Tx, u identity.User) error
	InsertPasswordCredential(ctx context.Context, tx uow.Tx, c identity.PasswordCredential) error
	// EnsureRole inserts a role unless one with the same key exists, adds the permissions it
	// lacks, and returns the stored role (whose ID and creation time are those of the first insert).
	EnsureRole(ctx context.Context, tx uow.Tx, r identity.Role, perms []identity.Permission) (identity.Role, error)

	FindUser(ctx context.Context, id string) (identity.User, error)
	// FindCurrentPasswordCredential returns the newest password credential of a user.
	FindCurrentPasswordCredential(ctx context.Context, userID string) (identity.PasswordCredential, error)
	// FindRole returns a role by key with its permissions, sorted.
	FindRole(ctx context.Context, key string) (identity.Role, []identity.Permission, error)
}
