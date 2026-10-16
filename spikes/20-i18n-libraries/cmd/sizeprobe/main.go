// SPDX-License-Identifier: AGPL-3.0-only

// Command sizeprobe measures the binary-size cost of each Go engine:
//
//	CGO_ENABLED=0 go build -ldflags=-s -w -tags <engine> -o /tmp/x ./cmd/sizeprobe
//
// with <engine> = none | kaptinlin | gotnospirit | fullpipe.
package main

import (
	"fmt"
	"os"
)

var format func(locale, source string, args map[string]any) (string, error)

func main() {
	if format == nil {
		fmt.Println("no engine")
		return
	}
	out, err := format("en", "{n, plural, one {# item} other {# items}}", map[string]any{"n": 2})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(out)
}
