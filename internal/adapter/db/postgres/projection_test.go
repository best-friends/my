// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"testing"

	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/pkg/testkit"
)

func TestProjectionStoreContract(t *testing.T) {
	testkit.ProjectionStoreContract(t, func(t *testing.T) testkit.ProjectionStoreHarness {
		db := newTestDB(t, uow.Options{})
		return testkit.ProjectionStoreHarness{UnitOfWork: db, Store: db.ProjectionStore()}
	})
}
