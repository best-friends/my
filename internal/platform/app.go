// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	blobfs "github.com/best-friends/my/internal/adapter/blob/fs"
	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/adapter/keymanager/local"
	kvdb "github.com/best-friends/my/internal/adapter/kv/db"
	kvmemory "github.com/best-friends/my/internal/adapter/kv/memory"
	"github.com/best-friends/my/internal/adapter/media/native"
	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/adapter/queue/inline"
	"github.com/best-friends/my/internal/adapter/system"
	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/blob"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/activitypub"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/interface/clientapi"
	mediahttp "github.com/best-friends/my/internal/interface/media"
	"github.com/best-friends/my/internal/platform/config"
)

// defaultMaxPostCharacters is the post length limit that the client API reports until the
// instance settings hold it (FR-101).
const defaultMaxPostCharacters = 500

// App is the assembled application: the adapters chosen by the configuration and the
// application services built on them. NewApp builds it with plain constructor calls; there is
// no DI container.
type App struct {
	Config   *config.Config
	Logger   *slog.Logger
	Messages *i18n.Bundle
	// Locales resolves the locale of every HTTP response (ADR-0009 §2) on Messages and the
	// instance's default_locale setting; every HTTP surface localizes through it.
	Locales *i18n.Resolver

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
	// Instance holds the instance settings and texts, which live in the database; Config holds
	// the deployment configuration. They never share a key (docs/reference/configuration.md).
	Instance *instance.Service
	Graph    *graph.Service
	Content  *content.Service
	// Interactions runs boosts, favourites, bookmarks, and pins (FR-107, FR-113).
	Interactions *content.Interactions
	// Notifications lists a local account's notifications, which projectors create from the
	// graph and content events, with its read marker and dismissals (FR-205).
	Notifications *notification.Service

	// Blobs stores media files (ADR-0005); MediaProcessor checks and re-encodes them.
	Blobs          blob.Store
	MediaProcessor media.Processor
	Media          *content.MediaService
	// Scheduler enqueues the periodic jobs for the worker role.
	Scheduler *queue.Scheduler

	source queue.Source
	// ready reports whether the database is reachable and its schema applied.
	ready  func(ctx context.Context) error
	closer func() error
	// closeBlobs releases the blob store.
	closeBlobs func() error
	routes     []route
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
		identities   identity.Repository
		accounts     account.Repository
		instances    instance.Repository
		follows      graph.Repository
		statuses     content.Repository
		interactions content.InteractionRepository
		notifs       notification.Repository
		medias       content.MediaRepository
	)
	queueOpts := queuedb.Options{Clock: a.Clock, IDs: a.IDs}
	switch cfg.DB {
	case "sqlite":
		d, err := sqlite.Open(ctx, cfg.DatabaseURL, uow.Options{Projectors: a.Projectors})
		if err != nil {
			return nil, err
		}
		a.UnitOfWork, a.ready, a.closer = d, d.Ready, d.Close
		identities, accounts, instances = d.IdentityRepository(), d.AccountRepository(), d.InstanceRepository()
		follows, statuses, medias = d.GraphRepository(), d.StatusRepository(), d.MediaRepository()
		interactions = d.InteractionRepository()
		notifs = d.NotificationRepository()
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
		identities, accounts, instances = d.IdentityRepository(), d.AccountRepository(), d.InstanceRepository()
		follows, statuses, medias = d.GraphRepository(), d.StatusRepository(), d.MediaRepository()
		interactions = d.InteractionRepository()
		notifs = d.NotificationRepository()
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
	instance.RegisterProjectors(a.Projectors, instances)
	a.Instance = instance.NewService(instance.Deps{UnitOfWork: a.UnitOfWork, Repository: instances, Clock: a.Clock, IDs: a.IDs})
	// The user step joins once API requests are authenticated (#107).
	a.Locales = &i18n.Resolver{Bundle: a.Messages, InstanceDefault: a.instanceDefaultLocale}
	a.Identity = identity.NewService(identity.Deps{
		UnitOfWork: a.UnitOfWork, Repository: identities, Clock: a.Clock, IDs: a.IDs,
		Passwords: identity.NewPasswordHasher(identity.DefaultArgon2Params, 0), Accounts: a.Accounts,
		RateLimiter: a.RateLimiter,
		// The default locale is an instance setting in the database, never configuration
		// (docs/reference/configuration.md); it is read per registration so changes apply at once.
		DefaultLocale: a.Instance.DefaultLocale,
	})
	graph.RegisterProjectors(a.Projectors, follows)
	a.Graph = graph.NewService(graph.Deps{
		UnitOfWork: a.UnitOfWork, Repository: follows, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
	})
	content.RegisterProjectors(a.Projectors, statuses)
	a.Content = content.NewService(content.Deps{
		UnitOfWork: a.UnitOfWork, Repository: statuses, Accounts: a.Accounts, Relationships: a.Graph,
		// No moderation restrictions exist yet; posting still goes through the policy port.
		Restrictions:    content.NoRestrictions{},
		MaxStatusLength: a.Instance.MaxStatusLength,
		DefaultLanguage: a.defaultPostLanguage,
		Clock:           a.Clock, IDs: a.IDs, Host: cfg.Host, AccountDomain: cfg.AccountDomain,
	})
	// After RegisterProjectors, which creates the counters that these projectors update.
	content.RegisterInteractionProjectors(a.Projectors, interactions)
	a.Interactions = content.NewInteractions(content.InteractionDeps{
		UnitOfWork: a.UnitOfWork, Statuses: statuses, Interactions: interactions,
		Accounts: a.Accounts, Relationships: a.Graph, Restrictions: content.NoRestrictions{},
		Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
	})
	// After the graph and content projectors, whose events these projectors turn into notifications.
	notification.RegisterProjectors(a.Projectors, notification.ProjectorDeps{
		Repository: notifs, Accounts: a.Accounts, Relationships: a.Graph, IDs: a.IDs,
	})
	a.Notifications = notification.NewService(notification.Deps{UnitOfWork: a.UnitOfWork, Repository: notifs, Clock: a.Clock})

	if opts.Version == "" {
		opts.Version = "dev"
	}
	activitypub.New(activitypub.Options{
		Host: cfg.Host, AccountDomain: cfg.AccountDomain, SoftwareVersion: opts.Version, SourceURL: cfg.SourceURL,
		Accounts: a.Accounts, Problem: a.writeStatusProblem, Logger: a.Logger,
	}).Register(a)
	authpages.New(authpages.Options{Identity: a.Identity, Locales: a.Locales, Logger: a.Logger}).Register(a)
	if err := a.setMedia(cfg, medias); err != nil {
		_ = a.Close()
		return nil, err
	}
	mediahttp.New(mediahttp.Options{Blobs: a.Blobs, Problem: a.writeStatusProblem, Logger: a.Logger}).Register(a)
	// The instance is named after its host until the instance settings hold a name.
	clientapi.New(clientapi.Options{
		Host: cfg.Host, Name: cfg.Host, SoftwareVersion: opts.Version, SourceURL: cfg.SourceURL,
		MaxPostCharacters: defaultMaxPostCharacters,
		Locales:           a.Locales, RequestID: RequestID, Logger: a.Logger,
	}).Register(a)
	return a, nil
}

