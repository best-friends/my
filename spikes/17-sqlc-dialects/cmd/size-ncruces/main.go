// SPDX-License-Identifier: AGPL-3.0-only

// Command size-ncruces measures the CGO_ENABLED=0 binary size with only github.com/ncruces/go-sqlite3 linked.
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	var v string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&v); err != nil {
		panic(err)
	}
	fmt.Println("ncruces", v)
}
