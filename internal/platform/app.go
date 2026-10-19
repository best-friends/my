// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"slices"

	blobfs "github.com/best-friends/my/internal/adapter/blob/fs"
	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/db/sqlite"
	"github.com/best-friends/my/internal/adapter/httpclient"
	"github.com/best-friends/my/internal/adapter/httpsig"
	"github.com/best-friends/my/internal/adapter/keymanager/local"
	kvdb "github.com/best-friends/my/internal/adapter/kv/db"
	kvmemory "github.com/best-friends/my/internal/adapter/kv/memory"
	mailnone "github.com/best-friends/my/internal/adapter/mail/none"
	mailsmtp "github.com/best-friends/my/internal/adapter/mail/smtp"
	"github.com/best-friends/my/internal/adapter/media/native"
	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/adapter/queue/inline"
	"github.com/best-friends/my/internal/adapter/system"
	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/blob"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/app/uow"
	domainidentity "github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/activitypub"
	"github.com/best-friends/my/internal/interface/authpages"
	"github.com/best-friends/my/internal/interface/clientapi"
	mediahttp "github.com/best-friends/my/internal/interface/media"
	"github.com/best-friends/my/internal/interface/oauth"
	"github.com/best-friends/my/internal/interface/web"
	"github.com/best-friends/my/internal/platform/config"
)

// WebClientCallbackPath is the path on the web origin where the web client receives the
// authorization response; its URL is the redirect URI of the first-party web client.
const WebClientCallbackPath = "/login/callback"

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
	// Mailer is the mail adapter that cfg.Mail selects.
	Mailer kernel.Mailer
	// HTTPClient sends every outbound HTTP request through the SSRF guard (ADR-0019 §1).
	HTTPClient kernel.HTTPClient

	Accounts *account.Service
	Identity *identity.Service
	// AccountAdmin is the moderators' account management (FR-714).
	AccountAdmin *identity.AccountAdmin
	// Instance holds the instance settings and texts, which live in the database; Config holds
	// the deployment configuration. They never share a key (docs/reference/configuration.md).
	Instance *instance.Service
	Graph    *graph.Service
	Content  *content.Service
	// Interactions runs boosts, favourites, bookmarks, and pins (FR-107, FR-113).
	Interactions *content.Interactions
	// Statuses presents statuses, their threads and history, and bookmarks to a viewer.
	Statuses *content.Reader
	// Timeline reads the home, local, hashtag, and profile timelines (FR-201 to FR-204).
	Timeline *timeline.Service
	// Moderation records the actions on domains and accounts and the reports received from
	// other servers; ModerationPolicy reads the restrictions they impose for federation and the
	// media proxy (docs/architecture/federation.md §11), and AccountPolicy those that the
	// identity, graph, and content modules and the client API apply (data-model §5.3).
	Moderation       *moderation.Service
	ModerationPolicy *moderation.FederationPolicy
	AccountPolicy    *moderation.AccountPolicy
	// LocalReports files the reports of local users (FR-701).
	LocalReports *moderation.LocalReports

	// Notifications lists a local account's notifications, which projectors create from the
	// graph and content events, with its read marker and dismissals (FR-205).
	Notifications *notification.Service
	// Streams is the in-process broker of the streaming endpoint (FR-207): the timeline and
	// notification commit hooks publish to it, and each open stream subscribes. It reaches only
	// the streams of this process (docs/architecture/overview.md §4.4).
	Streams *stream.Hub

	// Blobs stores media files (ADR-0005); MediaProcessor checks and re-encodes them.
	Blobs          blob.Store
	MediaProcessor media.Processor
	Media          *content.MediaService
	// MediaProxy relays remote media files under /media/proxy/ and signs their URLs (ADR-0005).
	MediaProxy *content.MediaProxy
	// Previews reads profiles and statuses as an anonymous visitor sees them, for the link
	// previews of the web client's profile and post pages (ADR-0028).
	Previews *content.Previews
	// Scheduler enqueues the periodic jobs for the worker role.
	Scheduler *queue.Scheduler
	// FanOut turns the outbound activities that the federation projectors record into
	// deliveries (docs/architecture/federation.md §7).
	FanOut *federation.FanOut
	// RemoteActors fetches remote actors, checks their handles, keeps their keys, and resolves
	// the keys of inbound signatures (docs/architecture/federation.md §6).
	RemoteActors *federation.RemoteActors
	// Deliverer sends the deliveries; InstanceHealth records the remote servers that answer
	// outside deliveries (docs/architecture/federation.md §7.3).
	Deliverer      *federation.Deliverer
	InstanceHealth *federation.Health
	// Inbox verifies and records the activities POSTed to the inboxes
	// (docs/architecture/federation.md §3.1).
	Inbox *federation.Inbox
	// InboxProcessor processes the recorded activities and dispatches them to the handler of
	// their type (docs/architecture/federation.md §3.2).
	InboxProcessor *federation.InboxProcessor
	// RemoteStatuses ingests remote posts from the inbox and fetches remote posts and threads;
	// Resolver resolves handles and the URLs of accounts and posts
	// (docs/architecture/federation.md §6.2, §6.3).
	RemoteStatuses *federation.StatusHandlers
	Resolver       *federation.Resolver

	source queue.Source
	// ready reports whether the database is reachable and its schema applied.
	ready  func(ctx context.Context) error
	closer func() error
	// closeBlobs releases the blob store.
	closeBlobs func() error
	routes     []route
	// page serves the web client's HTML at the GET paths that no route serves.
	page http.Handler
	// webStylesheet is the URL of the server-rendered pages' stylesheet from the web build, empty without one.
	webStylesheet string
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
	// WebBuild is the build of the web client; the default is the one embedded in the binary.
	WebBuild fs.FS
	// DevViteOrigin, when set, makes the web client's shell load the client from a Vite dev
	// server at this origin (`my serve -dev-vite-origin`). Development only.
	DevViteOrigin string
	// PseudoLocales makes the pseudo-locales en-XA and ar-XB selectable on every surface
	// (`my serve -pseudo-locales`), for the E2E of the server-rendered pages. Development and tests only.
	PseudoLocales bool
	// HTTPClient sends every outbound request; nil builds the SSRF-guarded client from the
	// configuration. Tests only, to reach other test servers by their public hosts.
	HTTPClient kernel.HTTPClient
}

