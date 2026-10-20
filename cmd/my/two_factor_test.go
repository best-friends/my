// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"

	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/moderation"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/internal/platform/config"
)

func TestAdminAccountDisableTwoFactor(t *testing.T) {
	newAdminEnv(t)
	if code, _, stderr := runAdminCreate(t, adminPassword+"\n", "-username", "root", "-email", "root@example.com"); code != 0 {
		t.Fatalf("admin create: %d %q", code, stderr)
	}
	cfg, _, err := config.Load(config.Source{})
	if err != nil {
		t.Fatal(err)
	}
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = app.Close() }()
	ctx := t.Context()
	root, err := app.Accounts.FindByHandle(ctx, "root", "")
	if err != nil {
		t.Fatal(err)
	}
	userID := root.Local.UserID

	if code, _, stderr := runCmd(t, adminCmd, "account", "disable-two-factor", "@root"); code != 1 || !strings.Contains(stderr, "has no two-factor authentication") {
		t.Errorf("disable-two-factor without TOTP = %d %q", code, stderr)
	}
	setup, err := app.Identity.BeginTOTPSetup(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Identity.EnableTOTP(ctx, userID, setup.Token, identity.TOTPCode(secret, identity.TOTPStep(time.Now()))); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCmd(t, adminCmd, "account", "disable-two-factor", "@root")
	if code != 0 || stdout != "removed the two-factor authentication of account "+root.Account.ID+"\n" {
		t.Fatalf("disable-two-factor = %d %q %q", code, stdout, stderr)
	}
	if st, err := app.Identity.TwoFactorStatus(ctx, userID); err != nil || st.Enabled {
		t.Errorf("TwoFactorStatus = %+v, %v; want disabled", st, err)
	}
	entries, err := app.Moderation.AuditLog(ctx, appmoderation.AuditQuery{Action: moderation.AuditTwoFactorDisabled}, 10)
	if err != nil || len(entries) != 1 || entries[0].ActorKind != moderation.AuditActorCLI || entries[0].TargetID != root.Account.ID {
		t.Errorf("audit log = %+v, %v; want one entry by the CLI", entries, err)
	}
}
