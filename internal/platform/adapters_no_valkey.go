// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_valkey

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/platform/config"
)

// newValkeyKV reports that the build tag no_valkey left the Valkey adapters out.
func (a *App) newValkeyKV(context.Context, *config.Config) (kernel.KeyValueStore, error) {
	return nil, config.ExcludedError("kv", "valkey", "no_valkey")
}

// newValkeyBus reports that the build tag no_valkey left the Valkey adapters out.
func (a *App) newValkeyBus(context.Context, *config.Config) (stream.Bus, error) {
	return nil, config.ExcludedError("stream", "valkey", "no_valkey")
}
