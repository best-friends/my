// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_postgres

package platform

import (
	"context"

	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/platform/config"
)

// openPostgres reports that the build tag no_postgres left the PostgreSQL adapter out.
func (a *App) openPostgres(context.Context, *config.Config, queuedb.Options) (*repositories, error) {
	return nil, config.ExcludedError("db", "postgres", "no_postgres")
}
