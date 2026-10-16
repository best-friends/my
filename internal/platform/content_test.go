// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"errors"
	"strings"
	"testing"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/identity"
	appinstance "github.com/best-friends/my/internal/app/instance"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/platform"
)

// TestPostingIsWired checks that the composition root wires the posting use cases to the
// database, the account module, the instance settings, and the user's preferences.
func TestPostingIsWired(t *testing.T) {
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
