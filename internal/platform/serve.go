// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/best-friends/my/internal/platform/config"
)

// DefaultShutdownTimeout is how long Serve waits for in-flight requests and jobs after ctx
// ends. It fits in the 30-second grace period that Kubernetes and ECS give after SIGTERM.
const DefaultShutdownTimeout = 25 * time.Second

// ServeOptions configure Serve.
type ServeOptions struct {
	// Listener, when set, is used instead of listening on the configured address.
	Listener net.Listener
	// ShutdownTimeout bounds the graceful shutdown; the default is DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
}

// Serve runs the App in its configured role until ctx ends, then shuts down gracefully:
//
//   - every role serves HTTP on the listen address (the worker role only the health checks);
//   - the worker and all roles run the job runner and the scheduler of periodic jobs.
//
// When ctx ends, Serve stops accepting connections and claiming jobs, and waits up to
// ShutdownTimeout for the requests and the job in progress to finish. After the timeout it
// cancels their contexts, closes the remaining connections, and waits for the job runner to
// return. Serve returns nil after a graceful shutdown and an error when it cannot listen,
// the server fails, or the timeout was hit.
func Serve(ctx context.Context, a *App, opts ServeOptions) error {
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = DefaultShutdownTimeout
	}
	ln := opts.Listener
	if ln == nil {
		var err error
		if ln, err = (&net.ListenConfig{}).Listen(ctx, "tcp", a.Config.Listen); err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	}

	// Requests and jobs run under hardCtx, which outlives ctx by the shutdown timeout.
	hardCtx, hardStop := context.WithCancel(context.WithoutCancel(ctx))
	defer hardStop()

	srv := &http.Server{
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(a.Logger.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return hardCtx },
	}
	// Open streams never finish on their own: end them when the shutdown starts, so that it
	// waits only for the other requests.
	if a.Streams != nil {
		srv.RegisterOnShutdown(a.Streams.Close)
	}
	if a.Config.Role != config.RoleWorker {
		// The authorization server also registers the web client on its first request, so a
		// schema applied after the start only delays this.
		if err := a.Identity.EnsureWebClient(ctx); err != nil {
			a.Logger.WarnContext(ctx, "web client not registered yet", slog.Any("error", err))
		}
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	var jobs sync.WaitGroup
	stopJobs := make(chan struct{})
	if a.Config.Role != config.RoleWeb {
		jobs.Go(func() { _ = a.Runner.RunUntil(hardCtx, stopJobs) })
		if a.Scheduler != nil {
			jobs.Go(func() { a.Scheduler.RunUntil(hardCtx, stopJobs, 0) })
		}
	}
	a.Logger.InfoContext(ctx, "started", slog.String("role", string(a.Config.Role)), slog.String("addr", ln.Addr().String()))

	var err error
	select {
	case <-ctx.Done():
	case err = <-serveErr:
		err = fmt.Errorf("http server: %w", err)
	}
	a.Logger.InfoContext(ctx, "shutting down", slog.Duration("timeout", opts.ShutdownTimeout))
	close(stopJobs)

	graceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opts.ShutdownTimeout)
	defer cancel()
	if shutdownErr := srv.Shutdown(graceCtx); shutdownErr != nil {
		err = errors.Join(err, fmt.Errorf("http server shutdown: %w", shutdownErr))
	}
	jobsDone := make(chan struct{})
	go func() { jobs.Wait(); close(jobsDone) }()
	select {
	case <-jobsDone:
	case <-graceCtx.Done():
		err = errors.Join(err, errors.New("job runner did not stop within the shutdown timeout"))
	}
	hardStop()
	_ = srv.Close()
	<-jobsDone
	if err == nil {
		a.Logger.InfoContext(ctx, "stopped")
	}
	return err
}
