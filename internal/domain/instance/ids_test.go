// SPDX-License-Identifier: AGPL-3.0-only

package instance

import "fmt"

// testID returns a canonical UUIDv7 that sorts by n, for ids in tests.
func testID(n int) string {
	return fmt.Sprintf("01890a5d-ac96-7000-8000-%012d", n)
}
