// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/pkg/testkit"
	"github.com/best-friends/my/schema"
)

// Environment variables that control where the tests find PostgreSQL.
const (
	// envURL points the tests at an existing server, as a postgres:// URL whose user may create
	// databases. Without it, the tests start a container with testcontainers-go.
	envURL = "TEST_POSTGRES_URL"
	// envRequired, when non-empty, turns "no PostgreSQL available" into a failure instead of a
	// skip. CI sets it so that the PostgreSQL tests can never be skipped silently.
	envRequired = "TEST_POSTGRES_REQUIRED"
	// envImage overrides the container image; it defaults to the one `make db-up` uses.
	envImage     = "TEST_POSTGRES_IMAGE"
	defaultImage = "postgres:18-alpine"
)

var (
	// serverURL is the URL of the test server, or empty when there is none.
	serverURL string
	// skipReason says why serverURL is empty.
	skipReason string
)

func TestMain(m *testing.M) {
	code, cleanup := setUpServer()
	if code == 0 {
		code = m.Run()
	}
	cleanup()
	os.Exit(code)
}

// setUpServer finds or starts the PostgreSQL server for the tests.
func setUpServer() (code int, cleanup func()) {
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

// newTestDB creates a new database on the test server, applies the bootstrap SQL and the product
// schema (see package schema for why executing the files matches an Atlas apply), and opens it.
func newTestDB(t *testing.T, opts uow.Options) *DB {
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
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("%s must be a postgres:// URL: %v", envURL, err)
	}
	u.Path = "/" + name

	db, err := Open(ctx, u.String(), opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		ctx := context.Background()
		if c, err := pgx.Connect(ctx, serverURL); err == nil {
			_, _ = c.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
			_ = c.Close(ctx)
		}
	})
	for _, ddl := range []string{schema.PostgresBootstrap, schema.Postgres} {
		if _, err := db.pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	}
	return db
}

// The UnitOfWork contract uses users as its probe table: a row per key, keyed by id.

func insertProbe(ctx context.Context, q *postgresgen.Queries, key string) error {
	return q.InsertUser(ctx, postgresgen.InsertUserParams{ID: key, CreatedAt: time.UnixMilli(0)})
}

func probeKeys(ctx context.Context, db *DB) ([]string, error) {
	rows, err := db.pool.Query(ctx, "SELECT id FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if keys == nil {
		keys = []string{}
	}
	return keys, err
}

func TestAccountRepositoryContract(t *testing.T) {
	testkit.AccountRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.AccountHarness {
		db := newTestDB(t, opts)
		return testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()}
	})
}

func TestMediaRepositoryContract(t *testing.T) {
	testkit.MediaRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.MediaHarness {
		db := newTestDB(t, opts)
		return testkit.MediaHarness{UnitOfWork: db, Media: db.MediaRepository(), Statuses: db.StatusRepository()}
	})
}

func TestStatusRepositoryContract(t *testing.T) {
	testkit.StatusRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.StatusHarness {
		db := newTestDB(t, opts)
		return testkit.StatusHarness{UnitOfWork: db, Statuses: db.StatusRepository()}
	})
}

func TestInteractionRepositoryContract(t *testing.T) {
	testkit.InteractionRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.InteractionHarness {
		db := newTestDB(t, opts)
		return testkit.InteractionHarness{UnitOfWork: db, Statuses: db.StatusRepository(), Interactions: db.InteractionRepository()}
	})
}

func TestTimelineContract(t *testing.T) {
	testkit.TimelineContract(t, func(t *testing.T, opts uow.Options) testkit.TimelineHarness {
		db := newTestDB(t, opts)
		return testkit.TimelineHarness{
			UnitOfWork: db, Statuses: db.StatusRepository(), Interactions: db.InteractionRepository(),
			Graph: db.GraphRepository(), Timeline: db.TimelineRepository(),
		}
	})
}

