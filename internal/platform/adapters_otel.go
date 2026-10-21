// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_otel

package platform

import (
	"context"

	"github.com/best-friends/my/internal/adapter/telemetry/otel"
	"github.com/best-friends/my/internal/platform/config"
)

// setUpOTelMeter sets the Meter of metrics_exporter = prometheus or otlp.
func (a *App) setUpOTelMeter(ctx context.Context, cfg *config.Config, opts AppOptions) error {
	m, err := otel.New(ctx, otel.Options{Exporter: cfg.MetricsExporter, ServiceVersion: opts.Version, Role: string(cfg.Role)})
	if err != nil {
		return err
	}
	a.Meter, a.metricsHandler, a.closeMeter = m, m.Handler(), m.Shutdown
	return nil
}
