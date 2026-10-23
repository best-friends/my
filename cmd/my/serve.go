// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/mycroblog/my/internal/platform"
	"github.com/mycroblog/my/internal/platform/config"
)

// runServe implements `my serve`: it runs the server in the configured role until SIGTERM or
// SIGINT, then shuts down gracefully. Logs go to stderr as JSON lines.
func runServe(args []string, stderr io.Writer) int {
	sfs := flag.NewFlagSet("my serve", flag.ContinueOnError)
	sfs.SetOutput(stderr)
	sfs.Usage = func() { serveUsage(stderr) }
	file := sfs.String("config", "", "TOML config file (default: $"+config.EnvConfigFile+")")
	devVite := sfs.String("dev-vite-origin", "", "development only: load the web client from the Vite dev server at this origin")
	pseudo := sfs.Bool("pseudo-locales", false, "development and tests only: make the pseudo-locales en-XA and ar-XB selectable")
	if err := sfs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if sfs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "my serve: unexpected argument %q\n", sfs.Arg(0))
		return 2
	}

	cfg, _, err := config.Load(config.Source{File: *file})
	if err != nil {
		printLoadError(stderr, err)
		return 1
	}
	logger := platform.NewLogger(stderr, slog.LevelInfo)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	v := resolveVersion(version, debug.ReadBuildInfo)
	logger.InfoContext(ctx, "starting", slog.String("version", v), slog.Any("config", cfg))
	app, err := platform.NewApp(ctx, cfg, platform.AppOptions{
		Logger: logger, Version: v, DevViteOrigin: *devVite, PseudoLocales: *pseudo,
	})
	if err != nil {
		logger.ErrorContext(ctx, "start failed", slog.Any("error", err))
		return 1
	}
	defer func() { _ = app.Close() }()
	if err := platform.Serve(ctx, app, platform.ServeOptions{}); err != nil {
		logger.ErrorContext(ctx, "serve failed", slog.Any("error", err))
		return 1
	}
	return 0
}

func serveUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: my serve [-config <file>] [-dev-vite-origin <origin>] [-pseudo-locales]

Runs the server in the role set by role (MY_ROLE): web, worker, or all.
Stops gracefully on SIGTERM or SIGINT, waiting for in-flight requests and jobs.

-dev-vite-origin (development only) makes the web client's HTML load the client from a
Vite dev server, such as http://localhost:5173, instead of the build embedded in the binary.

-pseudo-locales (development and tests only) makes the pseudo-locales en-XA (longer, accented
text) and ar-XB (right-to-left) selectable through Accept-Language.
The E2E of the server-rendered pages uses it.
`)
}
