// SPDX-License-Identifier: AGPL-3.0-only

package sqlite

import (
	"testing"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/pkg/testkit"
)

func TestSignupBlockContract(t *testing.T) {
	testkit.SignupBlockContract(t, func(t *testing.T, opts uow.Options) testkit.ModerationHarness {
		db, _ := newTestDB(t, "", opts)
		return testkit.ModerationHarness{UnitOfWork: db, Moderation: db.ModerationRepository()}
	})
}
