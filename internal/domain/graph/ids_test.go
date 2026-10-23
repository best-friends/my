// SPDX-License-Identifier: AGPL-3.0-only

package graph

import "fmt"

// testID returns the canonical UUIDv7 with the sequence number n.
func testID(n int) string { return fmt.Sprintf("01890a5d-ac96-7000-8000-%012d", n) }

// The ids are in the order of their names.
var (
	idA = testID(1)
	idB = testID(2)
	idC = testID(3)
	idX = testID(4)
	idY = testID(5)
	idF = testID(6)
)
