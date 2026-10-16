// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"reflect"
	"testing"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/identity"
	appnotification "github.com/best-friends/my/internal/app/notification"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/notification"
	"github.com/best-friends/my/internal/platform"
)

// TestNotificationsAreWired checks that the composition root registers the notification
// projectors once, on the graph and content events: a follow, a mention, and a favourite each
// notify the other account once, and the unread count follows.
func TestNotificationsAreWired(t *testing.T) {
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
		alice, bob := accounts[0], accounts[1]
		if _, err := a.Graph.Follow(ctx, bob, alice); err != nil {
			t.Fatalf("Follow: %v", err)
		}
		p, err := a.Content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "hi @bob"}, Visibility: domcontent.VisibilityPublic,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		for range 2 {
			if _, err := a.Interactions.Favourite(ctx, bob, p.Status.ID); err != nil {
				t.Fatalf("Favourite: %v", err)
			}
		}
		types := func(accountID string) []notification.Type {
			ns, err := a.Notifications.List(ctx, accountID, appnotification.Query{})
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			out := []notification.Type{}
			for _, n := range ns {
				out = append(out, n.Type)
			}
			return out
		}
		if got, want := types(alice), []notification.Type{notification.TypeFavourite, notification.TypeFollow}; !reflect.DeepEqual(got, want) {
			t.Errorf("notifications of alice = %v, want %v", got, want)
		}
		if got, want := types(bob), []notification.Type{notification.TypeMention}; !reflect.DeepEqual(got, want) {
			t.Errorf("notifications of bob = %v, want %v", got, want)
		}
		if n, err := a.Notifications.UnreadCount(ctx, alice); err != nil || n != 2 {
			t.Errorf("UnreadCount = %d, %v; want 2", n, err)
		}
	})
}
