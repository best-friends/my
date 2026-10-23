// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	appcontent "github.com/mycroblog/my/internal/app/content"
	"github.com/mycroblog/my/internal/app/identity"
	appinstance "github.com/mycroblog/my/internal/app/instance"
	"github.com/mycroblog/my/internal/domain/account"
	"github.com/mycroblog/my/internal/domain/instance"
	"github.com/mycroblog/my/internal/interface/clientapi"
	"github.com/mycroblog/my/internal/platform"
	"github.com/mycroblog/my/pkg/testkit"
)

// TestInstanceContactAndThumbnail checks end to end that the contact settings and the thumbnail
// reach GET /instance and NodeInfo's metadata as soon as they are set, and that the contact
// account must be a local account.
func TestInstanceContactAndThumbnail(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true)})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	ctx := t.Context()
	reg, err := app.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
		Username: "admin", Password: "correct horse battery staple", DisplayName: "The Admin", Locale: "en",
	})
	if err != nil {
		t.Fatalf("RegisterLocalUser: %v", err)
	}
	if _, err := app.Instance.SetSetting(ctx, appinstance.SetSettingInput{Key: instance.SettingContactAccount, Value: "nobody"}); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("SetSetting(contact_account, nobody) = %v, want ErrInvalidValue", err)
	}
	m, err := app.Media.Upload(ctx, appcontent.UploadInput{OwnerAccountID: reg.AccountID, Content: bytes.NewReader(testkit.FixturePNG(1600, 900, false))})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if r := app.Media.Process(ctx, m.File.ID, ""); r.Err() != nil {
		t.Fatalf("Process: %v", r.Err())
	}
	if _, err := app.Instance.SetSettings(ctx, []appinstance.SetSettingInput{
		{Key: instance.SettingContactAccount, Value: "Admin"},
		{Key: instance.SettingContactEmail, Value: "admin@example.com"},
		{Key: instance.SettingThumbnail, Value: m.File.ID},
	}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if _, err := app.Instance.SetText(ctx, appinstance.SetTextInput{Key: instance.TextThumbnailAlt, Body: "海辺"}); err != nil {
		t.Fatalf("SetText(thumbnail_alt): %v", err)
	}
	processed, err := app.Media.Media(ctx, m.File.ID)
	if err != nil || !processed.Ready() {
		t.Fatalf("Media = %+v, %v", processed.Result, err)
	}
	thumbnailURL := "https://" + cfg.Host + "/media/" + m.File.ID + "/" + processed.Result.ID + "/large.jpg"

	h := app.Handler()
	get := func(path, lang string, out any) {
		t.Helper()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "https://"+cfg.Host+path, nil)
		req.Header.Set("Accept-Language", lang)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}

	var inst clientapi.Instance
	get(clientapi.BasePath+"/instance", "ja", &inst)
	if c := inst.Contact; c == nil || c.Email == nil || *c.Email != "admin@example.com" || c.Account == nil || c.Account.Id != reg.AccountID {
		t.Errorf("GET /instance contact = %+v, want admin and admin@example.com", c)
	}
	if th := inst.Thumbnail; th == nil || th.Url != thumbnailURL || th.Width != 1600 || th.Height != 900 || th.Alt == nil || *th.Alt != "海辺" {
		t.Errorf("GET /instance thumbnail = %+v, want %s with its alt text", th, thumbnailURL)
	}

	var nodeInfo struct {
		Metadata struct {
			Maintainer    struct{ Name, Email string }
			StaffAccounts []string
			Thumbnail     struct{ URL, Alt string }
		}
	}
	get("/nodeinfo/2.1", "ja", &nodeInfo)
	meta := nodeInfo.Metadata
	if meta.Maintainer.Name != "The Admin" || meta.Maintainer.Email != "admin@example.com" ||
		len(meta.StaffAccounts) != 1 || meta.StaffAccounts[0] != account.ActorURI(cfg.Host, reg.AccountID) {
		t.Errorf("NodeInfo maintainer = %+v, staff %q", meta.Maintainer, meta.StaffAccounts)
	}
	if meta.Thumbnail.URL != thumbnailURL || meta.Thumbnail.Alt != "海辺" {
		t.Errorf("NodeInfo thumbnail = %+v, want %s with its alt text", meta.Thumbnail, thumbnailURL)
	}
}
