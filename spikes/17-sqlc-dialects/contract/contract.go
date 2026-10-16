// SPDX-License-Identifier: AGPL-3.0-only

// Package contract is the single test suite every store adapter must pass.
// Adapters call Run from their own _test.go with a Factory that returns a
// fresh, migrated, isolated database per subtest.
package contract

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/best-friends/my/spikes/17-sqlc-dialects/app"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/internal/txretry"
	"github.com/best-friends/my/spikes/17-sqlc-dialects/store"
)

// Factory returns a fresh UnitOfWork using retry policy p.
type Factory func(t *testing.T, p txretry.Policy) store.UnitOfWork

// Run executes the whole suite.
func Run(t *testing.T, newUoW Factory) {
	tests := []struct {
		name string
		fn   func(*testing.T, Factory)
	}{
		{"AccountUpsertByURI", testAccountUpsert},
		{"GetNotFound", testGetNotFound},
		{"TimestampRoundTrip", testTimestampRoundTrip},
		{"FollowIdempotent", testFollowIdempotent},
		{"FollowMissingTarget", testFollowMissingTarget},
		{"RollbackOnError", testRollbackOnError},
		{"Unfollow", testUnfollow},
		{"ListFollowersPagination", testListFollowersPagination},
		{"RecountFollowCounts", testRecount},
		{"RetryBounded", testRetryBounded},
		{"ConcurrentFollowDistinct", testConcurrentFollowDistinct},
		{"ConcurrentFollowSamePair", testConcurrentFollowSamePair},
		{"ConcurrentUpsertSameURI", testConcurrentUpsertSameURI},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, newUoW) })
	}
}

// --- helpers ---

func newAccount(name string) store.Account {
	now := store.Timestamp(time.Now())
	return store.Account{
		ID: uuid.NewV7().String(), Username: name, Domain: "example.com",
		URI: "https://example.com/users/" + name + "-" + uuid.NewV4().String()[:8], DisplayName: name,
		CreatedAt: now, UpdatedAt: now,
	}
}

