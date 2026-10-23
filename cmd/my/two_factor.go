// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	appidentity "github.com/mycroblog/my/internal/app/identity"
)

// runAdminAccountDisableTwoFactor implements `my admin account disable-two-factor <account>`:
// it removes the two-factor authentication of a local user who lost their authenticator app and
// recovery codes (FR-005), such as the only administrator, who cannot ask another one. The CLI
// acts as no user; the audit log records it as the CLI's.
func runAdminAccountDisableTwoFactor(args []string, stdout, stderr io.Writer) int {
	c, code := parseSubcommand("my admin account disable-two-factor", args, 1, stderr, adminAccountDisableTwoFactorUsage, nil)
	if code >= 0 {
		return code
	}
	ctx := context.Background()
	app, code := c.open(ctx, stderr)
	if code >= 0 {
		return code
	}
	defer func() { _ = app.Close() }()
	id, err := resolveAccount(ctx, app, c.fs.Arg(0))
	if err == nil {
		err = app.AccountAdmin.DisableTwoFactor(ctx, id, "")
		switch {
		case errors.Is(err, appidentity.ErrNotLocalAccount):
			err = fmt.Errorf("%s is not the account of a local user", c.fs.Arg(0))
		case errors.Is(err, appidentity.ErrTwoFactorNotEnabled):
			err = fmt.Errorf("%s has no two-factor authentication", c.fs.Arg(0))
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", c.name, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "removed the two-factor authentication of account %s\n", id)
	return 0
}

func adminAccountDisableTwoFactorUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: my admin account disable-two-factor [-config <file>] <account>

Removes the two-factor authentication (TOTP) of a local account's user, who
then signs in with the password alone, for a user who lost both the
authenticator app and the recovery codes. Make sure the request comes from the
account's owner first. The audit log records it as two_factor_disabled.
<account> is @username or an account ID.
`)
}
