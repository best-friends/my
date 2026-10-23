// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"fmt"

	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/projection"
	"github.com/best-friends/my/internal/app/supporters"
	"github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/app/webhook"
	"github.com/best-friends/my/internal/platform/config"
)

// The adapters that need a client library, a driver, or a cloud SDK are wired in files of
// their own, adapters_<name>.go, which the build tag no_<name> replaces with
// adapters_no_<name>.go (NFR-PORT-4, ADR-0010): there, the constructor reports that the
// adapter is not in this build, and the binary links neither the adapter nor its
// dependencies. Load already rejects the settings that select such an adapter
// (config.ExcludedError), so the replacements are reached only through a Config built
// without Load. docs/reference/build-tags.md lists the tags.

// repositories are the repositories of the database that cfg.DB selects.
type repositories struct {
	identities   identity.Repository
	accounts     account.Repository
	instances    instance.Repository
	follows      graph.Repository
	statuses     content.Repository
	linkCards    content.LinkCardRepository
	quotes       content.QuoteRepository
	interactions content.InteractionRepository
	feeds        timeline.Repository
	notifs       notification.Repository
	pushes       notification.PushRepository
	medias       content.MediaRepository
	emojis       content.CustomEmojiRepository
	feds         federationRepository
	mods         moderation.Repository
	hooks        webhook.Repository
	finance      finance.Repository
	payments     supporters.PaymentRepository
	supporters   supporters.Repository
	retention    retentionRepositories
	projections  projection.Store
}

// openDatabase connects the database that cfg.DB selects and returns its repositories. With
// queue = db and kv = db it also sets the queue and the key-value store on the database.
func (a *App) openDatabase(ctx context.Context, cfg *config.Config) (*repositories, error) {
	queueOpts := queuedb.Options{Clock: a.Clock, IDs: a.IDs, Tracer: a.Tracer}
	switch cfg.DB {
	case "sqlite":
		return a.openSQLite(ctx, cfg, queueOpts)
	case "postgres":
		return a.openPostgres(ctx, cfg, queueOpts)
	default:
		return nil, fmt.Errorf("db %q is not implemented yet", cfg.DB)
	}
}

// valkeyServer is the connection to valkey_url that the adapters configured with it share.
type valkeyServer interface {
	// Ping reports whether the server answers.
	Ping(ctx context.Context) error
	Close()
}