// NewApp connects the adapters that cfg selects and builds the application services on them.
// It does not apply the database schema (Atlas does, ADR-0023) and succeeds without it; the
// readiness check reports a missing schema. Close the App to release its connections.
func NewApp(ctx context.Context, cfg *config.Config, opts AppOptions) (*App, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	var i18nOpts []i18n.Option
	if opts.PseudoLocales {
		i18nOpts = append(i18nOpts, i18n.WithPseudoLocales())
	}
	messages, err := i18n.Load(i18nOpts...)
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
		feeds        timeline.Repository
		notifs       notification.Repository
		medias       content.MediaRepository
		feds         federationRepository
		mods         moderation.Repository
		retention    retentionRepositories
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
		interactions, feeds = d.InteractionRepository(), d.TimelineRepository()
		notifs = d.NotificationRepository()
		feds = d.FederationRepository()
		mods = d.ModerationRepository()
		retention = retentionRepositories{statuses: d.StatusRepository(), purges: d.FederationRepository(), profiles: d.AccountRepository()}
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
		interactions, feeds = d.InteractionRepository(), d.TimelineRepository()
		notifs = d.NotificationRepository()
		feds = d.FederationRepository()
		mods = d.ModerationRepository()
		retention = retentionRepositories{statuses: d.StatusRepository(), purges: d.FederationRepository(), profiles: d.AccountRepository()}
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
	a.Scheduler = queue.NewScheduler(a.Jobs, a.Clock, a.Logger)
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
	a.registerKVCleanup()

	account.RegisterProjectors(a.Projectors, accounts)
	a.Accounts = account.NewService(account.Deps{
		UnitOfWork: a.UnitOfWork, Repository: accounts, Clock: a.Clock, IDs: a.IDs, Sealer: a.Sealer, Host: cfg.Host,
		// Profiles are rendered and their images checked by the content module, built below; the
		// account module does not import it, since the content module imports the account module.
		RenderProfileText: func(ctx context.Context, text string) (string, error) {
			return a.Content.RenderProfileText(ctx, text)
		},
		CheckProfileImage: func(ctx context.Context, accountID, mediaID string) error {
			return a.Media.CheckProfileImage(ctx, accountID, mediaID)
		},
		Sanitizer:         newHTMLSanitizer(),
		RecordRemoteImage: a.recordRemoteImage,
	})
	instance.RegisterProjectors(a.Projectors, instances)
	a.Instance = instance.NewService(instance.Deps{
		UnitOfWork: a.UnitOfWork, Repository: instances, Clock: a.Clock, IDs: a.IDs,
		// The preview_image setting refers to a media file, which setMedia sets up later.
		Media: instanceMedia{a},
	})
	if a.Mailer, err = newMailer(cfg, a.Logger); err != nil {
		_ = a.Close()
		return nil, err
	}
	// The user step is the locale of the client API request's authenticated viewer.
	a.Locales = &i18n.Resolver{Bundle: a.Messages, UserLocale: a.viewerLocale, InstanceDefault: a.instanceDefaultLocale}
	// Moderation needs only the account module; the modules below read its restrictions.
	a.setUpModeration(mods)
	identity.RegisterModerationProjectors(a.Projectors, identities, a.Accounts, a.Clock)
	identity.RegisterLifecycleProjectors(a.Projectors, identities, a.Accounts, a.Clock)
	a.Identity = identity.NewService(identity.Deps{
		UnitOfWork: a.UnitOfWork, Repository: identities, Clock: a.Clock, IDs: a.IDs,
		Passwords: identity.NewPasswordHasher(identity.DefaultArgon2Params, 0), Accounts: a.Accounts,
		Logins: a.AccountPolicy, RateLimiter: a.RateLimiter,
		// The default locale and the registration mode are instance settings in the database,
		// never configuration (docs/reference/configuration.md); they are read per registration so
		// changes apply at once.
		DefaultLocale: a.Instance.DefaultLocale,
		WebClient: identity.WebClient{
			ClientID: identity.DefaultWebClientID, Name: cfg.Host, RedirectURIs: []string{cfg.WebOrigin + WebClientCallbackPath},
		},
		RegistrationMode: func(ctx context.Context) (domainidentity.RegistrationMode, error) {
			mode, err := a.Instance.RegistrationMode(ctx)
			return domainidentity.RegistrationMode(mode), err
		},
		Mailer: a.Mailer,
		// The registration page screens sign-ups against the sign-up block list (FR-719).
		SignupBlocks: a.Moderation,
		// mail = none delivers nothing, so a moderator's approval stands in for verification (FR-002).
		RequireEmailVerification: cfg.Mail != "none",
		Messages:                 a.Messages,
		BaseURL:                  cfg.ServerOrigin(),
	})
	a.AccountAdmin = identity.NewAccountAdmin(identity.AccountAdminDeps{
		UnitOfWork: a.UnitOfWork, Repository: identities, Accounts: a.Accounts, Restrictions: a.AccountPolicy,
		Clock: a.Clock, IDs: a.IDs,
	})
	// The sign-in and registration origins are erased after their retention (ADR-0038).
	identity.NewOriginPurger(identity.OriginPurgeDeps{
		UnitOfWork: a.UnitOfWork, Repository: identities, Clock: a.Clock, Policy: originRetention{a},
	}).RegisterJobs(a.Handlers, a.Scheduler)
	graph.RegisterProjectors(a.Projectors, follows)
	a.Graph = graph.NewService(graph.Deps{
		UnitOfWork: a.UnitOfWork, Repository: follows, Accounts: a.Accounts, Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
		Interactions: a.AccountPolicy,
	})
	a.Graph.RegisterJobs(a.Handlers, a.Scheduler)
	content.RegisterProjectors(a.Projectors, statuses)
	a.Content = content.NewService(content.Deps{
		UnitOfWork: a.UnitOfWork, Repository: statuses, Accounts: a.Accounts, Relationships: a.Graph,
		Restrictions:    a.AccountPolicy.Content(),
		Media:           medias,
		RemoteMentions:  remoteMentions{a}, // reads a.RemoteActors, built by setUpRemoteActors
		Sanitizer:       newHTMLSanitizer(),
		RemoteMedia:     remoteMediaRecorder{a}, // reads a.Media, built by setMedia
		MaxStatusLength: a.Instance.MaxStatusLength,
		DefaultLanguage: a.defaultPostLanguage,
		Clock:           a.Clock, IDs: a.IDs, Host: cfg.Host, AccountDomain: cfg.AccountDomain,
	})
	// After RegisterProjectors, which creates the counters that these projectors update.
	content.RegisterInteractionProjectors(a.Projectors, interactions)
	a.Interactions = content.NewInteractions(content.InteractionDeps{
		UnitOfWork: a.UnitOfWork, Statuses: statuses, Interactions: interactions,
		Accounts: a.Accounts, Relationships: a.Graph, Restrictions: a.AccountPolicy.Content(),
		Clock: a.Clock, IDs: a.IDs, Host: cfg.Host,
	})
	// After the content and graph projectors, which reject the commands that must not reach
	// the timelines (data-model §4.6).
	timeline.RegisterProjectors(a.Projectors, feeds)
	timeline.RegisterModerationProjectors(a.Projectors, feeds, a.Accounts)
	a.Timeline = timeline.NewService(timeline.Deps{Repository: feeds, Clock: a.Clock})
	// After the graph and content projectors, whose events these projectors turn into notifications.
	notification.RegisterProjectors(a.Projectors, notification.ProjectorDeps{
		Repository: notifs, Accounts: a.Accounts, Relationships: a.Graph, Reach: a.AccountPolicy, IDs: a.IDs,
	})
	a.Notifications = notification.NewService(notification.Deps{
		UnitOfWork: a.UnitOfWork, Repository: notifs, Clock: a.Clock, Staff: moderatorStaff{a},
	})
	a.setUpModeratorNotifications(cfg, notifs, moderatorStaff{a})
	a.setUpRetention(cfg, retention, statuses)
	a.Streams = stream.NewHub(stream.HubOptions{})
	timeline.RegisterCommitHooks(a.Projectors, a.Streams)
	notification.RegisterCommitHooks(a.Projectors, a.Streams)
	fedOutbox := a.setUpFederation(cfg, feds)

	if opts.Version == "" {
		opts.Version = "dev"
	}
	a.HTTPClient = opts.HTTPClient
	if a.HTTPClient == nil {
		a.HTTPClient = newHTTPClient(cfg, opts.Version, a.Logger)
	}
	a.setUpDelivery(feds)
	if err := a.setBlobs(cfg); err != nil {
		_ = a.Close()
		return nil, err
	}
	// The media service presents remote files with the proxy's signed URLs, so the proxy comes first.
	if a.MediaProxy, err = newMediaProxy(cfg, medias, a); err != nil {
		_ = a.Close()
		return nil, err
	}
	if err := a.setMedia(cfg, medias); err != nil {
		_ = a.Close()
		return nil, err
	}
	if err := a.setUpErasure(identities, accounts, statuses, follows, feeds, notifs, medias); err != nil {
		_ = a.Close()
		return nil, err
	}
	a.setUpRemoteActors(cfg, feds)
	a.setUpInbox(feds)
	a.Statuses = content.NewReader(content.ReaderDeps{
		Statuses: statuses, Interactions: interactions, Accounts: a.Accounts, Relationships: a.Graph,
		Restrictions: a.AccountPolicy.Content(), Media: a.Media,
	})
	a.setUpLocalReports(mods)
	a.setUpFanOut(cfg, feds, fedOutbox)
	// Link previews read statuses and their images, so the web client comes after media.
	a.Previews = content.NewPreviews(content.PreviewDeps{
		Repository: statuses, Accounts: a.Accounts, Restrictions: a.AccountPolicy.Content(), Media: a.Media,
		DefaultLanguage: a.defaultPostLanguage,
	})
	if err := a.setUpWeb(opts); err != nil {
		_ = a.Close()
		return nil, err
	}
	activitypub.New(activitypub.Options{
		Host: cfg.Host, AccountDomain: cfg.AccountDomain, SoftwareVersion: opts.Version, SourceURL: cfg.SourceURL,
		Accounts: a.Accounts, Objects: a.newObjects(cfg), Graph: a.Graph, Inbox: a.Inbox,
		HTML: a.page, Problem: a.writeStatusProblem, Logger: a.Logger,
		Suspensions: a.ModerationPolicy,
	}).Register(a)
	// The sign-in screens load the stylesheet the web build makes for them (ADR-0030).
	authpages.New(authpages.Options{
		Identity: a.Identity, OAuth: a.Identity, Issuer: cfg.ServerOrigin(), FormActionOrigins: []string{cfg.WebOrigin},
		Locales: a.Locales, Stylesheet: a.webStylesheet, Logger: a.Logger,
		Moderation: a.Moderation, UserLocales: a.Identity,
	}).Register(a)
	tokens, err := oauth.New(oauth.Options{Identity: a.Identity, WebOrigin: cfg.WebOrigin, Logger: a.Logger})
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	tokens.Register(a)
	mediahttp.New(mediahttp.Options{Blobs: a.Blobs, Problem: a.writeStatusProblem, Logger: a.Logger}).Register(a)
	mediahttp.NewProxy(mediahttp.ProxyOptions{Media: a.MediaProxy, Problem: a.writeStatusProblem, Logger: a.Logger}).Register(a)
	clientapi.New(clientapi.Options{
		Host: cfg.Host, Origin: cfg.ServerOrigin(), SoftwareVersion: opts.Version, SourceURL: cfg.SourceURL, Instance: a.Instance,
		SendsEmail:    sendsEmail(cfg),
		Authenticator: a.Identity, Accounts: a.Accounts, Profiles: a.Accounts, Posting: a.Content, Preferences: a.Identity, Media: a.Media,
		AccountDomain: cfg.AccountDomain, Graph: a.Graph,
		Resolver: a.RemoteActors, RemoteProfiles: a.Accounts,
		Notifications: a.Notifications,
		Statuses:      a.Content, StatusReader: a.Statuses, Interactions: a.Interactions, Threads: a.RemoteStatuses,
		Timelines: a.Timeline, TimelineReader: a.Statuses, TimelineLanguages: a.timelineLanguages,
		Streams: a.Streams, HomeEntries: a.Timeline,
		Permissions: a.Identity, InstanceHealth: a.InstanceHealth, DeadJobs: a.DeadJobs,
		Moderation: a.Moderation, UserAccounts: a.Accounts, FiledReports: a.Moderation, Severances: a.Moderation,
		AccountModeration: a.Moderation, ProfileRestrictions: a.AccountPolicy, OwnModeration: a.Moderation,
		AccountAdmin: a.AccountAdmin, AccountReports: a.Moderation,
		AuditLog:     a.Moderation,
		Rules:        a.Instance,
		Roles:        a.Identity,
		Reports:      a.LocalReports,
		ReportCounts: a.Moderation, RegistrationCounts: a.Identity, Registrations: a.Identity,
		OwnAppeals: a.Moderation, AppealModeration: a.Moderation, SignupBlocks: a.Moderation,
		RateLimiter: a.RateLimiter, Locales: a.Locales, RequestID: RequestID, Logger: a.Logger,
	}).Register(a)
	return a, nil
}

