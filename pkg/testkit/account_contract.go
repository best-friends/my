// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/identity"
)

// AccountHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the identity and account repositories on it.
type AccountHarness struct {
	UnitOfWork uow.UnitOfWork
	Identity   appidentity.Repository
	Accounts   appaccount.Repository
}

// AccountFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type AccountFactory func(t *testing.T, opts uow.Options) AccountHarness

// AccountRepositoryContract verifies the identity and account repositories of a database
// adapter through the use cases that drive them: registering a local user with sealed RSA and
// Ed25519 keys, rejecting a taken username, creating the instance actor exactly once, lookups by
// handle and ID, profile revisions, lifecycle events, the built-in roles, and the property that
// the account module's synchronous projections equal a rebuild from their events.
func AccountRepositoryContract(t *testing.T, newHarness AccountFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *accountEnv)
	}{
		{"RegistersALocalUser", accountRegisters},
		{"RejectsATakenUsername", accountRejectsTakenUsername},
		{"RejectsInvalidInput", accountRejectsInvalidInput},
		{"CreatesTheInstanceActorOnce", accountCreatesInstanceActorOnce},
		{"ReportsMissingAccounts", accountReportsMissing},
		{"RevisesProfiles", accountRevisesProfiles},
		{"RecordsLifecycleEvents", accountRecordsLifecycle},
		{"EnsuresBuiltinRoles", accountEnsuresBuiltinRoles},
		{"IncrementalProjectionEqualsRebuild", accountProjectionEqualsRebuild},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newAccountEnv(t, newHarness)) })
	}
}

// accountEnv wires the use cases onto one harness with deterministic time and IDs.
type accountEnv struct {
	h        AccountHarness
	clock    *Clock
	accounts *appaccount.Service
	identity *appidentity.Service
	hasher   *appidentity.PasswordHasher
}

const contractHost = "social.example"

// cheapArgon2 keeps the contract fast; the parameters do not affect what is stored.
var cheapArgon2 = appidentity.Argon2Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func newAccountEnv(t *testing.T, newHarness AccountFactory) *accountEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	appaccount.RegisterProjectors(projectors, h.Accounts)
	clock := NewClock(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	ids := NewIDGenerator(clock)
	accounts := appaccount.NewService(appaccount.Deps{
		UnitOfWork:  h.UnitOfWork,
		Repository:  h.Accounts,
		Clock:       clock,
		IDs:         ids,
		Sealer:      kernel.NewSealer(NewKeyManager("contract")),
		Host:        contractHost,
		GenerateKey: cachedKey,
	})
	hasher := appidentity.NewPasswordHasher(cheapArgon2, 0)
	return &accountEnv{
		h:        h,
		clock:    clock,
		accounts: accounts,
		hasher:   hasher,
		identity: appidentity.NewService(appidentity.Deps{
			UnitOfWork: h.UnitOfWork,
			Repository: h.Identity,
			Clock:      clock,
			IDs:        ids,
			Passwords:  hasher,
			Accounts:   accounts,
		}),
	}
}

// cachedKey generates one key per algorithm per process: RSA generation is slow, and the
// contract does not need distinct keys.
func cachedKey(alg account.KeyAlgorithm) (crypto.Signer, error) {
	keyCache.once.Do(func() {
		keyCache.keys = map[account.KeyAlgorithm]crypto.Signer{}
		for _, a := range appaccount.LocalKeyAlgorithms {
			k, err := appaccount.GenerateKey(a)
			if err != nil {
				keyCache.err = err
				return
			}
			keyCache.keys[a] = k
		}
	})
	if keyCache.err != nil {
		return nil, keyCache.err
	}
	return keyCache.keys[alg], nil
}

var keyCache struct {
	once sync.Once
	keys map[account.KeyAlgorithm]crypto.Signer
	err  error
}

func (e *accountEnv) register(t *testing.T, username string) appidentity.Registration {
	t.Helper()
	e.clock.Advance(time.Second)
	reg, err := e.identity.RegisterLocalUser(t.Context(), appidentity.RegisterInput{
		Username: username, Password: "correct horse battery staple", DisplayName: "Display " + username,
	})
	if err != nil {
		t.Fatalf("RegisterLocalUser(%q) = %v", username, err)
	}
	return reg
}

