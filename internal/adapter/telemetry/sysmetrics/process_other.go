// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package sysmetrics

import "github.com/mycroblog/my/internal/app/kernel"

// registerOSProcess reports nothing more: the process metrics beyond the uptime are read on
// Linux only, where the server runs in production.
func registerOSProcess(kernel.Meter) {}