// setUpFederation registers the projectors that record outbound activities, after those of the
// modules whose events they follow (federation.md §7), and returns the outbox they enqueue the
// fan-out with.
func (a *App) setUpFederation(cfg *config.Config, repo federation.Repository) queue.Outbox {
	outbox := a.Outbox
	if outbox == nil {
		// The inline queue has no transactional enqueue; its jobs are enqueued after the commit.
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	federation.RegisterOutboundProjectors(a.Projectors, federation.RecorderDeps{
		Repository: repo, Outbox: outbox, IDs: a.IDs, Host: cfg.Host, Accounts: a.Accounts,
	})
	federation.RegisterBlockProjectors(a.Projectors, federation.RecorderDeps{
		Repository: repo, Outbox: outbox, IDs: a.IDs, Host: cfg.Host, Accounts: a.Accounts,
	})
	federation.RegisterSuspensionProjectors(a.Projectors, federation.RecorderDeps{
		Repository: repo, Outbox: outbox, IDs: a.IDs, Host: cfg.Host, Accounts: a.Accounts,
	})
	// Forwarded reports (FR-709) are Flags of the instance actor.
	federation.RegisterReportProjectors(a.Projectors, federation.RecorderDeps{
		Repository: repo, Outbox: outbox, IDs: a.IDs, Host: cfg.Host,
	}, a.Accounts)
	return outbox
}

// setUpFanOut registers the fan-out of outbound activities (federation.md §7.2). It comes after
// the status reader, which resolves the media, mentions, and hashtags of the Notes it builds.
func (a *App) setUpFanOut(cfg *config.Config, repo federation.Repository, outbox queue.Outbox) {
	a.FanOut = federation.NewFanOut(federation.FanOutDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Outbox: outbox,
		Statuses: federation.ContentStatuses{Service: a.Content, Interactions: a.Interactions},
		Accounts: a.Accounts, Graph: a.Graph, Parts: a.Statuses, Follows: a.Graph,
		Blocks:  a.Graph,
		Reports: a.Moderation,
		Domains: a.ModerationPolicy,
		Clock:   a.Clock, IDs: a.IDs, Host: cfg.Host, AccountDomain: cfg.AccountDomain, Logger: a.Logger,
		Suspensions: a.ModerationPolicy,
	})
	a.FanOut.RegisterJobs(a.Handlers)
}