func accountRegisters(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "Alice")

	user, err := e.h.Identity.FindUser(ctx, reg.User.ID)
	if err != nil || !user.CreatedAt.Equal(reg.User.CreatedAt) {
		t.Fatalf("FindUser = %+v, %v; want %+v", user, err, reg.User)
	}
	cred, err := e.h.Identity.FindCurrentPasswordCredential(ctx, reg.User.ID)
	if err != nil {
		t.Fatalf("FindCurrentPasswordCredential = %v", err)
	}
	if ok, _, err := e.hasher.Verify(ctx, "correct horse battery staple", cred.Hash); !ok || err != nil {
		t.Errorf("stored hash %q does not verify the password: %v, %v", cred.Hash, ok, err)
	}

	for _, name := range []string{"alice", "ALICE", "Alice"} {
		got, err := e.accounts.FindByHandle(ctx, name, account.LocalDomain)
		if err != nil || got.Account.ID != reg.AccountID {
			t.Fatalf("FindByHandle(%q) = %+v, %v; want account %s", name, got.Account, err, reg.AccountID)
		}
	}
	actor, err := e.accounts.FindByID(ctx, reg.AccountID)
	if err != nil {
		t.Fatalf("FindByID = %v", err)
	}
	a := actor.Account
	if a.URI != "https://"+contractHost+"/ap/actors/"+a.ID || a.Domain != account.LocalDomain || a.ActorType != account.ActorPerson {
		t.Errorf("account = %+v, want a local person at /ap/actors/{id}", a)
	}
	if actor.Local == nil || actor.Local.UserID != reg.User.ID || actor.Local.Username != "Alice" || actor.Local.UsernameNormalized != "alice" {
		t.Errorf("local account = %+v, want user %s and username Alice", actor.Local, reg.User.ID)
	}
	if actor.Handle == nil || *actor.Handle != (account.Handle{AccountID: a.ID, Username: "Alice", UsernameNormalized: "alice", Domain: account.LocalDomain}) {
		t.Errorf("handle = %+v", actor.Handle)
	}
	if actor.Profile == nil || actor.Profile.Seq != 1 || actor.Profile.DisplayName != "Display Alice" {
		t.Errorf("profile = %+v, want revision 1 with the display name", actor.Profile)
	}
	if actor.Lifecycle.State != account.StateActive {
		t.Errorf("lifecycle = %+v, want active", actor.Lifecycle)
	}
	checkKeys(t, e, actor)
}

// checkKeys verifies that an actor has an RSA main key and an Ed25519 key whose sealed private
// halves open to the stored public keys.
func checkKeys(t *testing.T, e *accountEnv, actor appaccount.Actor) {
	t.Helper()
	if len(actor.Keys) != 2 {
		t.Fatalf("keys = %+v, want an RSA and an Ed25519 key", actor.Keys)
	}
	seen := map[account.KeyAlgorithm]bool{}
	for _, k := range actor.Keys {
		seen[k.Algorithm] = true
		if k.KeyID != account.KeyIRI(actor.Account.URI, k.Algorithm) || k.AccountID != actor.Account.ID {
			t.Errorf("key %+v: wrong key ID or account", k)
		}
		pub, err := appaccount.ParsePublicKeyPEM(k.PublicKeyPEM)
		if err != nil {
			t.Fatalf("parse public key %s: %v", k.KeyID, err)
		}
		priv, err := e.accounts.PrivateKey(t.Context(), k)
		if err != nil {
			t.Fatalf("PrivateKey(%s) = %v", k.KeyID, err)
		}
		switch p := priv.(type) {
		case *rsa.PrivateKey:
			if k.Algorithm != account.KeyRSASHA256 || p.N.BitLen() != appaccount.RSAKeyBits || !p.PublicKey.Equal(pub) {
				t.Errorf("key %s: RSA private key does not match", k.KeyID)
			}
		case ed25519.PrivateKey:
			if k.Algorithm != account.KeyEd25519 || !p.Public().(ed25519.PublicKey).Equal(pub) {
				t.Errorf("key %s: Ed25519 private key does not match", k.KeyID)
			}
		default:
			t.Errorf("key %s: private key is a %T", k.KeyID, priv)
		}
	}
	if !seen[account.KeyRSASHA256] || !seen[account.KeyEd25519] {
		t.Errorf("key algorithms = %v, want rsa_sha256 and ed25519", seen)
	}
}