func mustUpsert(t *testing.T, u store.UnitOfWork, a store.Account) store.Account {
	t.Helper()
	var got store.Account
	err := u.Tx(t.Context(), func(tx store.Tx) error {
		var err error
		got, err = tx.Accounts().Upsert(t.Context(), a)
		return err
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return got
}

func mustGet(t *testing.T, u store.UnitOfWork, id string) store.Account {
	t.Helper()
	var got store.Account
	err := u.Tx(t.Context(), func(tx store.Tx) error {
		var err error
		got, err = tx.Accounts().Get(t.Context(), id)
		return err
	})
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	return got
}

func wantCounts(t *testing.T, u store.UnitOfWork, id string, followers, following int64) {
	t.Helper()
	a := mustGet(t, u, id)
	if a.FollowersCount != followers || a.FollowingCount != following {
		t.Fatalf("counts of %s = (%d,%d), want (%d,%d)", a.Username, a.FollowersCount, a.FollowingCount, followers, following)
	}
}

func retryCounter() (txretry.Policy, *atomic.Int64) {
	var n atomic.Int64
	return txretry.Policy{MaxAttempts: 50, OnRetry: func(int, error) { n.Add(1) }}, &n
}

// --- tests ---

func testAccountUpsert(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	a := mustUpsert(t, u, newAccount("alice"))

	again := a
	again.ID = uuid.NewV7().String() // a remote re-fetch would carry a fresh ID
	again.DisplayName = "Alice v2"
	again.UpdatedAt = a.UpdatedAt.Add(time.Second)
	got := mustUpsert(t, u, again)

	if got.ID != a.ID {
		t.Errorf("upsert changed ID: %s -> %s", a.ID, got.ID)
	}
	if got.DisplayName != "Alice v2" || !got.UpdatedAt.Equal(again.UpdatedAt) {
		t.Errorf("upsert did not update mutable fields: %+v", got)
	}
	if !got.CreatedAt.Equal(a.CreatedAt) {
		t.Errorf("upsert changed created_at: %v -> %v", a.CreatedAt, got.CreatedAt)
	}
}

func testGetNotFound(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	err := u.Tx(t.Context(), func(tx store.Tx) error {
		_, err := tx.Accounts().Get(t.Context(), uuid.NewV7().String())
		return err
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func testTimestampRoundTrip(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	jst := time.FixedZone("JST", 9*3600)
	in := time.Date(2026, 10, 16, 21, 4, 5, 123_456_789, jst) // sub-ms, non-UTC
	a := newAccount("tz")
	a.CreatedAt, a.UpdatedAt = store.Timestamp(in), store.Timestamp(in)
	mustUpsert(t, u, a)
	got := mustGet(t, u, a.ID)
	want := time.Date(2026, 10, 16, 12, 4, 5, 123_000_000, time.UTC)
	if !got.CreatedAt.Equal(want) || got.CreatedAt.Location() != time.UTC || got.CreatedAt != want {
		t.Fatalf("created_at = %v (%v), want %v", got.CreatedAt, got.CreatedAt.Location(), want)
	}
}

func testFollowIdempotent(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	alice, bob := mustUpsert(t, u, newAccount("alice")), mustUpsert(t, u, newAccount("bob"))
	svc := &app.Follows{UoW: u}

	for i, want := range []bool{true, false, false} {
		created, err := svc.Follow(t.Context(), alice.ID, bob.ID)
		if err != nil || created != want {
			t.Fatalf("follow #%d = (%v, %v), want (%v, nil)", i, created, err, want)
		}
	}
	wantCounts(t, u, bob.ID, 1, 0)
	wantCounts(t, u, alice.ID, 0, 1)

	if _, err := svc.Follow(t.Context(), alice.ID, alice.ID); !errors.Is(err, app.ErrSelfFollow) {
		t.Fatalf("self follow err = %v", err)
	}
}

func testFollowMissingTarget(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	alice := mustUpsert(t, u, newAccount("alice"))
	svc := &app.Follows{UoW: u}
	_, err := svc.Follow(t.Context(), alice.ID, uuid.NewV7().String())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	wantCounts(t, u, alice.ID, 0, 0)
}

func testRollbackOnError(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	alice, bob := mustUpsert(t, u, newAccount("alice")), mustUpsert(t, u, newAccount("bob"))
	boom := errors.New("boom")
	err := u.Tx(t.Context(), func(tx store.Tx) error {
		ok, err := tx.Follows().Insert(t.Context(), store.Follow{
			ID: uuid.NewV7().String(), FollowerID: alice.ID, TargetID: bob.ID, CreatedAt: store.Timestamp(time.Now()),
		})
		if err != nil || !ok {
			return fmt.Errorf("insert: %v %w", ok, err)
		}
		if err := tx.Accounts().AddFollowCounts(t.Context(), bob.ID, 1, 0); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	wantCounts(t, u, bob.ID, 0, 0)
	err = u.Tx(t.Context(), func(tx store.Tx) error {
		_, err := tx.Follows().Get(t.Context(), alice.ID, bob.ID)
		return err
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("follow survived rollback: %v", err)
	}
}

func testUnfollow(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	alice, bob := mustUpsert(t, u, newAccount("alice")), mustUpsert(t, u, newAccount("bob"))
	svc := &app.Follows{UoW: u}
	if _, err := svc.Follow(t.Context(), alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{true, false} {
		deleted, err := svc.Unfollow(t.Context(), alice.ID, bob.ID)
		if err != nil || deleted != want {
			t.Fatalf("unfollow #%d = (%v, %v), want (%v, nil)", i, deleted, err, want)
		}
	}
	wantCounts(t, u, bob.ID, 0, 0)
	wantCounts(t, u, alice.ID, 0, 0)
}

func testListFollowersPagination(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	target := mustUpsert(t, u, newAccount("target"))
	svc := &app.Follows{UoW: u}
	var ids []string // in creation order == UUIDv7 order
	for i := range 7 {
		a := mustUpsert(t, u, newAccount(fmt.Sprintf("f%d", i)))
		if _, err := svc.Follow(t.Context(), a.ID, target.ID); err != nil {
			t.Fatal(err)
		}
		var fl store.Follow
		_ = u.Tx(t.Context(), func(tx store.Tx) error {
			var err error
			fl, err = tx.Follows().Get(t.Context(), a.ID, target.ID)
			return err
		})
		ids = append(ids, fl.ID)
	}
	var got []string
	maxID := ""
	for {
		var page []store.Follow
		if err := u.Tx(t.Context(), func(tx store.Tx) error {
			var err error
			page, err = tx.Follows().ListFollowers(t.Context(), target.ID, maxID, 3)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, fl := range page {
			got = append(got, fl.ID)
		}
		maxID = page[len(page)-1].ID
	}
	if len(got) != len(ids) {
		t.Fatalf("got %d follows, want %d", len(got), len(ids))
	}
	for i := range ids {
		if got[i] != ids[len(ids)-1-i] {
			t.Fatalf("order mismatch at %d: got %v, want reverse of %v", i, got, ids)
		}
	}
}

func testRecount(t *testing.T, f Factory) {
	u := f(t, txretry.Policy{})
	alice, bob := mustUpsert(t, u, newAccount("alice")), mustUpsert(t, u, newAccount("bob"))
	svc := &app.Follows{UoW: u}
	if _, err := svc.Follow(t.Context(), alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if err := u.Tx(t.Context(), func(tx store.Tx) error { // simulate drift
		return tx.Accounts().AddFollowCounts(t.Context(), bob.ID, 41, -7)
	}); err != nil {
		t.Fatal(err)
	}
	if err := u.Tx(t.Context(), func(tx store.Tx) error {
		return tx.Accounts().RecountFollowCounts(t.Context(), bob.ID)
	}); err != nil {
		t.Fatal(err)
	}
	wantCounts(t, u, bob.ID, 1, 0)
}

func testRetryBounded(t *testing.T, f Factory) {
	var retries atomic.Int64
	u := f(t, txretry.Policy{MaxAttempts: 3, OnRetry: func(int, error) { retries.Add(1) }})
	calls := 0
	err := u.Tx(t.Context(), func(store.Tx) error {
		calls++
		return fmt.Errorf("conflict: %w", store.ErrRetryable)
	})
	if !errors.Is(err, store.ErrRetryable) || calls != 3 || retries.Load() != 2 {
		t.Fatalf("retryable: err=%v calls=%d retries=%d, want ErrRetryable/3/2", err, calls, retries.Load())
	}

	calls = 0
	boom := errors.New("boom")
	err = u.Tx(t.Context(), func(store.Tx) error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("non-retryable: err=%v calls=%d, want boom/1", err, calls)
	}

	calls = 0
	err = u.Tx(t.Context(), func(store.Tx) error {
		calls++
		if calls == 1 {
			return store.ErrRetryable
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("retry then succeed: err=%v calls=%d", err, calls)
	}
}

const concurrency = 16

func testConcurrentFollowDistinct(t *testing.T, f Factory) {
	p, retries := retryCounter()
	u := f(t, p)
	target := mustUpsert(t, u, newAccount("popular"))
	followers := make([]store.Account, concurrency)
	for i := range followers {
		followers[i] = mustUpsert(t, u, newAccount(fmt.Sprintf("fan%d", i)))
	}
	svc := &app.Follows{UoW: u}
	runParallel(t, concurrency, func(i int) error {
		created, err := svc.Follow(context.WithoutCancel(t.Context()), followers[i].ID, target.ID)
		if err == nil && !created {
			err = errors.New("not created")
		}
		return err
	})
	wantCounts(t, u, target.ID, concurrency, 0)
	t.Logf("retries observed: %d", retries.Load())
}

func testConcurrentFollowSamePair(t *testing.T, f Factory) {
	p, retries := retryCounter()
	u := f(t, p)
	alice, bob := mustUpsert(t, u, newAccount("alice")), mustUpsert(t, u, newAccount("bob"))
	svc := &app.Follows{UoW: u}
	var created atomic.Int64
	runParallel(t, concurrency, func(int) error {
		ok, err := svc.Follow(t.Context(), alice.ID, bob.ID)
		if ok {
			created.Add(1)
		}
		return err
	})
	if created.Load() != 1 {
		t.Fatalf("created %d times, want exactly 1", created.Load())
	}
	wantCounts(t, u, bob.ID, 1, 0)
	t.Logf("retries observed: %d", retries.Load())
}

func testConcurrentUpsertSameURI(t *testing.T, f Factory) {
	p, retries := retryCounter()
	u := f(t, p)
	base := newAccount("remote")
	ids := make([]string, concurrency)
	runParallel(t, concurrency, func(i int) error {
		a := base
		a.ID = uuid.NewV7().String()
		return u.Tx(t.Context(), func(tx store.Tx) error {
			got, err := tx.Accounts().Upsert(t.Context(), a)
			ids[i] = got.ID
			return err
		})
	})
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("upsert by URI returned different IDs: %v", ids)
		}
	}
	t.Logf("retries observed: %d", retries.Load())
}

func runParallel(t *testing.T, n int, fn func(i int) error) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			errs[i] = fn(i)
		})
	}
	close(start)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
}