// newObjects builds the use case behind the ActivityPub objects and collections of local
// statuses (federation.md §10). Signed GETs are verified with the HTTP signature adapter and
// the keys that RemoteActors resolves, so setUpRemoteActors runs first.
func (a *App) newObjects(cfg *config.Config) *federation.Objects {
	return federation.NewObjects(federation.ObjectsDeps{
		Statuses: a.Content, Accounts: a.Accounts, Graph: a.Graph, Timeline: a.Timeline, Reader: a.Statuses,
		Signatures: federation.NewSignatureVerifier(httpsig.New(), a.RemoteActors, a.KV, a.Clock),
		Domains:    a.ModerationPolicy,
		// Authorized fetch (FR-408) is the instance setting, read per request.
		Settings: a.Instance,
		Host:     cfg.Host, AccountDomain: cfg.AccountDomain, Logger: a.Logger,
	})
}

// timelineLanguages returns the languages that a user's timelines show (FR-208); a user without
// preferences sees every language.
func (a *App) timelineLanguages(ctx context.Context, userID string) ([]string, error) {
	p, err := a.Identity.Preferences(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p.TimelineLanguages, nil
}

// setUpWeb builds the web client's handler (ADR-0028), registers its assets and manifest, and
// sets a.page, which serves its HTML to the requests that do not ask for ActivityPub.
func (a *App) setUpWeb(opts AppOptions) error {
	if opts.WebBuild == nil {
		opts.WebBuild = web.Embedded()
	}
	h, err := web.New(web.Options{
		Build: opts.WebBuild, DevViteOrigin: opts.DevViteOrigin,
		Host: a.Config.Host, AccountDomain: a.Config.AccountDomain, WebOrigin: a.Config.WebOrigin,
		// The instance is named after its host until the instance settings hold a name.
		InstanceName: a.Config.Host,
		// The HTML is the same for every visitor, so only the instance default applies.
		DefaultLocale: func(ctx context.Context) string { return a.Locales.Locale(ctx, "") },
		// Profile and post pages carry link previews of what an anonymous visitor may see.
		Previews: a.Previews, Messages: a.Messages,
		// The instance's preview image is read per request, so that a change applies at once.
		DefaultImage: a.previewImage,
		NotFound:     func(w http.ResponseWriter, r *http.Request) { a.writeStatusProblem(w, r, http.StatusNotFound) },
		Logger:       a.Logger,
	})
	if err != nil {
		return err
	}
	h.Register(a)
	a.webStylesheet = h.Stylesheet()
	a.page = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The same URL may be an ActivityPub document, so every answer depends on Accept.
		if !slices.Contains(w.Header().Values("Vary"), "Accept") {
			w.Header().Add("Vary", "Accept")
		}
		if activitypub.AsksForActivityPub(r) {
			a.writeStatusProblem(w, r, http.StatusNotFound)
			return
		}
		h.ServeHTTP(w, r)
	})
	return nil
}

