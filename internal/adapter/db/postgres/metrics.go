// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"github.com/mycroblog/my/internal/adapter/telemetry/sysmetrics"
	"github.com/mycroblog/my/internal/app/kernel"
)

// PoolName is the db.client.connection.pool.name of the pool of DB.
const PoolName = "postgres"

// RegisterMetrics reports the connection pool on m from pgxpool's statistics (ADR-0056 §1).
func (d *DB) RegisterMetrics(m kernel.Meter) {
	sysmetrics.RegisterPool(m, PoolName, func() sysmetrics.PoolStats {
		s := d.pool.Stat()
		timeouts := s.CanceledAcquireCount()
		return sysmetrics.PoolStats{
			Idle: int64(s.IdleConns()), Used: int64(s.AcquiredConns()), Max: int64(s.MaxConns()),
			Waits: s.EmptyAcquireCount(), WaitTime: s.EmptyAcquireWaitTime(), Timeouts: &timeouts,
		}
	})
}
