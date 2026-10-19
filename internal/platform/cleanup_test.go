// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"slices"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/platform"
)

// The scheduler's first tick enqueues every periodic cleanup, and the key-value store's cleanup
// removes the expired entries physically while it keeps the live ones.
func TestSchedulerRunsTheCleanups(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_QUEUE": "inline"})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	var names []string
	for _, s := range app.Scheduler.Schedules() {
		names = append(names, s.Name)
	}
	for _, want := range []string{platform.JobCleanupKV, graph.JobCleanupMutes, content.JobCleanupMedia} {
		if !slices.Contains(names, want) {
			t.Errorf("schedules = %v, want %s among them", names, want)
		}
	}

	ctx := t.Context()
	for _, key := range []string{"a", "b", "c"} {
		if err := app.KV.Set(ctx, "test", key, []byte("v"), time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.KV.Set(ctx, "test", "live", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	// The inline queue runs the enqueued jobs before Tick returns.
	app.Scheduler.Tick(ctx)
	if n, err := app.KV.DeleteExpired(ctx, 100); err != nil || n != 0 {
		t.Errorf("DeleteExpired after the cleanup = %d, %v; want nothing left", n, err)
	}
	if v, err := app.KV.Get(ctx, "test", "live"); err != nil || string(v) != "v" {
		t.Errorf("Get(live) = %q, %v; want it kept", v, err)
	}
}
