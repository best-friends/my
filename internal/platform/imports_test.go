// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	domcontent "github.com/best-friends/my/internal/domain/content"
	domidentity "github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// TestCSVExportAndImport checks the CSV files on the real database: what one user exports, in
// Mastodon's formats, another user imports, and the import's jobs follow, block, mute, and
// bookmark the same accounts and posts, local and remote, with the follow's settings; an
// address that leads to no account is counted as failed, and an overwrite removes the rest.
func TestCSVExportAndImport(t *testing.T) {
	t.Parallel()
	e := newTimelineAPI(t)
	ctx := t.Context()
	alice, bob, dave, erin := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "dave"), e.user(t, "erin")
	const actor = "https://remote.example/users/carol"
	var carol account.Account
	err := e.app.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		var err error
		carol, err = e.app.Accounts.UpsertRemoteActor(ctx, tx, appaccount.RemoteActorInput{
			URI: actor, Domain: "remote.example", Type: "Person", PreferredUsername: "carol",
			InboxURL: actor + "/inbox", FollowersURL: actor + "/followers", Document: []byte(`{"id":"` + actor + `"}`),
			Handle: appaccount.HandleCheck{Outcome: account.HandleVerified, Username: "carol", Domain: "remote.example"},
		})
		return err
	})
	if err != nil {
		t.Fatalf("UpsertRemoteActor: %v", err)
	}
	post := e.post(t, bob, "worth keeping", domcontent.VisibilityPublic, "en")

	// Alice follows bob without his boosts and carol, blocks dave, mutes erin, and bookmarks bob's post.
	e.follow(t, alice, bob)
	if _, err := e.app.Graph.UpdatePreferences(ctx, alice.accountID, bob.accountID, graph.PreferencesInput{ShowBoosts: false, Notify: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Graph.Follow(ctx, alice.accountID, carol.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Graph.Block(ctx, alice.accountID, dave.accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Graph.Mute(ctx, alice.accountID, erin.accountID, graph.MuteInput{HideNotifications: false}); err != nil {
		t.Fatal(err)
	}
	if err := e.app.Interactions.Bookmark(ctx, alice.accountID, post); err != nil {
		t.Fatal(err)
	}

	domain := e.app.Config.Host
	if e.app.Config.AccountDomain != "" {
		domain = e.app.Config.AccountDomain
	}
	export := func(kind string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/csv_exports/"+kind, nil)
		req.Header.Set("Authorization", "Bearer "+alice.token)
		res := e.spec.Serve(t, e.h, req)
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /csv_exports/%s = %d %s", kind, res.StatusCode, body)
		}
		return string(body)
	}
	// The follow of carol is pending until her server accepts it, so only bob is listed.
	files := map[domidentity.ImportKind]string{}
	for kind, want := range map[domidentity.ImportKind]string{
		domidentity.ImportFollowing: "Account address,Show boosts,Notify on new posts,Languages\nbob@" + domain + ",false,true,\n",
		domidentity.ImportBlocks:    "dave@" + domain + "\n",
		domidentity.ImportMutes:     "Account address,Hide notifications\nerin@" + domain + ",false\n",
	} {
		if got := export(string(kind)); got != want {
			t.Errorf("%s = %q, want %q", kind, got, want)
		}
		files[kind] = want
	}
	bookmarks := export("bookmarks")
	if want := "https://" + e.app.Config.Host + "/"; !bytes.HasPrefix([]byte(bookmarks), []byte(want)) {
		t.Errorf("bookmarks = %q, want the post's URI", bookmarks)
	}
	files[domidentity.ImportBookmarks] = bookmarks
	// A Mastodon file may name remote accounts and accounts that do not exist.
	files[domidentity.ImportFollowing] += "carol@remote.example,true,false,\nghost@" + domain + ",true,false,\n"

	// Bob imports them.
	for _, kind := range domidentity.ImportKinds {
		in := identity.ImportInput{UserID: bob.userID, AccountID: bob.accountID, Kind: kind, Mode: domidentity.ImportMerge, Text: files[kind]}
		started, err := e.app.Imports.StartImport(ctx, in)
		if err != nil {
			t.Fatalf("StartImport(%s) = %v", kind, err)
		}
		if err := e.app.Imports.ProcessImport(ctx, started.ID, false); err != nil {
			t.Fatalf("ProcessImport(%s) = %v", kind, err)
		}
	}
	list, err := e.app.Imports.ListImports(ctx, bob.userID)
	if err != nil || len(list) != 4 {
		t.Fatalf("ListImports = %+v, %v", list, err)
	}
	for _, imp := range list {
		wantFailed := 0
		if imp.Kind == domidentity.ImportFollowing {
			wantFailed = 2 // himself, and the ghost
		}
		if !imp.Finished() || imp.Failed != wantFailed {
			t.Errorf("%s import = %+v, want finished with %d failed", imp.Kind, imp, wantFailed)
		}
	}
	rel := func(target string) graph.Relationship {
		t.Helper()
		r, err := e.app.Graph.Relationship(ctx, bob.accountID, target)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := rel(carol.ID); !r.RequestedFollow() {
		t.Errorf("bob → carol = %+v, want a follow request sent to her server", r)
	}
	if r := rel(dave.accountID); !r.Blocking {
		t.Errorf("bob → dave = %+v, want blocking", r)
	}
	if r := rel(erin.accountID); !r.Muting || r.MutingNotifications {
		t.Errorf("bob → erin = %+v, want muting with notifications", r)
	}
	page, err := e.app.Statuses.Bookmarks(ctx, bob.accountID, "", 10)
	if err != nil || len(page.Statuses) != 1 || page.Statuses[0].Status.ID != post {
		t.Errorf("bob's bookmarks = %+v, %v; want the post", page, err)
	}

	// Alice overwrites her blocks with an empty list.
	in := identity.ImportInput{UserID: alice.userID, AccountID: alice.accountID, Kind: domidentity.ImportBlocks, Mode: domidentity.ImportOverwrite}
	started, err := e.app.Imports.StartImport(ctx, in)
	if err != nil || started.Total != 1 {
		t.Fatalf("StartImport(overwrite) = %+v, %v; want one removal", started, err)
	}
	if err := e.app.Imports.ProcessImport(ctx, started.ID, false); err != nil {
		t.Fatal(err)
	}
	if blocking, _ := e.app.Graph.IsBlocking(ctx, alice.accountID, dave.accountID); blocking {
		t.Error("alice still blocks dave after the overwrite")
	}
}