func accountRejectsTakenUsername(t *testing.T, e *accountEnv) {
	e.register(t, "bob")
	_, err := e.identity.RegisterLocalUser(t.Context(), appidentity.RegisterInput{Username: "BOB", Password: "another password"})
	if !errors.Is(err, appaccount.ErrUsernameTaken) {
		t.Fatalf("RegisterLocalUser(BOB) = %v, want ErrUsernameTaken", err)
	}
	handles, err := e.h.Accounts.ListHandles(t.Context())
	if err != nil || len(handles) != 1 {
		t.Fatalf("handles = %+v, %v; want only bob (the failed registration must roll back)", handles, err)
	}
}

func accountRejectsInvalidInput(t *testing.T, e *accountEnv) {
	for _, in := range []appidentity.RegisterInput{
		{Username: "", Password: "long enough"},
		{Username: "bad name", Password: "long enough"},
		{Username: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Password: "long enough"},
	} {
		if _, err := e.identity.RegisterLocalUser(t.Context(), in); !errors.Is(err, account.ErrInvalidUsername) {
			t.Errorf("RegisterLocalUser(%q) = %v, want ErrInvalidUsername", in.Username, err)
		}
	}
	if _, err := e.identity.RegisterLocalUser(t.Context(), appidentity.RegisterInput{Username: "carol", Password: "short"}); !errors.Is(err, identity.ErrInvalidPassword) {
		t.Errorf("RegisterLocalUser with a short password = %v, want ErrInvalidPassword", err)
	}
	if handles, _ := e.h.Accounts.ListHandles(t.Context()); len(handles) != 0 {
		t.Errorf("handles = %+v after invalid input, want none", handles)
	}
}

func accountCreatesInstanceActorOnce(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	if _, err := e.accounts.InstanceActor(ctx); !errors.Is(err, appaccount.ErrNotFound) {
		t.Fatalf("InstanceActor before creation = %v, want ErrNotFound", err)
	}
	const callers = 4
	ids := make(chan string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			a, err := e.accounts.EnsureInstanceActor(ctx)
			if err != nil {
				t.Errorf("EnsureInstanceActor = %v", err)
			}
			ids <- a.ID
		})
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("EnsureInstanceActor returned %s and %s, want one actor", first, id)
		}
	}
	again, err := e.accounts.EnsureInstanceActor(ctx)
	if err != nil || again.ID != first {
		t.Fatalf("EnsureInstanceActor again = %s, %v; want %s", again.ID, err, first)
	}
	actor, err := e.accounts.InstanceActor(ctx)
	if err != nil {
		t.Fatalf("InstanceActor = %v", err)
	}
	if actor.Account.ID != first || actor.Account.ActorType != account.ActorApplication || !actor.Account.IsLocal() {
		t.Errorf("instance actor = %+v, want the local Application actor %s", actor.Account, first)
	}
	if actor.Local != nil || actor.Handle != nil {
		t.Errorf("instance actor has local account %+v and handle %+v, want neither", actor.Local, actor.Handle)
	}
	checkKeys(t, e, actor)
}

func accountReportsMissing(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	missing := "00000000-0000-7000-8000-000000000000"
	if _, err := e.accounts.FindByID(ctx, missing); !errors.Is(err, appaccount.ErrNotFound) {
		t.Errorf("FindByID(missing) = %v, want ErrNotFound", err)
	}
	if _, err := e.accounts.FindByHandle(ctx, "nobody", account.LocalDomain); !errors.Is(err, appaccount.ErrNotFound) {
		t.Errorf("FindByHandle(nobody) = %v, want ErrNotFound", err)
	}
	if _, err := e.h.Identity.FindUser(ctx, missing); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindUser(missing) = %v, want ErrNotFound", err)
	}
	if _, err := e.h.Identity.FindCurrentPasswordCredential(ctx, missing); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindCurrentPasswordCredential(missing) = %v, want ErrNotFound", err)
	}
	if _, _, err := e.h.Identity.FindRole(ctx, "nope"); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("FindRole(nope) = %v, want ErrNotFound", err)
	}
}

