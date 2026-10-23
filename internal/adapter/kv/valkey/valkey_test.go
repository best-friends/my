// SPDX-License-Identifier: AGPL-3.0-only

package valkey_test

import (
	"os"
	"testing"
	"time"

	kvvalkey "github.com/mycroblog/my/internal/adapter/kv/valkey"
	"github.com/mycroblog/my/internal/adapter/valkey"
	"github.com/mycroblog/my/internal/app/kernel"
	"github.com/mycroblog/my/pkg/testkit"
	"github.com/mycroblog/my/pkg/testkit/valkeytest"
)

// TestMain stops the Valkey container of package valkeytest after the tests.
func TestMain(m *testing.M) {
	os.Exit(valkeytest.Main(m))
}

func TestKeyValueStoreContract(t *testing.T) {
	testkit.KeyValueStoreContract(t, func(t *testing.T, clock *testkit.Clock) kernel.KeyValueStore {
		s := valkeytest.New(t)
		return kvvalkey.New(open(t, s), s.KeyPrefix, clock)
	})
}

// Stores with different key prefixes on one server keep their entries apart, and a namespace
// with the separator of the key layout does not collide with another namespace and key.
func TestStoresAreSeparate(t *testing.T) {
	s := valkeytest.New(t)
	c := open(t, s)
	clock := testkit.NewClock(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	a := kvvalkey.New(c, s.KeyPrefix+"a:", clock)
	b := kvvalkey.New(c, s.KeyPrefix+"b:", clock)
	ctx := t.Context()
	if err := a.Set(ctx, "ns", "k", []byte("in a"), 0); err != nil {
		t.Fatal(err)
	}
	if v, err := b.Get(ctx, "ns", "k"); err == nil {
		t.Fatalf("b.Get = %q; want ErrKeyNotFound", v)
	}
	if err := a.Set(ctx, "x:y", "z", []byte("x:y/z"), 0); err != nil {
		t.Fatal(err)
	}
	if v, err := a.Get(ctx, "x", "y:z"); err == nil {
		t.Fatalf("Get(x, y:z) = %q; want ErrKeyNotFound", v)
	}
}

// open connects to the test server and closes the client when t ends.
func open(t *testing.T, s valkeytest.Server) valkey.Client {
	t.Helper()
	c, err := valkey.Open(t.Context(), valkey.Options{URL: s.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}
