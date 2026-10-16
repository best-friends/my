// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/adapter/keymanager/local"
	"github.com/best-friends/my/internal/adapter/system"
	appaccount "github.com/best-friends/my/internal/app/account"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/platform/config"
)

// AdminCLI holds what the `my admin` subcommands run on: the identity use cases over the
// configured database and key manager. Close it when done.
type AdminCLI struct {
	Identity *appidentity.Service
	close    func()
}

// Close releases the database connections.
func (a *AdminCLI) Close() { a.close() }

// OpenAdminCLI connects to the database of cfg and wires the identity and account use cases
// onto it. It does not apply the schema; Atlas does (ADR-0023).
func OpenAdminCLI(ctx context.Context, cfg *config.Config) (*AdminCLI, error) {
	if cfg.KeyManager != "local" {
		return nil, fmt.Errorf("key_manager %q is not supported yet; use local", cfg.KeyManager)
	}
	km, err := local.New(cfg.KEK, cfg.PreviousKEKs...)
	if err != nil {
		return nil, err
	}
	projectors := &uow.Projectors{}
	opts := uow.Options{Projectors: projectors}
	var (
		unit       uow.UnitOfWork
		identities appidentity.Repository
		accounts   appaccount.Repository
		closeDB    func()
	)
	switch cfg.DB {
	case "sqlite":
		db, err := sqlite.Open(ctx, cfg.DatabaseURL, opts)
		if err != nil {
			return nil, err
		}
		unit, identities, accounts = db, db.IdentityRepository(), db.AccountRepository()
		closeDB = func() { _ = db.Close() }
	case "postgres":
		db, err := postgres.Open(ctx, cfg.DatabaseURL, opts)
		if err != nil {
			return nil, err
		}
		unit, identities, accounts = db, db.IdentityRepository(), db.AccountRepository()
		closeDB = db.Close
	default:
		return nil, fmt.Errorf("db %q is not supported yet; use sqlite or postgres", cfg.DB)
	}
	appaccount.RegisterProjectors(projectors, accounts)

	clock, ids := system.Clock{}, system.IDGenerator{}
	accountService := appaccount.NewService(appaccount.Deps{
		UnitOfWork: unit,
		Repository: accounts,
		Clock:      clock,
		IDs:        ids,
		Sealer:     kernel.NewSealer(km),
		Host:       cfg.Host,
	})
	identityService := appidentity.NewService(appidentity.Deps{
		UnitOfWork: unit,
		Repository: identities,
		Clock:      clock,
		IDs:        ids,
		Passwords:  appidentity.NewPasswordHasher(appidentity.DefaultArgon2Params, 1),
		Accounts:   accountService,
	})
	return &AdminCLI{Identity: identityService, close: closeDB}, nil
}
