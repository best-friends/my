// SPDX-License-Identifier: AGPL-3.0-only

// Command agentkit gives coding agents the context of mycroblog/my at a fraction of the tokens of reading
// files: a knowledge graph of the documents, the code, and the GitHub issues, and wrappers for the workflow
// steps (issues, pull requests, CI, checks). Run `agentkit help` for the commands.
package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		var ex exitError
		if errors.As(err, &ex) {
			if ex.msg != "" {
				fmt.Fprintln(os.Stderr, ex.msg)
			}
			os.Exit(ex.code)
		}
		fmt.Fprintln(os.Stderr, "agentkit:", err)
		os.Exit(1)
	}
}

// exitError ends the command with a status other than 1 (pr wait: 3 for a conflict, 124 for a timeout).
type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return e.msg }