// setMedia builds the blob store, the media processor, and the media use cases, and registers
// their jobs and schedules.
func (a *App) setMedia(cfg *config.Config, repo content.MediaRepository) error {
	switch cfg.Blob {
	case "fs":
		store, err := blobfs.New(blobfs.Options{Dir: cfg.BlobFSDir, BaseURL: "https://" + cfg.Host})
		if err != nil {
			return err
		}
		a.Blobs, a.closeBlobs = store, store.Close
	default:
		return fmt.Errorf("blob %q is not implemented yet", cfg.Blob)
	}
	switch cfg.Media {
	case "native":
		a.MediaProcessor = native.New(native.Options{})
	default:
		return fmt.Errorf("media %q is not implemented yet", cfg.Media)
	}
	a.Scheduler = queue.NewScheduler(a.Jobs, a.Clock, a.Logger)
	a.Media = content.NewMediaService(content.MediaDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Blobs: a.Blobs, Processor: a.MediaProcessor,
		Outbox: a.Outbox, Jobs: a.Jobs, Clock: a.Clock, IDs: a.IDs,
		Limits: media.Limits{
			MaxBytes: cfg.MediaMaxImageBytes, MaxSide: cfg.MediaMaxImageSide, MaxPixels: cfg.MediaMaxImagePixels,
		},
		UnattachedTTL: cfg.MediaUnattachedTTL,
		// Avatars and headers refer to media files from the account module.
		Referrers: []content.MediaReferrer{a.Accounts},
		Logger:    a.Logger,
	})
	a.Media.RegisterJobs(a.Handlers, a.Scheduler)
	return nil
}

// defaultPostLanguage returns the default post language of a user's preferences, or "" for a
// user without preferences.
func (a *App) defaultPostLanguage(ctx context.Context, userID string) (string, error) {
	p, err := a.Identity.Preferences(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return p.DefaultPostLanguage, nil
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
	if a.closeBlobs != nil {
		_ = a.closeBlobs()
	}
	if a.closer == nil {
		return nil
	}
	return a.closer()
}