// setMedia builds the blob store, the media processor, and the media use cases, and registers
// their jobs and schedules.
func (a *App) setMedia(cfg *config.Config, repo content.MediaRepository) error {
	switch cfg.Media {
	case "native":
		a.MediaProcessor = native.New(native.Options{})
	default:
		return fmt.Errorf("media %q is not implemented yet", cfg.Media)
	}
	a.Media = content.NewMediaService(content.MediaDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Blobs: a.Blobs, Processor: a.MediaProcessor,
		Outbox: a.Outbox, Jobs: a.Jobs, Clock: a.Clock, IDs: a.IDs,
		Limits: media.Limits{
			MaxBytes: cfg.MediaMaxImageBytes, MaxSide: cfg.MediaMaxImageSide, MaxPixels: cfg.MediaMaxImagePixels,
		},
		UnattachedTTL: cfg.MediaUnattachedTTL,
		// Avatars and headers refer to media files from the account module.
		Referrers: []content.MediaReferrer{a.Accounts, a.Instance},
		Proxy:     a.MediaProxy,
		Logger:    a.Logger,
	})
	a.Media.RegisterJobs(a.Handlers, a.Scheduler)
	return nil
}

// setBlobs builds the blob store, which the media proxy's cache and the media use cases share.
func (a *App) setBlobs(cfg *config.Config) error {
	switch cfg.Blob {
	case "fs":
		store, err := blobfs.New(blobfs.Options{Dir: cfg.BlobFSDir, BaseURL: cfg.ServerOrigin()})
		if err != nil {
			return err
		}
		a.Blobs, a.closeBlobs = store, store.Close
	default:
		return fmt.Errorf("blob %q is not implemented yet", cfg.Blob)
	}
	return nil
}

