// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_otel

package config

func init() { exclude("no_otel", "metrics_exporter", "prometheus", "otlp") }
