// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"github.com/mycroblog/my/internal/adapter/telemetry/sysmetrics"
	"github.com/mycroblog/my/internal/app/kernel"
)

// PoolName is the db.client.connection.pool.name of the pool of DB.
const PoolName = "sqlite"

// RegisterMetrics reports the connection pool on m from database/sql's statistics (ADR-0056
// §1). database/sql does not count the acquisitions that gave up, so there is no timeouts
// gauge.
func (d *DB) RegisterMetrics(m kernel.Meter) {
	sysmetrics.RegisterPool(m, PoolName, func() sysmetrics.PoolStats {
		s := d.db.Stats()
		return sysmetrics.PoolStats{
			Idle: int64(s.Idle), Used: int64(s.InUse), Max: int64(s.MaxOpenConnections),
			Waits: s.WaitCount, WaitTime: s.WaitDuration,
		}
	})
}
