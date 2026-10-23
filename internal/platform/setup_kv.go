// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"fmt"

	kvmemory "github.com/mycroblog/my/internal/adapter/kv/memory"
	"github.com/mycroblog/my/internal/adapter/telemetry/sysmetrics"
	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/internal/platform/config"
)

// setUpKV builds the key-value store that cfg.KV selects, the rate limiter on it, and the
// cleanup of its expired keys. The db store is set with the database, before it.
func (a *App) setUpKV(ctx context.Context, cfg *config.Config) error {
	switch cfg.KV {
	case "db":
		// Set with the database above.
	case "memory":
		a.KV = kvmemory.New(a.Clock)
	case "valkey":
		var err error
		if a.KV, err = a.newValkeyKV(ctx, cfg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("kv %q is not implemented yet", cfg.KV)
	}
	a.KV = sysmetrics.KV(a.KV, a.Meter, "kv."+cfg.KV)
	a.RateLimiter = kernel.NewRateLimiter(a.KV, a.Clock)
	a.registerKVCleanup()
	return nil
}
