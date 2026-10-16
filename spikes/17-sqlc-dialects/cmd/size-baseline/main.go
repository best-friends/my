// SPDX-License-Identifier: AGPL-3.0-only

// Command size-baseline is the database/sql baseline for binary size comparison.
package main

import (
	"database/sql"
	"fmt"
)

func main() { fmt.Println(sql.Drivers()) }
