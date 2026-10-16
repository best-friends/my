// SPDX-License-Identifier: AGPL-3.0-only

package db_test

import (
	"context"
	"testing"

	"github.com/best-friends/my/internal/adapter/db/postgres"
	"github.com/best-friends/my/internal/adapter/db/postgres/postgrestest"
	queuedb "github.com/best-friends/my/internal/adapter/queue/db"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/pkg/testkit"
)

func TestMain(m *testing.M) { postgrestest.Main(m) }

func postgresHarness(t *testing.T, clock *testkit.Clock, url string, relay bool) testkit.QueueHarness {
	t.Helper()
	d, err := postgres.Open(t.Context(), url, uow.Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(d.Close)
	opts := queuedb.Options{Clock: clock, IDs: testkit.NewIDGenerator(clock)}
	q := queuedb.NewPostgres(d, opts)
	h := testkit.QueueHarness{Queue: q, Source: q, DeadJobs: q, UnitOfWork: d, Outbox: q}
	if relay {
		o := queuedb.NewPostgresOutbox(d, opts)
		h.Outbox = o
		h.Relay = func(ctx context.Context) error {
			_, err := o.Relay(ctx, q, 0)
			return err
		}
		return h
	}
	h.Restart = func(t *testing.T) testkit.QueueHarness {
		d.Close()
		return postgresHarness(t, clock, url, false)
	}
	return h
}

func TestPostgresQueueContract(t *testing.T) {
	testkit.QueueContract(t, func(t *testing.T, clock *testkit.Clock) testkit.QueueHarness {
		return postgresHarness(t, clock, postgrestest.NewDatabase(t), false)
	})
}

// TestPostgresOutboxContract runs the contract with jobs enqueued in transactions through
// outbox_events and relayed into the queue.
func TestPostgresOutboxContract(t *testing.T) {
	testkit.QueueContract(t, func(t *testing.T, clock *testkit.Clock) testkit.QueueHarness {
		return postgresHarness(t, clock, postgrestest.NewDatabase(t), true)
	})
}
