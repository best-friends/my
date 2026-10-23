// SPDX-License-Identifier: AGPL-3.0-only

package notification

import "fmt"

// testID returns the canonical UUIDv7 with the sequence number n.
func testID(n int) string { return fmt.Sprintf("01890a5d-ac96-7000-8000-%012d", n) }

var (
	idN = testID(1)
	idA = testID(2)
	idB = testID(3)
	idR = testID(4)
	idM = testID(5)
	idS = testID(6)
	idU = testID(7)
	idP = testID(8)
	idT = testID(9)
	idF = testID(10)
)
