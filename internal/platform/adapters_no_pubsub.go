// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_pubsub

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/platform/config"
)

// newPubSubBus reports that the build tag no_pubsub left the Pub/Sub adapter out.
func (a *App) newPubSubBus(context.Context, *config.Config, string) (stream.Bus, func() error, error) {
	return nil, nil, config.ExcludedError("stream", "pubsub", "no_pubsub")
}
