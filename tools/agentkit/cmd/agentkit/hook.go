// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/best-friends/my/tools/agentkit/internal/hook"
	"github.com/best-friends/my/tools/agentkit/internal/repo"
	"github.com/best-friends/my/tools/agentkit/internal/stats"
)

func runHook(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agentkit hook pre|post (the hook JSON on stdin)")
	}
	return hook.Run(args[0], os.Stdin, os.Stdout)
}

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	var o stats.Options
	fs.StringVar(&o.Since, "since", "", "first day (2026-10-01)")
	fs.StringVar(&o.Until, "until", "", "day after the last")
	fs.IntVar(&o.Top, "top", 20, "categories to print")
	dirs, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		r, err := repo.Open(".")
		if err != nil {
			return err
		}
		if dirs, err = stats.ProjectDirs(filepath.Dir(r.Common)); err != nil {
			return err
		}
	}
	o.Dirs = dirs
	return stats.Report(os.Stdout, o)
}
