// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// Retention of the sign-in and registration origins (ADR-0038, data-model §4.1, §8).
const (
	// JobPurgeOrigins deletes the session and registration origins past their retention; no
	// payload.
	JobPurgeOrigins = "identity.purge_origins"
	// OriginPurgeInterval is the period of the purge's schedule.
	OriginPurgeInterval = time.Hour
	// DefaultSessionOriginRetention is how long the IP address and User-Agent of a sign-in are
	// kept after the session started; DefaultRegistrationOriginRetention how long the IP address
	// of a registration is kept after it.
	DefaultSessionOriginRetention      = 180 * 24 * time.Hour
	DefaultRegistrationOriginRetention = 180 * 24 * time.Hour
	// OriginPurgeBatch is the number of rows one statement deletes; originPurgeMaxBatches bounds
	// the batches of one table in one run, and the next run continues.
	OriginPurgeBatch      = 1000
	originPurgeMaxBatches = 20
)

// OriginRepository is the part of the identity module's persistence that records where sign-ins
// and registrations came from. The database adapters implement it and pass
// testkit.AccountRepositoryContract and testkit.RegistrationRepositoryContract.
type OriginRepository interface {
	InsertSessionOrigin(ctx context.Context, tx uow.Tx, o identity.SessionOrigin) error
	// InsertRegistrationOrigin records the origin of a registration; the user has none yet.
	InsertRegistrationOrigin(ctx context.Context, tx uow.Tx, o identity.RegistrationOrigin) error
	// FindSessionOrigin returns the origin of a session.
	FindSessionOrigin(ctx context.Context, sessionID string) (identity.SessionOrigin, error)
	// FindRegistrationOrigin returns the origin of a user's registration.
	FindRegistrationOrigin(ctx context.Context, userID string) (identity.RegistrationOrigin, error)
	// DeleteSessionOriginsBefore deletes inside tx at most limit session origins created before
	// t, the oldest first, in one statement, and returns how many it deleted.
	DeleteSessionOriginsBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error)
	// DeleteRegistrationOriginsBefore deletes inside tx at most limit registration origins
	// recorded before t, the oldest first, in one statement, and returns how many it deleted.
	DeleteRegistrationOriginsBefore(ctx context.Context, tx uow.Tx, t time.Time, limit int) (int64, error)
}

// normalizeIP returns the canonical form of a client address (an IPv4-mapped IPv6 address as
// IPv4, without a zone), so that the same address always compares equal, or "" when ip is not
// an IP address: the origin is then recorded without it.
func normalizeIP(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	return a.Unmap().WithZone("").String()
}

// OriginPurgeDeps are the ports of the origin purge.
type OriginPurgeDeps struct {
	UnitOfWork uow.UnitOfWork
	Repository OriginRepository
	Clock      kernel.Clock
	// SessionPeriod is how long a session origin is kept; zero means
	// DefaultSessionOriginRetention.
	SessionPeriod time.Duration
	// RegistrationPeriod is how long a registration origin is kept; zero means
	// DefaultRegistrationOriginRetention.
	RegistrationPeriod time.Duration
	// Batch is the number of rows per statement; zero means OriginPurgeBatch.
	Batch int
}

// OriginPurger runs JobPurgeOrigins: it deletes the session origins older than SessionPeriod
// and the registration origins older than RegistrationPeriod (ADR-0038). The sessions and users
// stay. Each statement deletes a bounded batch in a transaction of its own.
type OriginPurger struct{ d OriginPurgeDeps }

// NewOriginPurger returns the origin purge on d.
func NewOriginPurger(d OriginPurgeDeps) *OriginPurger {
	if d.SessionPeriod <= 0 {
		d.SessionPeriod = DefaultSessionOriginRetention
	}
	if d.RegistrationPeriod <= 0 {
		d.RegistrationPeriod = DefaultRegistrationOriginRetention
	}
	if d.Batch <= 0 {
		d.Batch = OriginPurgeBatch
	}
	return &OriginPurger{d: d}
}

// RegisterJobs registers the JobPurgeOrigins handler and its hourly schedule.
func (p *OriginPurger) RegisterJobs(h *queue.JobHandlerRegistry, s *queue.Scheduler) {
	h.Register(JobPurgeOrigins, func(ctx context.Context, _ queue.Job) queue.Result {
		if _, err := p.Run(ctx); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	s.Add(queue.Schedule{Name: JobPurgeOrigins, Every: OriginPurgeInterval, Job: queue.NewJob{Type: JobPurgeOrigins}})
}

// Run deletes the origins past their retention, at most originPurgeMaxBatches batches per
// table, and returns how many rows it deleted. It is safe to repeat.
func (p *OriginPurger) Run(ctx context.Context) (int64, error) {
	now := p.d.Clock.Now()
	r := p.d.Repository
	steps := []struct {
		name   string
		before time.Time
		del    func(context.Context, uow.Tx, time.Time, int) (int64, error)
	}{
		{"session origins", now.Add(-p.d.SessionPeriod), r.DeleteSessionOriginsBefore},
		{"registration origins", now.Add(-p.d.RegistrationPeriod), r.DeleteRegistrationOriginsBefore},
	}
	var total int64
	for _, s := range steps {
		for range originPurgeMaxBatches {
			var n int64
			err := p.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
				var err error
				n, err = s.del(ctx, tx, s.before, p.d.Batch)
				return err
			})
			if err != nil {
				return total, fmt.Errorf("identity: purge %s: %w", s.name, err)
			}
			total += n
			if n < int64(p.d.Batch) {
				break
			}
		}
	}
	return total, nil
}
