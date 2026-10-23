// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"testing"

	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/pkg/testkit"
)

func TestModerationRetentionContract(t *testing.T) {
	testkit.ModerationRetentionContract(t, func(t *testing.T, opts uow.Options) testkit.ModerationHarness {
		db := newTestDB(t, opts)
		return testkit.ModerationHarness{UnitOfWork: db, Moderation: db.ModerationRepository()}
	})
}
