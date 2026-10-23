// SPDX-License-Identifier: AGPL-3.0-only

package memory_test

import (
	"testing"

	"github.com/mycroblog/my/internal/adapter/kv/memory"
	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/pkg/testkit"
)

func TestKeyValueStoreContract(t *testing.T) {
	testkit.KeyValueStoreContract(t, func(_ *testing.T, clock *testkit.Clock) kernel.KeyValueStore {
		return memory.New(clock)
	})
}