func TestNotificationContract(t *testing.T) {
	testkit.NotificationContract(t, func(t *testing.T, opts uow.Options) testkit.NotificationHarness {
		db := newTestDB(t, opts)
		return testkit.NotificationHarness{
			AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
			Graph:          db.GraphRepository(),
			Statuses:       db.StatusRepository(),
			Interactions:   db.InteractionRepository(),
			Notifications:  db.NotificationRepository(),
		}
	})
}

func TestFollowRepositoryContract(t *testing.T) {
	testkit.FollowRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.FollowHarness {
		db := newTestDB(t, opts)
		return testkit.FollowHarness{
			AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
			Graph:          db.GraphRepository(),
		}
	})
}

func TestPostingContract(t *testing.T) {
	testkit.PostingContract(t, func(t *testing.T, opts uow.Options) testkit.PostingHarness {
		db := newTestDB(t, opts)
		return testkit.PostingHarness{
			AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
			Graph:          db.GraphRepository(),
			Instance:       db.InstanceRepository(),
			Statuses:       db.StatusRepository(),
			Media:          db.MediaRepository(),
		}
	})
}

func TestBlockRepositoryContract(t *testing.T) {
	testkit.BlockRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.FollowHarness {
		db := newTestDB(t, opts)
		return testkit.FollowHarness{
			AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
			Graph:          db.GraphRepository(),
		}
	})
}

func TestPreferencesRepositoryContract(t *testing.T) {
	testkit.PreferencesRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.AccountHarness {
		db := newTestDB(t, opts)
		return testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()}
	})
}

func TestRegistrationRepositoryContract(t *testing.T) {
	testkit.RegistrationRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.AccountHarness {
		db := newTestDB(t, opts)
		return testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()}
	})
}

func TestInstanceRepositoryContract(t *testing.T) {
	testkit.InstanceRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.InstanceHarness {
		db := newTestDB(t, opts)
		return testkit.InstanceHarness{UnitOfWork: db, Instance: db.InstanceRepository()}
	})
}

func TestFederationRepositoryContract(t *testing.T) {
	testkit.FederationRepositoryContract(t, func(t *testing.T, opts uow.Options) testkit.FederationHarness {
		db := newTestDB(t, opts)
		return testkit.FederationHarness{UnitOfWork: db, Federation: db.FederationRepository()}
	})
}

func TestInboxContract(t *testing.T) {
	testkit.InboxContract(t, func(t *testing.T, opts uow.Options) testkit.InboxHarness {
		db := newTestDB(t, opts)
		feds := db.FederationRepository()
		return testkit.InboxHarness{UnitOfWork: db, Inbox: feds, Federation: feds, Instances: feds, Accounts: db.AccountRepository()}
	})
}

func TestInboxProcessingContract(t *testing.T) {
	testkit.InboxProcessingContract(t, func(t *testing.T, opts uow.Options) testkit.InboxHarness {
		db := newTestDB(t, opts)
		feds := db.FederationRepository()
		return testkit.InboxHarness{
			UnitOfWork: db, Inbox: feds, Federation: feds, Instances: feds, Accounts: db.AccountRepository(), Statuses: db.StatusRepository(),
			Graph: db.GraphRepository(), Moderation: db.ModerationRepository(),
		}
	})
}

func TestModerationContract(t *testing.T) {
	testkit.ModerationContract(t, func(t *testing.T, opts uow.Options) testkit.ModerationHarness {
		db := newTestDB(t, opts)
		return testkit.ModerationHarness{UnitOfWork: db, Moderation: db.ModerationRepository()}
	})
}

func TestRemoteStatusContract(t *testing.T) {
	testkit.RemoteStatusContract(t, func(t *testing.T, opts uow.Options) testkit.RemoteStatusHarness {
		db := newTestDB(t, opts)
		return testkit.RemoteStatusHarness{
			AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
			Graph:          db.GraphRepository(),
			Statuses:       db.StatusRepository(),
			Media:          db.MediaRepository(),
			Timeline:       db.TimelineRepository(),
			Notifications:  db.NotificationRepository(),
			Inbox:          db.FederationRepository(),
		}
	})
}

