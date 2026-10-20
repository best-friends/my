// SPDX-License-Identifier: AGPL-3.0-only

package sns

import "time"

// SetReceiveWaits shortens the waits of the receive loop until the returned function restores
// them.
func SetReceiveWaits(receive, broken, retry time.Duration) (restore func()) {
	r, b, rmin, rmax := receiveTimeout, brokenAfter, retryMin, retryMax
	receiveTimeout, brokenAfter, retryMin, retryMax = receive, broken, retry, retry
	return func() { receiveTimeout, brokenAfter, retryMin, retryMax = r, b, rmin, rmax }
}
