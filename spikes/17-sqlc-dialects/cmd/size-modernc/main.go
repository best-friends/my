// SPDX-License-Identifier: AGPL-3.0-only

// Command size-modernc measures the CGO_ENABLED=0 binary size with only modernc.org/sqlite linked.
package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	var v string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&v); err != nil {
		panic(err)
	}
	fmt.Println("modernc", v)
}