func accountRevisesProfiles(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "dave")
	in := appaccount.ProfileInput{
		DisplayName: "Dave", NoteHTML: "<p>hi</p>", Locked: true, Bot: true, Discoverable: true, Indexable: true,
		AvatarMediaID: "0192f3a4-0000-7000-8000-000000000001",
		Fields:        []account.ProfileField{{Name: "web", ValueHTML: "<a>x</a>"}, {Name: "pronouns", ValueHTML: "he/him"}},
	}
	e.clock.Advance(time.Second)
	rev, err := e.accounts.ReviseProfile(ctx, reg.AccountID, in)
	if err != nil || rev.Seq != 2 {
		t.Fatalf("ReviseProfile = %+v, %v; want seq 2", rev, err)
	}
	actor, err := e.accounts.FindByID(ctx, reg.AccountID)
	if err != nil || actor.Profile == nil {
		t.Fatalf("FindByID = %+v, %v", actor, err)
	}
	got := *actor.Profile
	if !got.RevisedAt.Equal(rev.RevisedAt) {
		t.Errorf("RevisedAt = %v, want %v", got.RevisedAt, rev.RevisedAt)
	}
	got.RevisedAt = rev.RevisedAt
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("current profile = %+v, want %+v", got, rev)
	}
	if got.Fields[1].Position != 1 {
		t.Errorf("field positions = %+v, want 0 and 1", got.Fields)
	}

	// Concurrent revisions each take the next seq; none is lost.
	const writers = 8
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			if _, err := e.accounts.ReviseProfile(ctx, reg.AccountID, appaccount.ProfileInput{DisplayName: "concurrent"}); err != nil {
				t.Errorf("concurrent ReviseProfile = %v", err)
			}
		})
	}
	wg.Wait()
	if actor, err := e.accounts.FindByID(ctx, reg.AccountID); err != nil || actor.Profile == nil || actor.Profile.Seq != 2+writers {
		t.Errorf("after %d concurrent revisions: profile = %+v, %v; want seq %d", writers, actor.Profile, err, 2+writers)
	}
}

func accountRecordsLifecycle(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "erin")
	record := func(kind account.LifecycleKind, due *time.Time) account.LifecycleEvent {
		t.Helper()
		e.clock.Advance(time.Second)
		ev, err := e.accounts.RecordLifecycleEvent(ctx, appaccount.LifecycleInput{
			AccountID: reg.AccountID, Kind: kind, InitiatedBy: account.InitiatedBySelf, DeletionDueAt: due,
		})
		if err != nil {
			t.Fatalf("RecordLifecycleEvent(%s) = %v", kind, err)
		}
		return ev
	}
	state := func() account.LifecycleState {
		t.Helper()
		actor, err := e.accounts.FindByID(ctx, reg.AccountID)
		if err != nil {
			t.Fatalf("FindByID = %v", err)
		}
		return actor.Lifecycle
	}

	ev := record(account.LifecycleDeactivated, nil)
	if s := state(); s.State != account.StateDeactivated || !s.Since.Equal(ev.OccurredAt) {
		t.Errorf("after deactivated: %+v", s)
	}
	due := e.clock.Now().Add(30 * 24 * time.Hour)
	record(account.LifecycleDeletionScheduled, &due)
	if s := state(); s.State != account.StateDeletionScheduled || s.DeletionDueAt == nil || !s.DeletionDueAt.Equal(due) {
		t.Errorf("after deletion_scheduled: %+v, want due %v", s, due)
	}
	record(account.LifecycleDeletionCancelled, nil)
	if s := state(); s.State != account.StateActive || s.DeletionDueAt != nil {
		t.Errorf("after deletion_cancelled: %+v", s)
	}
	if _, err := e.accounts.RecordLifecycleEvent(ctx, appaccount.LifecycleInput{
		AccountID: reg.AccountID, Kind: account.LifecycleDeletionScheduled, InitiatedBy: account.InitiatedBySelf,
	}); err == nil {
		t.Error("RecordLifecycleEvent(deletion_scheduled) without a due time succeeded")
	}
}

func accountEnsuresBuiltinRoles(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	if err := e.identity.EnsureBuiltinRoles(ctx); err != nil {
		t.Fatalf("EnsureBuiltinRoles = %v", err)
	}
	first := map[string]identity.Role{}
	for _, b := range identity.BuiltinRoles() {
		r, _, err := e.h.Identity.FindRole(ctx, b.Key)
		if err != nil {
			t.Fatalf("FindRole(%s) = %v", b.Key, err)
		}
		first[b.Key] = r
	}
	e.clock.Advance(time.Hour)
	if err := e.identity.EnsureBuiltinRoles(ctx); err != nil {
		t.Fatalf("EnsureBuiltinRoles again = %v", err)
	}
	for _, b := range identity.BuiltinRoles() {
		r, perms, err := e.h.Identity.FindRole(ctx, b.Key)
		if err != nil {
			t.Fatalf("FindRole(%s) = %v", b.Key, err)
		}
		if r.ID != first[b.Key].ID || !r.CreatedAt.Equal(first[b.Key].CreatedAt) || r.Position != b.Position {
			t.Errorf("role %s = %+v after a second run, want %+v", b.Key, r, first[b.Key])
		}
		want := map[identity.Permission]bool{}
		for _, p := range b.Permissions {
			want[p] = true
		}
		got := map[identity.Permission]bool{}
		for _, p := range perms {
			got[p] = true
		}
		if !reflect.DeepEqual(got, want) || len(perms) != len(b.Permissions) {
			t.Errorf("role %s permissions = %v, want %v", b.Key, perms, b.Permissions)
		}
	}
}

