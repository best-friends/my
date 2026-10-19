// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"context"
	"database/sql"
	"log/slog"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/projection"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/platform"
)

// TestProjectionsVerifyAndRebuild fills every projection through the services, on both
// dialects, and checks that each one verifies, that verify finds the rows that raw SQL breaks
// and changes nothing, and that rebuild restores them and keeps the fetch state of the remote
// actors' heads, which no event records. The scheduled job projection.verify corrects them.
func TestProjectionsVerifyAndRebuild(t *testing.T) {
	t.Parallel()
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			t.Parallel()
			dbURL := newDatabase(t, true)
			if dialect == "postgres" {
				dbURL = newPostgresDatabase(t)
			}
			cfg := loadConfig(t, map[string]string{"MY_DB": dialect, "MY_DATABASE_URL": dbURL, "MY_WEB_ORIGIN": webOrigin})
			app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler)})
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			t.Cleanup(func() { _ = app.Close() })
			fillProjections(t, app)
			exec, count := rawSQL(t, dialect, dbURL)
			checkProjections(t, app, exec, count)
		})
	}
}

func fillProjections(t *testing.T, a *platform.App) {
	t.Helper()
	ctx := t.Context()
	ids := map[string]string{}
	for _, name := range []string{"alice", "bob", "carol"} {
		reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
			Username: name, Password: "correct horse battery staple", DisplayName: name, Locale: "en",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser(%s): %v", name, err)
		}
		ids[name] = reg.AccountID
	}
	alice, bob, carol := ids["alice"], ids["bob"], ids["carol"]
	err := a.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		_, err := a.Accounts.UpsertRemoteActor(ctx, tx, appaccount.RemoteActorInput{
			URI: "https://remote.example/users/dave", Domain: "remote.example", Type: "Person",
			PreferredUsername: "dave", InboxURL: "https://remote.example/users/dave/inbox",
			SharedInboxURL: "https://remote.example/inbox",
		})
		return err
	})
	if err != nil {
		t.Fatalf("UpsertRemoteActor: %v", err)
	}
	if _, err := a.Graph.Follow(ctx, bob, alice); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	post := func(author, text string) string {
		p, err := a.Content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: author, TextInput: appcontent.TextInput{Text: text}, Visibility: domcontent.VisibilityPublic,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		return p.Status.ID
	}
	first := post(alice, "hi @bob #projections")
	gone := post(alice, "soon gone")
	post(bob, "hello")
	for _, id := range []string{first, gone} {
		if _, err := a.Interactions.Favourite(ctx, bob, id); err != nil {
			t.Fatalf("Favourite: %v", err)
		}
	}
	if _, err := a.Interactions.Boost(ctx, bob, first); err != nil {
		t.Fatalf("Boost: %v", err)
	}
	if _, err := a.Content.Delete(ctx, alice, gone); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := a.Graph.Block(ctx, carol, bob); err != nil {
		t.Fatalf("Block: %v", err)
	}
	if _, err := a.Graph.Mute(ctx, alice, carol, appgraph.MuteInput{HideNotifications: true}); err != nil {
		t.Fatalf("Mute: %v", err)
	}
}

