// SPDX-License-Identifier: AGPL-3.0-only

// Package none is the kernel.Meter adapter of metrics_exporter = none, the default (ADR-0055
// §2): it records nothing, so that an instance without metrics costs what it did before them.
package none

import "github.com/best-friends/my/internal/app/kernel"

// New returns the Meter that records nothing.
func New() kernel.Meter { return kernel.NopMeter{} }
