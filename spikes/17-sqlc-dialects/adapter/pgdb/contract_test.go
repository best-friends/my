// SPDX-License-Identifier: AGPL-3.0-only

package pgdb_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/adapter/pgdb"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/contract"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// postgresURL returns TEST_POSTGRES_URL, or starts a throwaway container with
// testcontainers-go when docker is reachable. Otherwise the test is skipped.
func postgresURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("TEST_POSTGRES_URL"); u != "" {
		return u
	}
	if os.Getenv("TEST_POSTGRES_DOCKER") == "0" {
		t.Skip("TEST_POSTGRES_URL unset and TEST_POSTGRES_DOCKER=0")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("TEST_POSTGRES_URL unset and docker unavailable (%v); skipping PostgreSQL contract tests", err)
	}
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("spike"), tcpostgres.WithUsername("spike"), tcpostgres.WithPassword("spike"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	u, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// factory isolates every subtest in its own schema via search_path.
func factory(baseURL string, iso pgx.TxIsoLevel) contract.Factory {
	return func(t *testing.T, p txretry.Policy) store.UnitOfWork {
		ctx := t.Context()
		schemaName := "t_" + strings.ReplaceAll(uuid.NewV4().String(), "-", "")
		admin, err := pgx.Connect(ctx, baseURL)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = admin.Close(context.Background()) }()
		if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
			t.Fatal(err)
		}
		cfg, err := pgxpool.ParseConfig(baseURL)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schemaName
		cfg.MaxConns = 20
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Close()
			c, err := pgx.Connect(context.Background(), baseURL)
			if err == nil {
				_, _ = c.Exec(context.Background(), "DROP SCHEMA "+schemaName+" CASCADE")
				_ = c.Close(context.Background())
			}
		})
		db := pgdb.New(pool)
		db.IsoLevel = iso
		db.Policy = p
		if err := db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
		return db
	}
}

func TestContract(t *testing.T) {
	u := postgresURL(t)
	start := time.Now()
	t.Run("serializable", func(t *testing.T) { contract.Run(t, factory(u, pgx.Serializable)) })
	t.Run("read-committed", func(t *testing.T) { contract.Run(t, factory(u, pgx.ReadCommitted)) })
	t.Logf("postgres contract suite took %v", time.Since(start))
}