// rawSQL returns functions that run SQL on the database directly, to break projections and to
// read a number.
func rawSQL(t *testing.T, dialect, dbURL string) (exec func(q string), count func(q string) int64) {
	t.Helper()
	if dialect == "postgres" {
		conn, err := pgx.Connect(t.Context(), dbURL)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close(context.Background()) })
		return func(q string) {
				if _, err := conn.Exec(t.Context(), q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}, func(q string) int64 {
				var n int64
				if err := conn.QueryRow(t.Context(), q).Scan(&n); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
				return n
			}
	}
	db, err := sql.Open("sqlite", dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return func(q string) {
			if _, err := db.ExecContext(t.Context(), q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}, func(q string) int64 {
			var n int64
			if err := db.QueryRowContext(t.Context(), q).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			return n
		}
}

func checkProjections(t *testing.T, a *platform.App, exec func(string), count func(string) int64) {
	t.Helper()
	ctx := t.Context()
	svc := a.Projections
	verifyAll := func() map[string]projection.Report {
		out := map[string]projection.Report{}
		for _, p := range svc.List() {
			r, err := svc.Verify(ctx, p)
			if err != nil {
				t.Fatalf("Verify(%s) = %v", p.Name, err)
			}
			out[p.Name] = r
		}
		return out
	}
	// Every projection verifies, and the commands filled the tables below.
	held := map[string]int{}
	for name, r := range verifyAll() {
		if r.Drifted() {
			t.Errorf("Verify(%s) after the commands = %+v, want no drift", name, r.Tables)
		}
		for _, d := range r.Tables {
			held[d.Table] = d.Rows
		}
	}
	for _, table := range []string{
		"account_handles", "account_profile_heads", "remote_account_heads", "follow_edges", "follow_counters",
		"block_edges", "status_heads", "status_mention_index", "status_counters", "account_post_counters",
		"boost_edges", "feed_entries", "feed_tag_entries", "feed_follow_edges", "feed_exclusions",
		"notification_exclusions", "notification_deleted_statuses",
	} {
		if held[table] == 0 {
			t.Errorf("%s holds no rows", table)
		}
	}

	exec("UPDATE status_counters SET favourites_count = favourites_count + 5")
	exec("DELETE FROM feed_tag_entries")
	exec("DELETE FROM notification_exclusions WHERE reason = 'mute'")
	exec("INSERT INTO notification_deleted_statuses (status_id) VALUES ('no-such-status')")
	exec("UPDATE remote_account_heads SET inbox_url = 'https://elsewhere.example/inbox', consecutive_fetch_failures = 3")
	exec("DELETE FROM follow_counters")

	drifted := []string{}
	for _, p := range svc.List() {
		r, err := svc.Verify(ctx, p)
		if err != nil {
			t.Fatalf("Verify(%s) = %v", p.Name, err)
		}
		for _, d := range r.Tables {
			if !d.Empty() {
				drifted = append(drifted, d.Table)
			}
		}
	}
	slices.Sort(drifted)
	want := []string{
		"feed_tag_entries", "follow_counters", "notification_deleted_statuses", "notification_exclusions",
		"remote_account_heads", "status_counters",
	}
	if !slices.Equal(drifted, want) {
		t.Fatalf("tables that verify reports = %v, want %v", drifted, want)
	}
	// Verify changed nothing.
	again, err := svc.Lookup("feed_tag_entries")
	if err != nil {
		t.Fatal(err)
	}
	if r, err := svc.Verify(ctx, again); err != nil || !r.Drifted() {
		t.Errorf("Verify(feeds) a second time = %v, %v; want drift", r.Drifted(), err)
	}

	// The scheduled job corrects every projection.
	if !slices.ContainsFunc(a.Scheduler.Schedules(), func(s queue.Schedule) bool { return s.Name == projection.JobVerify }) {
		t.Errorf("no %s schedule", projection.JobVerify)
	}
	if err := a.Jobs.Enqueue(ctx, queue.NewJob{Type: projection.JobVerify}); err != nil {
		t.Fatalf("Enqueue = %v", err)
	}
	if n, err := a.Runner.Drain(ctx); err != nil || n == 0 {
		t.Fatalf("Drain = %d, %v; want the job run", n, err)
	}
	for name, r := range verifyAll() {
		if r.Drifted() {
			t.Errorf("Verify(%s) after the job = %+v, want no drift", name, r.Tables)
		}
	}
	if n := count("SELECT COUNT(*) FROM remote_account_heads WHERE inbox_url = 'https://remote.example/users/dave/inbox' AND consecutive_fetch_failures = 3"); n != 1 {
		t.Errorf("remote heads with the snapshot's inbox and the 3 failures kept = %d, want 1", n)
	}
}
