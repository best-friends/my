// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/adapter/keymanager/local"
	kvdb "github.com/best-friends/my/internal/adapter/kv/db"
	kvmemory "github.com/best-friends/my/internal/adapter/kv/memory"
	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/adapter/queue/inline"
	"github.com/best-friends/my/internal/adapter/system"
	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/activitypub"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/platform/config"
)

// App is the assembled application: the adapters chosen by the configuration and the
// application services built on them. NewApp builds it with plain constructor calls; there is
// no DI container.
type App struct {
	Config   *config.Config
	Logger   *slog.Logger
	Messages *i18n.Bundle

	Clock      kernel.Clock
	IDs        kernel.IDGenerator
	KeyManager kernel.KeyManager
	Sealer     *kernel.Sealer

	UnitOfWork uow.UnitOfWork
	Projectors *uow.Projectors

	Jobs queue.JobQueue
	// Outbox enqueues jobs inside a transaction; nil for the inline queue, which has no
	// transactional enqueue.
	Outbox   queue.Outbox
	DeadJobs queue.DeadJobs
	Handlers *queue.JobHandlerRegistry
	// Runner runs the jobs of the queue for the worker role.
	Runner *queue.PullRunner

	// KV is the key-value store that cfg.KV selects; RateLimiter keeps its counters there.
	KV          kernel.KeyValueStore
	RateLimiter *kernel.RateLimiter

	Accounts *account.Service
	Identity *identity.Service
	Graph    *graph.Service

	source queue.Source
	// ready reports whether the database is reachable and its schema applied.
	ready  func(ctx context.Context) error
	closer func() error
	routes []route
}

type route struct {
	pattern string
	handler http.Handler
}

// AppOptions configure NewApp.
type AppOptions struct {
	// Logger is the process logger; the default is slog.Default().
	Logger *slog.Logger
	// Version is the server version that NodeInfo reports; the default is "dev".
	Version string
}

