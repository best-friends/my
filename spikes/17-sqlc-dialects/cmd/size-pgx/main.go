// SPDX-License-Identifier: AGPL-3.0-only

// Command size-pgx measures the CGO_ENABLED=0 binary size with only pgx/v5 (pgxpool) linked.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	fmt.Println("pgx", pool != nil, err)
}
