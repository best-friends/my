// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/best-friends/my/internal/platform/config"
)

// runConfig implements `my config <subcommand>`.
func runConfig(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		configUsage(stderr)
		return 2
	case args[0] == "-h" || args[0] == "-help" || args[0] == "--help":
		configUsage(stderr)
		return 0
	case args[0] != "check":
		_, _ = fmt.Fprintf(stderr, "my config: unknown subcommand %q\n\n", args[0])
		configUsage(stderr)
		return 2
	}
	cfs := flag.NewFlagSet("my config check", flag.ContinueOnError)
	cfs.SetOutput(stderr)
	file := cfs.String("config", "", "TOML config file (default: $"+config.EnvConfigFile+")")
	if err := cfs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if cfs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "my config check: unexpected argument %q\n", cfs.Arg(0))
		return 2
	}

	_, settings, err := config.Load(config.Source{File: *file})
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	for _, s := range settings {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Key, s.Value, s.Origin)
	}
	_ = tw.Flush()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "configuration OK")
	return 0
}

func configUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: my config check [-config <file>]

Validates the configuration from the environment, the TOML file, and defaults,
listing every problem at once. Exits 1 if the configuration is invalid.
`)
}
