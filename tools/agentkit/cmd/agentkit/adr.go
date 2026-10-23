// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"flag"
	"fmt"

	"github.com/mycroblog/my/tools/agentkit/internal/adr"
	"github.com/mycroblog/my/tools/agentkit/internal/repo"
)

func runADR(r *repo.Repo, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return fmt.Errorf("usage: agentkit adr list [--status Proposed]")
	}
	fs := flag.NewFlagSet("adr list", flag.ContinueOnError)
	status := fs.String("status", "", "only the ADRs with this status (Proposed, Accepted, Superseded, ...)")
	if _, err := parse(fs, args[1:]); err != nil {
		return err
	}
	adrs, err := adr.List(r.Root)
	if err != nil {
		return err
	}
	for _, a := range adr.Filter(adrs, *status) {
		fmt.Printf("%s  %-10s  %s\n", a.Number, a.Status, a.Title)
	}
	return nil
}
