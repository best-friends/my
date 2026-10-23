// SPDX-License-Identifier: AGPL-3.0-only

package sysmetrics

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/mycroblog/my/internal/app/kernel"
)

// ConnCounter counts the connections that a dialer opens: Track adds 1 to an UpDownCounter
// and the connection's first Close subtracts it. When it counts reconnects, a connection
// opened after an earlier one closed is also a reconnect.
type ConnCounter struct {
	open       kernel.UpDownCounter
	reconnects kernel.Counter // nil: not counted
	labels     kernel.Labels
	everClosed atomic.Bool
}

// NewConnCounter returns the counter of the long-lived connections of adapter (such as
// valkey) on AdapterConnections and AdapterReconnects, or nil when m records nothing.
func NewConnCounter(m kernel.Meter, adapter string) *ConnCounter {
	if kernel.IsNop(m) {
		return nil
	}
	return &ConnCounter{
		open: m.UpDownCounter(AdapterConnections), reconnects: m.Counter(AdapterReconnects),
		labels: kernel.NewLabels(LabelAdapter, adapter),
	}
}

// NewOpenConnCounter returns the counter of the open connections on the UpDownCounter i,
// without reconnects, or nil when m records nothing.
func NewOpenConnCounter(m kernel.Meter, i kernel.Instrument) *ConnCounter {
	if kernel.IsNop(m) {
		return nil
	}
	return &ConnCounter{open: m.UpDownCounter(i)}
}

// Track counts c, which a dial just opened, and returns it wrapped so that its Close counts
// too. A nil ConnCounter returns c.
func (cc *ConnCounter) Track(ctx context.Context, c net.Conn) net.Conn {
	if cc == nil || c == nil {
		return c
	}
	ctx = context.WithoutCancel(ctx)
	if cc.reconnects != nil && cc.everClosed.Load() {
		cc.reconnects.Add(ctx, 1, cc.labels)
	}
	cc.open.Add(ctx, 1, cc.labels)
	return &trackedConn{Conn: c, closed: func() {
		cc.everClosed.Store(true)
		cc.open.Add(ctx, -1, cc.labels)
	}}
}

// trackedConn calls closed on its first Close.
type trackedConn struct {
	net.Conn
	once   sync.Once
	closed func()
}

func (c *trackedConn) Close() error {
	c.once.Do(c.closed)
	return c.Conn.Close()
}
