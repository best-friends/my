// SPDX-License-Identifier: AGPL-3.0-only

package timeline_test

import "fmt"

// testID returns a canonical UUIDv7 whose last digits are n, so ids sort in the order of n.
func testID(n int) string {
	return fmt.Sprintf("01890a5d-ac96-7000-8000-%012d", n)
}

var (
	viewerID   = testID(901)
	authorID   = testID(902)
	followeeID = testID(903)
	blockerID  = testID(904)
	mutedID    = testID(905)
)
