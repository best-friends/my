// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/best-friends/my/schema"
)

// The environment variables that the PostgreSQL tests of internal/adapter/db/postgres read:
// TEST_POSTGRES_URL (an existing server whose user may create databases; otherwise a
// testcontainers-go container), TEST_POSTGRES_REQUIRED (fail instead of skip without a server;
// CI sets it), and TEST_POSTGRES_IMAGE.
const (
	envPostgresURL      = "TEST_POSTGRES_URL"
	envPostgresRequired = "TEST_POSTGRES_REQUIRED"
	envPostgresImage    = "TEST_POSTGRES_IMAGE"
	defaultPostgresImg  = "postgres:18-alpine"
)

// postgresServer finds or starts the PostgreSQL server once, for the few tests of this package
// that run on both dialects; most tests use SQLite and never start it.
var postgresServer struct {
	once      sync.Once
	url       string
	skip      string
	err       error
	container testcontainers.Container
}

func TestMain(m *testing.M) {
	code := m.Run()
	if c := postgresServer.container; c != nil {
		_ = testcontainers.TerminateContainer(c)
	}
	os.Exit(code)
}

func findPostgres() {
	s := &postgresServer
	if u := os.Getenv(envPostgresURL); u != "" {
		s.url = u
		return
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		s.skip = fmt.Sprintf("%s is unset and Docker is unavailable (%v)", envPostgresURL, err)
		if os.Getenv(envPostgresRequired) != "" {
			s.err = fmt.Errorf("%s, but %s is set", s.skip, envPostgresRequired)
		}
		return
	}
	image := os.Getenv(envPostgresImage)
	if image == "" {
		image = defaultPostgresImg
	}
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("my"), tcpostgres.WithUsername("my"), tcpostgres.WithPassword("my"),
		tcpostgres.BasicWaitStrategies(),
	)
	if c != nil {
		s.container = c
	}
	if err != nil {
		s.err = fmt.Errorf("start container %s: %w", image, err)
		return
	}
	s.url, s.err = c.ConnectionString(ctx, "sslmode=disable")
}

// newPostgresDatabase creates a new database on the test server with the bootstrap SQL and the
// product schema applied, and returns its URL. It skips t when no server is available and
// drops the database when t ends.
func newPostgresDatabase(t *testing.T) string {
	t.Helper()
	s := &postgresServer
	s.once.Do(findPostgres)
	if s.err != nil {
		t.Fatalf("postgres: %v", s.err)
	}
	if s.url == "" {
		t.Skip(s.skip)
	}
	ctx := t.Context()
	admin, err := pgx.Connect(ctx, s.url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = admin.Close(context.WithoutCancel(ctx)) }()
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	name := "test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if c, err := pgx.Connect(ctx, s.url); err == nil {
			_, _ = c.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
			_ = c.Close(ctx)
		}
	})
	u, err := url.Parse(s.url)
	if err != nil {
		t.Fatalf("%s must be a postgres:// URL: %v", envPostgresURL, err)
	}
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	for _, ddl := range []string{schema.PostgresBootstrap, schema.Postgres} {
		if _, err := conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	}
	return u.String()
}
