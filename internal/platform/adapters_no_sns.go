// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_sns

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/platform/config"
)

// newSNSBus reports that the build tag no_sns left the SNS adapter out.
func (a *App) newSNSBus(context.Context, *config.Config, string) (stream.Bus, error) {
	return nil, config.ExcludedError("stream", "sns", "no_sns")
}
