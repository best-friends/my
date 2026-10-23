// SPDX-License-Identifier: AGPL-3.0-only

package queue_test

import "fmt"

// testID returns a canonical UUIDv7 whose last digits are n, so ids sort in the order of n.
func testID(n int) string {
	return fmt.Sprintf("01890a5d-ac96-7000-8000-%012d", n)
}