// accountProjectionEqualsRebuild runs random sequences of commands, which update the projections
// incrementally through the synchronous projectors, and checks that rebuilding the projections
// from their events yields exactly the same rows (data-model §9).
func accountProjectionEqualsRebuild(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	rng := rand.New(rand.NewPCG(58, 20261016)) //nolint:gosec // deterministic test input, not a secret
	kinds := []account.LifecycleKind{
		account.LifecycleDeactivated, account.LifecycleReactivated, account.LifecycleDeletionScheduled,
		account.LifecycleDeletionCancelled, account.LifecycleDeleted, account.LifecycleRemoteDeleted,
	}
	var ids []string
	for round := range 3 {
		for step := range 40 {
			e.clock.Advance(time.Duration(rng.IntN(5000)+1) * time.Millisecond)
			switch op := rng.IntN(10); {
			case op < 2 || len(ids) == 0:
				reg := e.register(t, fmt.Sprintf("u%d_%d", round, step))
				ids = append(ids, reg.AccountID)
			case op < 6:
				id := ids[rng.IntN(len(ids))]
				fields := make([]account.ProfileField, rng.IntN(4))
				for i := range fields {
					fields[i] = account.ProfileField{Name: fmt.Sprint("f", i), ValueHTML: fmt.Sprint(rng.IntN(100))}
				}
				in := appaccount.ProfileInput{DisplayName: fmt.Sprint("name ", step), Locked: rng.IntN(2) == 0, Fields: fields}
				if _, err := e.accounts.ReviseProfile(ctx, id, in); err != nil {
					t.Fatalf("ReviseProfile = %v", err)
				}
			default:
				in := appaccount.LifecycleInput{
					AccountID:   ids[rng.IntN(len(ids))],
					Kind:        kinds[rng.IntN(len(kinds))],
					InitiatedBy: account.InitiatedBySystem,
				}
				if in.Kind == account.LifecycleDeletionScheduled {
					due := e.clock.Now().Add(time.Duration(rng.IntN(30)+1) * 24 * time.Hour)
					in.DeletionDueAt = &due
				}
				if _, err := e.accounts.RecordLifecycleEvent(ctx, in); err != nil {
					t.Fatalf("RecordLifecycleEvent = %v", err)
				}
			}
		}
		incremental := snapshotAccountProjections(t, e.h.Accounts)
		if len(incremental.handles) == 0 || len(incremental.heads) == 0 {
			t.Fatalf("round %d: projections are empty: %+v", round, incremental)
		}
		if err := appaccount.RebuildProjections(ctx, e.h.UnitOfWork, e.h.Accounts); err != nil {
			t.Fatalf("RebuildProjections = %v", err)
		}
		rebuilt := snapshotAccountProjections(t, e.h.Accounts)
		if !reflect.DeepEqual(incremental, rebuilt) {
			t.Fatalf("round %d: incremental projections differ from a rebuild:\nincremental %+v\nrebuilt     %+v", round, incremental, rebuilt)
		}
	}
}

type accountProjections struct {
	handles []account.Handle
	heads   []account.ProfileHead
	states  []account.LifecycleState
}

func snapshotAccountProjections(t *testing.T, repo appaccount.Repository) accountProjections {
	t.Helper()
	var (
		p   accountProjections
		err error
	)
	if p.handles, err = repo.ListHandles(t.Context()); err != nil {
		t.Fatalf("ListHandles = %v", err)
	}
	if p.heads, err = repo.ListProfileHeads(t.Context()); err != nil {
		t.Fatalf("ListProfileHeads = %v", err)
	}
	if p.states, err = repo.ListLifecycleStates(t.Context()); err != nil {
		t.Fatalf("ListLifecycleStates = %v", err)
	}
	return p
}
