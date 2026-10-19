// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"testing"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/pkg/testkit"
)

func TestProjectionStoreContract(t *testing.T) {
	testkit.ProjectionStoreContract(t, func(t *testing.T) testkit.ProjectionStoreHarness {
		db := newTestDB(t, uow.Options{})
		return testkit.ProjectionStoreHarness{UnitOfWork: db, Store: db.ProjectionStore()}
	})
}