// NewApp connects the adapters that cfg selects and builds the application services on them.
// It does not apply the database schema (Atlas does, ADR-0023) and succeeds without it; the
// readiness check reports a missing schema. Close the App to release its connections.
func NewApp(ctx context.Context, cfg *config.Config, opts AppOptions) (*App, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	messages, err := i18n.Load()
	if err != nil {
		return nil, err
	}
	a := &App{
		Config:     cfg,
		Logger:     opts.Logger,
		Messages:   messages,
		Clock:      system.Clock{},
		IDs:        system.IDGenerator{},
		Projectors: &uow.Projectors{},
		Handlers:   &queue.JobHandlerRegistry{},
	}

	if a.KeyManager, err = newKeyManager(cfg); err != nil {
		return nil, err
	}
	a.Sealer = kernel.NewSealer(a.KeyManager)

	var (
		identities identity.Repository
		accounts   account.Repository
		follows    graph.Repository
	)
	queueOpts := queuedb.Options{Clock: a.Clock, IDs: a.IDs}
	switch cfg.DB {
	case "sqlite":
		d, err := sqlite.Open(ctx, cfg.DatabaseURL, uow.Options{Projectors: a.Projectors})
		if err != nil {
			return nil, err
		}
		a.UnitOfWork, a.ready, a.closer = d, d.Ready, d.Close
		identities, accounts = d.IdentityRepository(), d.AccountRepository()
		follows = d.GraphRepository()
		if cfg.Queue == "db" {
			a.setDBQueue(queuedb.NewSQLite(d, queueOpts))
		}
		if cfg.KV == "db" {
			a.KV = kvdb.NewSQLite(d, a.Clock)
		}
	case "postgres":
		d, err := postgres.Open(ctx, cfg.DatabaseURL, uow.Options{Projectors: a.Projectors})
		if err != nil {
			return nil, err
		}
		a.UnitOfWork, a.ready, a.closer = d, d.Ready, func() error { d.Close(); return nil }
		identities, accounts = d.IdentityRepository(), d.AccountRepository()
		follows = d.GraphRepository()
		if cfg.Queue == "db" {
			a.setDBQueue(queuedb.NewPostgres(d, queueOpts))
		}
		if cfg.KV == "db" {
			a.KV = kvdb.NewPostgres(d, a.Clock)
		}
	default:
		return nil, fmt.Errorf("db %q is not implemented yet", cfg.DB)
	}

	runnerOpts := queue.RunnerOptions{Clock: a.Clock, Logger: a.Logger}
	switch cfg.Queue {
	case "db":
		// Set with the database above.
	case "inline":
		q := inline.New(inline.Options{Clock: a.Clock, IDs: a.IDs})
		a.Jobs, a.DeadJobs = q, q
		a.Runner = queue.NewPullRunner(q, a.Handlers, runnerOpts)
		q.RunSynchronously(a.Runner)
	default:
		_ = a.Close()
		return nil, fmt.Errorf("queue %q is not implemented yet", cfg.Queue)
	}
	if a.Runner == nil {
		a.Runner = queue.NewPullRunner(a.source, a.Handlers, runnerOpts)
	}
	switch cfg.KV {
	case "db":
		// Set with the database above.
	case "memory":
		a.KV = kvmemory.New(a.Clock)
	default:
		_ = a.Close()
		return nil, fmt.Errorf("kv %q is not implemented yet", cfg.KV)
	}
	a.RateLimiter = kernel.NewRateLimiter(a.KV, a.Clock)

	account.RegisterProjectors(a.Projectors, accounts)
	a.Accounts = account.NewService(account.Deps{
		UnitOfWork: a.UnitOfWork, Repository: accounts, Clock: a.Clock, IDs: a.IDs, Sealer: a.Sealer, Host: cfg.Host,
	})
	a.Identity = identity.NewService(identity.Deps{
		UnitOfWork: a.UnitOfWork, Repository: identities, Clock: a.Clock, IDs: a.IDs,
		Passwords: identity.NewPasswordHasher(identity.DefaultArgon2Params, 0), Accounts: a.Accounts,
		RateLimiter: a.RateLimiter,
	})
	graph.RegisterProjectors(a.Projectors, follows)
	a.Graph = graph.NewService(graph.Deps{
		UnitOfWork: a.UnitOfWork, Repository: follows, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
	})

	if opts.Version == "" {
		opts.Version = "dev"
	}
	activitypub.New(activitypub.Options{
		Host: cfg.Host, AccountDomain: cfg.AccountDomain, SoftwareVersion: opts.Version, SourceURL: cfg.SourceURL,
		Accounts: a.Accounts, Problem: a.writeStatusProblem, Logger: a.Logger,
	}).Register(a)
	authpages.New(authpages.Options{Identity: a.Identity, Messages: a.Messages, Logger: a.Logger}).Register(a)
	return a, nil
}

func (a *App) setDBQueue(q *queuedb.Queue) {
	a.Jobs, a.Outbox, a.DeadJobs, a.source = q, q, q, q
}

func newKeyManager(cfg *config.Config) (kernel.KeyManager, error) {
	switch cfg.KeyManager {
	case "local":
		return local.New(cfg.KEK, cfg.PreviousKEKs...)
	default:
		return nil, fmt.Errorf("key_manager %q is not implemented yet", cfg.KeyManager)
	}
}

// Ready reports whether the App can serve requests: the database is reachable and every table
// of its schema file exists.
func (a *App) Ready(ctx context.Context) error {
	if a.ready == nil {
		return errors.New("no database")
	}
	return a.ready(ctx)
}

// Handle registers h for pattern (an http.ServeMux pattern) on the HTTP server of the web role.
// Call it before Serve.
func (a *App) Handle(pattern string, h http.Handler) {
	a.routes = append(a.routes, route{pattern: pattern, handler: h})
}

// Close releases the database connections.
func (a *App) Close() error {
	if a.closer == nil {
		return nil
	}
	return a.closer()
}