func TestRetentionContract(t *testing.T) {
	testkit.RetentionContract(t, func(t *testing.T, opts uow.Options) testkit.RetentionHarness {
		db := newTestDB(t, opts)
		return testkit.RetentionHarness{
			RemoteStatusHarness: testkit.RemoteStatusHarness{
				AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
				Graph:          db.GraphRepository(),
				Statuses:       db.StatusRepository(),
				Media:          db.MediaRepository(),
				Timeline:       db.TimelineRepository(),
				Notifications:  db.NotificationRepository(),
				Inbox:          db.FederationRepository(),
			},
			Interactions: db.InteractionRepository(),
			Federation:   db.FederationRepository(),
			Retention:    db.StatusRepository(),
			Purges:       db.FederationRepository(),
			Profiles:     db.AccountRepository(),
		}
	})
}

func TestRemoteFollowContract(t *testing.T) {
	testkit.RemoteFollowContract(t, func(t *testing.T, opts uow.Options) testkit.RemoteFollowHarness {
		db := newTestDB(t, opts)
		feds := db.FederationRepository()
		return testkit.RemoteFollowHarness{
			FollowHarness: testkit.FollowHarness{
				AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
				Graph:          db.GraphRepository(),
			},
			Notifications: db.NotificationRepository(),
			Federation:    feds,
			Inbox:         feds,
		}
	})
}

func TestRemoteActorContract(t *testing.T) {
	testkit.RemoteActorContract(t, func(t *testing.T, opts uow.Options) testkit.RemoteActorHarness {
		db := newTestDB(t, opts)
		feds := db.FederationRepository()
		return testkit.RemoteActorHarness{UnitOfWork: db, Accounts: db.AccountRepository(), Federation: feds, Instances: feds}
	})
}

func TestUnitOfWorkContract(t *testing.T) {
	testkit.UnitOfWorkContract(t, func(t *testing.T, opts uow.Options) testkit.UnitOfWorkHarness {
		db := newTestDB(t, opts)
		return testkit.UnitOfWorkHarness{
			UnitOfWork: db,
			Insert: func(ctx context.Context, tx uow.Tx, name string) error {
				q, err := queries(tx)
				if err != nil {
					return err
				}
				return insertProbe(ctx, q, name)
			},
			Keys: func(ctx context.Context) ([]string, error) { return probeKeys(ctx, db) },
		}
	})
}

func TestTransactionsAreReadCommitted(t *testing.T) {
	db := newTestDB(t, uow.Options{})
	err := db.Tx(t.Context(), func(tx uow.Tx) error {
		pgTx, err := txOf(tx)
		if err != nil {
			return err
		}
		var level string
		if err := pgTx.QueryRow(t.Context(), "SHOW transaction_isolation").Scan(&level); err != nil {
			return err
		}
		if level != "read committed" {
			t.Errorf("transaction_isolation = %q, want read committed", level)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx = %v", err)
	}
}

// TestRetriesDeadlocks makes two transactions lock two rows in opposite orders. PostgreSQL
// aborts one of them with deadlock_detected; the UnitOfWork must retry it to success.
func TestRetriesDeadlocks(t *testing.T) {
	var (
		mu      sync.Mutex
		retries []error
	)
	db := newTestDB(t, uow.Options{Retry: uow.RetryPolicy{OnRetry: func(_ int, err error) {
		mu.Lock()
		defer mu.Unlock()
		retries = append(retries, err)
	}}})
	setup := postgresgen.New(db.pool)
	for _, name := range []string{"x", "y"} {
		if err := insertProbe(t.Context(), setup, name); err != nil {
			t.Fatal(err)
		}
	}

	var bothLocked sync.WaitGroup
	bothLocked.Add(2)
	lock := func(tx uow.Tx, key string) error {
		pgTx, err := txOf(tx)
		if err != nil {
			return err
		}
		_, err = pgTx.Exec(t.Context(), "SELECT 1 FROM users WHERE id = $1 FOR UPDATE", key)
		return err
	}
	lockBoth := func(first, second string) error {
		attempt := 0
		return db.Tx(t.Context(), func(tx uow.Tx) error {
			attempt++
			if err := lock(tx, first); err != nil {
				return err
			}
			if attempt == 1 {
				bothLocked.Done()
				bothLocked.Wait()
			}
			if err := lock(tx, second); err != nil {
				return err
			}
			q, err := queries(tx)
			if err != nil {
				return err
			}
			return insertProbe(t.Context(), q, first+"-done")
		})
	}
	errs := make(chan error, 2)
	go func() { errs <- lockBoth("x", "y") }()
	go func() { errs <- lockBoth("y", "x") }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("Tx = %v", err)
		}
	}

	if len(retries) != 1 {
		t.Fatalf("retried %d times, want 1", len(retries))
	}
	var pgErr *pgconn.PgError
	if !errors.Is(retries[0], uow.ErrRetryable) || !errors.As(retries[0], &pgErr) || pgErr.Code != "40P01" {
		t.Errorf("retried error = %v, want a retryable deadlock_detected (40P01)", retries[0])
	}
	got, err := probeKeys(t.Context(), db)
	if err != nil || strings.Join(got, ",") != "x,x-done,y,y-done" {
		t.Errorf("committed keys = %q (%v), want both transactions committed", got, err)
	}
}

func TestIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{&pgconn.PgError{Code: "40001"}, true},
		{&pgconn.PgError{Code: "40P01"}, true},
		{fmt.Errorf("postgres: commit: %w", &pgconn.PgError{Code: "40001"}), true},
		{&pgconn.PgError{Code: "23505"}, false},
		{errors.New("40001"), false},
		{nil, false},
	} {
		if got := isRetryable(tc.err); got != tc.want {
			t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestOpenHidesTheURLInErrors(t *testing.T) {
	_, err := Open(t.Context(), "postgres://user:s3cret@host:notaport/db", uow.Options{})
	if err == nil {
		t.Fatal("Open succeeded with an invalid URL")
	}
	if got := err.Error(); strings.Contains(got, "s3cret") {
		t.Errorf("error %q leaks the password", got)
	}
}

func TestRejectsForeignTransactions(t *testing.T) {
	if _, err := txOf(foreignTx{}); err == nil {
		t.Fatal("txOf accepted a transaction of another adapter")
	}
}

type foreignTx struct{}

func (foreignTx) Emit(context.Context, uow.Event) error { return nil }

func TestReady(t *testing.T) {
	db := newTestDB(t, uow.Options{})
	if err := db.Ready(t.Context()); err != nil {
		t.Fatalf("Ready on an applied schema = %v", err)
	}
	if _, err := db.pool.Exec(t.Context(), "DROP TABLE dead_jobs"); err != nil {
		t.Fatal(err)
	}
	if err := db.Ready(t.Context()); err == nil || !strings.Contains(err.Error(), "dead_jobs") {
		t.Errorf("Ready with a missing table = %v, want an error naming dead_jobs", err)
	}
	db.Close()
	if err := db.Ready(t.Context()); err == nil {
		t.Error("Ready on a closed pool = nil, want an error")
	}
}

func TestRemoteInteractionContract(t *testing.T) {
	testkit.RemoteInteractionContract(t, func(t *testing.T, opts uow.Options) testkit.RemoteInteractionHarness {
		db := newTestDB(t, opts)
		feds := db.FederationRepository()
		return testkit.RemoteInteractionHarness{
			RemoteStatusHarness: testkit.RemoteStatusHarness{
				AccountHarness: testkit.AccountHarness{UnitOfWork: db, Identity: db.IdentityRepository(), Accounts: db.AccountRepository()},
				Graph:          db.GraphRepository(),
				Statuses:       db.StatusRepository(),
				Media:          db.MediaRepository(),
				Timeline:       db.TimelineRepository(),
				Notifications:  db.NotificationRepository(),
				Inbox:          feds,
			},
			Interactions: db.InteractionRepository(),
			Federation:   feds,
		}
	})
}
