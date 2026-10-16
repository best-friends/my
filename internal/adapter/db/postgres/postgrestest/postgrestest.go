// SPDX-License-Identifier: AGPL-3.0-only

// Package postgrestest provides a PostgreSQL server to the tests of adapters that keep their
// tables in the PostgreSQL database, with the same environment variables as the tests of
// package postgres:
//
//   - TEST_POSTGRES_URL points the tests at an existing server, as a postgres:// URL whose user
//     may create databases. Without it, Main starts a container with testcontainers-go.
//   - TEST_POSTGRES_REQUIRED, when non-empty, turns "no PostgreSQL available" into a failure
//     instead of a skip. CI sets it so that the PostgreSQL tests can never be skipped silently.
//   - TEST_POSTGRES_IMAGE overrides the container image (default postgres:18-alpine, the one
//     `make db-up` uses).
//
// Import it only from tests.
package postgrestest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/best-friends/my/schema"
)

const (
	envURL       = "TEST_POSTGRES_URL"
	envRequired  = "TEST_POSTGRES_REQUIRED"
	envImage     = "TEST_POSTGRES_IMAGE"
	defaultImage = "postgres:18-alpine"
)

var (
	serverURL  string
	skipReason string
)

// Main finds or starts the server, runs the tests, stops the server, and exits. Call it from
// TestMain.
func Main(m *testing.M) {
	code, cleanup := setUp()
	if code == 0 {
		code = m.Run()
	}
	cleanup()
	os.Exit(code)
}

func setUp() (code int, cleanup func()) {
	cleanup = func() {}
	if u := os.Getenv(envURL); u != "" {
		serverURL = u
		return 0, cleanup
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		skipReason = fmt.Sprintf("%s is unset and Docker is unavailable (%v)", envURL, err)
		if os.Getenv(envRequired) != "" {
			fmt.Fprintf(os.Stderr, "postgres tests: %s, but %s is set\n", skipReason, envRequired)
			return 1, cleanup
		}
		return 0, cleanup
	}
	image := os.Getenv(envImage)
	if image == "" {
		image = defaultImage
	}
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("my"), tcpostgres.WithUsername("my"), tcpostgres.WithPassword("my"),
		tcpostgres.BasicWaitStrategies(),
	)
	cleanup = func() {
		if c != nil {
			_ = testcontainers.TerminateContainer(c)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres tests: start container %s: %v\n", image, err)
		return 1, cleanup
	}
	if serverURL, err = c.ConnectionString(ctx, "sslmode=disable"); err != nil {
		fmt.Fprintf(os.Stderr, "postgres tests: %v\n", err)
		return 1, cleanup
	}
	return 0, cleanup
}

// NewDatabase creates a new database on the test server, applies the bootstrap SQL and the
// product schema (see package schema for why executing the files matches an Atlas apply), and
// returns its URL. It skips t when no server is available and drops the database when t ends.
func NewDatabase(t *testing.T) string {
	t.Helper()
	if serverURL == "" {
		t.Skip(skipReason)
	}
	ctx := t.Context()
	admin, err := pgx.Connect(ctx, serverURL)
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
		if c, err := pgx.Connect(ctx, serverURL); err == nil {
			_, _ = c.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
			_ = c.Close(ctx)
		}
	})
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("%s must be a postgres:// URL: %v", envURL, err)
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
