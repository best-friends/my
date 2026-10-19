// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/identity"
	appinstance "github.com/best-friends/my/internal/app/instance"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/platform"
)

// TestPostingIsWired checks that the composition root wires the posting use cases to the
// database, the account module, the instance settings, and the user's preferences.
func TestPostingIsWired(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", DisplayName: "Alice", Locale: "ja",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser: %v", err)
		}
		prefs, err := a.Identity.Preferences(ctx, reg.User.ID)
		if err != nil {
			t.Fatalf("Preferences: %v", err)
		}
		in := appcontent.CreateInput{
			AuthorAccountID: reg.AccountID, TextInput: appcontent.TextInput{Text: "hello @alice #wired"}, Visibility: domcontent.VisibilityPublic,
		}
		p, err := a.Content.Create(ctx, in)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if p.Status.URI != domcontent.StatusURI(cfg.Host, p.Status.ID) || p.Revision.Language != prefs.DefaultPostLanguage ||
			!strings.Contains(p.Revision.Bodies[0].ContentHTML, `class="u-url mention"`) {
			t.Errorf("Create = %+v, want a status on %s in the default post language %q mentioning alice", p, cfg.Host, prefs.DefaultPostLanguage)
		}
		if _, err := a.Instance.SetSetting(ctx, appinstance.SetSettingInput{Key: instance.SettingMaxStatusLength, Value: "3"}); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if _, err := a.Content.Create(ctx, in); !errors.Is(err, appcontent.ErrTextTooLong) {
			t.Errorf("Create over max_status_length = %v, want ErrTextTooLong", err)
		}
	})
}

// TestTimelinesAreWired checks that the composition root wires the timeline queries and registers
// the timeline projectors exactly once on the content and graph events: a second registration
// would fail the post on its feed entry, and every timeline lists each entry once.
func TestTimelinesAreWired(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		var accounts []string
		for _, name := range []string{"alice", "bob", "carol"} {
			reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
				Username: name, Password: "correct horse battery staple", DisplayName: name, Locale: "en",
			})
			if err != nil {
				t.Fatalf("RegisterLocalUser(%s): %v", name, err)
			}
			accounts = append(accounts, reg.AccountID)
		}
		alice, bob, carol := accounts[0], accounts[1], accounts[2]
		if _, err := a.Graph.Follow(ctx, bob, alice); err != nil {
			t.Fatalf("Follow: %v", err)
		}
		p, err := a.Content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "hello timelines"}, Visibility: domcontent.VisibilityPublic,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		b, err := a.Interactions.Boost(ctx, carol, p.Status.ID)
		if err != nil {
			t.Fatalf("Boost: %v", err)
		}
		if _, err := a.Graph.Follow(ctx, bob, carol); err != nil {
			t.Fatalf("Follow: %v", err)
		}
		ids := func(page apptimeline.Page, err error) []string {
			t.Helper()
			if err != nil {
				t.Fatalf("timeline: %v", err)
			}
			var out []string
			for _, e := range page.Entries {
				out = append(out, e.ID)
			}
			return out
		}
		local := domcontent.Viewer{Kind: domcontent.ViewerLocal, AccountID: bob}
		if got, want := ids(a.Timeline.Home(ctx, local, apptimeline.Query{})), []string{b.ID, p.Status.ID}; !slices.Equal(got, want) {
			t.Errorf("Home = %v, want %v", got, want)
		}
		anonymous := domcontent.Viewer{Kind: domcontent.ViewerAnonymous}
		if got, want := ids(a.Timeline.Local(ctx, anonymous, apptimeline.Query{})), []string{p.Status.ID}; !slices.Equal(got, want) {
			t.Errorf("Local = %v, want %v", got, want)
		}
		if _, err := a.Graph.Mute(ctx, bob, alice, appgraph.MuteInput{}); err != nil {
			t.Fatalf("Mute: %v", err)
		}
		if got := ids(a.Timeline.Home(ctx, local, apptimeline.Query{})); len(got) != 0 {
			t.Errorf("Home after muting the author = %v, want none", got)
		}
	})
}

// TestInteractionsAreWired checks that the composition root wires the interaction use cases and
// registers each content projector exactly once: a boost and a favourite count one each, and a
// repeated command changes nothing.
func TestInteractionsAreWired(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	start(t, cfg, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		var accounts []string
		for _, name := range []string{"alice", "bob"} {
			reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
				Username: name, Password: "correct horse battery staple", DisplayName: name, Locale: "en",
			})
			if err != nil {
				t.Fatalf("RegisterLocalUser(%s): %v", name, err)
			}
			accounts = append(accounts, reg.AccountID)
		}
		p, err := a.Content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: accounts[0], TextInput: appcontent.TextInput{Text: "boost me"}, Visibility: domcontent.VisibilityPublic,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		for range 2 {
			if _, err := a.Interactions.Boost(ctx, accounts[1], p.Status.ID); err != nil {
				t.Fatalf("Boost: %v", err)
			}
			if _, err := a.Interactions.Favourite(ctx, accounts[1], p.Status.ID); err != nil {
				t.Fatalf("Favourite: %v", err)
			}
		}
		c, err := a.Interactions.Counters(ctx, p.Status.ID)
		if err != nil || c.BoostsCount != 1 || c.FavouritesCount != 1 || c.RepliesCount != 0 {
			t.Errorf("Counters = %+v, %v; want one boost and one favourite", c, err)
		}
	})
}