// mediaProxyKeyInfo is the HKDF info that derives the media proxy's URL signing key from a KEK.
const mediaProxyKeyInfo = "my media proxy url signature v1"

// newMediaProxy builds the media proxy. Its HMAC keys are derived from the key-encryption keys
// with HKDF-SHA256, so that it needs no secret of its own: the current KEK signs, and the
// previous ones still verify the URLs signed before a rotation.
func newMediaProxy(cfg *config.Config, repo content.MediaRepository, a *App) (*content.MediaProxy, error) {
	var keys [][]byte
	for _, kek := range append([][]byte{cfg.KEK}, cfg.PreviousKEKs...) {
		if len(kek) == 0 {
			continue
		}
		k, err := hkdf.Key(sha256.New, kek, nil, mediaProxyKeyInfo, sha256.Size)
		if err != nil {
			return nil, fmt.Errorf("derive the media proxy key: %w", err)
		}
		keys = append(keys, k)
	}
	p, err := content.NewMediaProxy(content.ProxyDeps{
		Repository: repo, HTTP: a.HTTPClient, Accounts: a.Accounts,
		Policy: a.ModerationPolicy,
		Settings: func(ctx context.Context) (content.ProxySettings, error) {
			s, err := a.Instance.Settings(ctx)
			return content.ProxySettings{
				Policy: content.RemoteMediaPolicy(s.RemoteMediaPolicy()), CacheTTL: s.RemoteMediaCacheTTL(),
			}, err
		},
		// The account module's avatars and headers are what the avatars policy keeps.
		UnitOfWork: a.UnitOfWork, Blobs: a.Blobs, Clock: a.Clock, ProfileImages: a.Accounts,
		MaxCacheSize: cfg.RemoteMediaCacheMaxBytes,
		Keys:         keys, BaseURL: cfg.ServerOrigin(), Logger: a.Logger,
	})
	if err != nil {
		return nil, err
	}
	p.RegisterJobs(a.Handlers, a.Scheduler)
	return p, nil
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

// previewImage returns the instance's link preview image (the preview_image setting) for a
// preview in language lang, or nil when it is unset or its media file is not a processed
// image, which leaves the web client's built-in image.
func (a *App) previewImage(ctx context.Context, lang string) (*web.Image, error) {
	setting, err := a.Instance.PreviewImage(ctx, lang)
	if err != nil || setting.MediaID == "" {
		return nil, err
	}
	img, err := a.Previews.Image(ctx, setting.MediaID, setting.Alt)
	if err != nil || img == nil {
		return nil, err
	}
	return &web.Image{URL: img.URL, Width: img.Width, Height: img.Height, Alt: img.Alt}, nil
}

// instanceMedia checks the media files of the instance settings with the media use cases.
type instanceMedia struct{ a *App }

func (m instanceMedia) CheckImage(ctx context.Context, mediaID string) error {
	if m.a.Media == nil {
		return errors.New("media files are not set up")
	}
	return m.a.Media.CheckImage(ctx, mediaID)
}

func (a *App) setDBQueue(q *queuedb.Queue) {
	a.Jobs, a.Outbox, a.DeadJobs, a.source = q, q, q, q
}

func newMailer(cfg *config.Config, logger *slog.Logger) (kernel.Mailer, error) {
	switch cfg.Mail {
	case "", "none":
		return mailnone.New(logger), nil
	case "smtp":
		return mailsmtp.New(mailsmtp.Options{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.MailFrom, HelloName: hostOnly(cfg.Host),
		})
	default:
		return nil, fmt.Errorf("mail %q is not implemented yet", cfg.Mail)
	}
}

// newHTTPClient returns the SSRF-guarded HTTPClient. http_allow_private_networks, which the
// configuration accepts only in the development profile, turns the guard off with a warning.
func newHTTPClient(cfg *config.Config, version string, logger *slog.Logger) kernel.HTTPClient {
	opts := httpclient.Options{UserAgent: fmt.Sprintf("my/%s (+https://%s/)", version, cfg.Host)}
	if cfg.HTTPAllowPrivateNetworks {
		logger.Warn("http_allow_private_networks is on: outbound HTTP can reach loopback and private networks; never use it in production")
		opts.AllowPrivateNetworks, opts.AllowHTTP, opts.AnyPort = true, true, true
	}
	return httpclient.New(opts)
}

// hostOnly returns host without a port.
func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
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
	if a.Streams != nil {
		a.Streams.Close()
	}
	if a.closeBlobs != nil {
		_ = a.closeBlobs()
	}
	if a.closer == nil {
		return nil
	}
	return a.closer()
}
