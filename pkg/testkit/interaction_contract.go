// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/graph"
)

// InteractionHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the content module's status and interaction repositories on it.
type InteractionHarness struct {
	UnitOfWork   uow.UnitOfWork
	Statuses     appcontent.Repository
	Interactions appcontent.InteractionRepository
}

// InteractionFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type InteractionFactory func(t *testing.T, opts uow.Options) InteractionHarness

// InteractionRepositoryContract verifies the interaction repository of a database adapter
// together with the content module's projectors and its Interactions use cases: boosts and
// their retractions with boost_edges, favourites, bookmarks, and pins; the visibility check
// through content.CanView; counters that move only when a row is actually written, also under
// concurrent and repeated commands; the events for other modules' projectors; status deletions;
// and the property that incrementally updated projections equal a rebuild from the sources.
func InteractionRepositoryContract(t *testing.T, newHarness InteractionFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *interactionEnv)
	}{
		{"RejectsDuplicates", interactionRejectsDuplicates},
		{"BoostsAndRetracts", interactionBoosts},
		{"BoostsCopyVisibility", interactionBoostsCopyVisibility},
		{"FavouritesAndUnfavourites", interactionFavourites},
		{"BookmarksAndPins", interactionBookmarksAndPins},
		{"LimitsConcurrentPins", interactionLimitsConcurrentPins},
		{"ChecksVisibility", interactionChecksVisibility},
		{"CountsConcurrentCommandsOnce", interactionCountsConcurrentOnce},
		{"ForgetsDeletedStatuses", interactionForgetsDeletedStatuses},
		{"IncrementalProjectionEqualsRebuild", interactionProjectionEqualsRebuild},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newInteractionEnv(t, newHarness)) })
	}
}

// interactionEnv drives one harness with deterministic time and IDs, stub account and graph
// lookups, and a recorder of the interaction events.
type interactionEnv struct {
	*statusEnv
	h         InteractionHarness
	svc       *appcontent.Interactions
	actors    *stubActors
	relations *stubRelations

	mu     sync.Mutex
	events []uow.Event
}

func newInteractionEnv(t *testing.T, newHarness InteractionFactory) *interactionEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	appcontent.RegisterProjectors(projectors, h.Statuses)
	appcontent.RegisterInteractionProjectors(projectors, h.Interactions)
	clock := NewClock(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	ids := NewIDGenerator(clock)
	e := &interactionEnv{
		statusEnv: &statusEnv{h: StatusHarness{UnitOfWork: h.UnitOfWork, Statuses: h.Statuses}, clock: clock, ids: ids},
		h:         h,
		actors:    &stubActors{actors: map[string]appaccount.Actor{}},
		relations: &stubRelations{follows: map[[2]string]graph.State{}, blocks: map[[2]string]bool{}},
	}
	for _, typ := range []string{appcontent.EventBoosted, appcontent.EventBoostRetracted, appcontent.EventFavourited, appcontent.EventUnfavourited} {
		projectors.Register(typ, func(_ context.Context, _ uow.Tx, ev uow.Event) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.events = append(e.events, ev)
			return nil
		})
	}
	e.svc = appcontent.NewInteractions(appcontent.InteractionDeps{
		UnitOfWork: h.UnitOfWork, Statuses: h.Statuses, Interactions: h.Interactions,
		Accounts: e.actors, Relationships: e.relations, Clock: clock, IDs: ids, Host: contractHost,
	})
	return e
}

// takeEvents returns the event types recorded since the last call; the projectors record an
// event only when its transaction commits or retries, so callers compare after the command.
func (e *interactionEnv) takeEvents() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.events))
	for i, ev := range e.events {
		out[i] = ev.EventType()
	}
	e.events = nil
	return out
}

// account registers an account with the stub lookup and returns its ID.
func (e *interactionEnv) account(local bool, state account.LifecycleStateName) string {
	id := e.ids.NewID()
	domain := account.LocalDomain
	if !local {
		domain = "remote.example"
	}
	e.actors.set(appaccount.Actor{
		Account:   account.Account{ID: id, Domain: domain},
		Lifecycle: account.LifecycleState{AccountID: id, State: state},
	})
	return id
}

func (e *interactionEnv) localAccount() string { return e.account(true, account.StateActive) }

// post creates a status of author with mentions and returns it.
func (e *interactionEnv) post(t *testing.T, author string, v content.Visibility, mentions ...string) content.Status {
	t.Helper()
	s := e.newStatus(author, v)
	s.IsLocal = e.actors.local(author)
	e.mustCreate(t, s, mentions...)
	return s
}

func (e *interactionEnv) follow(follower, followee string, state graph.State) {
	e.relations.set(follower, followee, state)
}

type stubActors struct {
	mu     sync.Mutex
	actors map[string]appaccount.Actor
}

func (s *stubActors) set(a appaccount.Actor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actors[a.Account.ID] = a
}

func (s *stubActors) local(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.actors[id].Account.IsLocal()
}

func (s *stubActors) FindByHandle(context.Context, string, string) (appaccount.Actor, error) {
	return appaccount.Actor{}, appaccount.ErrNotFound
}

func (s *stubActors) FindByID(_ context.Context, id string) (appaccount.Actor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actors[id]
	if !ok {
		return appaccount.Actor{}, appaccount.ErrNotFound
	}
	return a, nil
}

type stubRelations struct {
	mu      sync.Mutex
	follows map[[2]string]graph.State
	blocks  map[[2]string]bool
}

func (s *stubRelations) block(blocker, blocked string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks[[2]string{blocker, blocked}] = true
}

func (s *stubRelations) set(follower, followee string, state graph.State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.follows[[2]string{follower, followee}] = state
}

func (s *stubRelations) Relationship(_ context.Context, accountID, targetID string) (appgraph.Relationship, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := func(a, b string) graph.State {
		if st, ok := s.follows[[2]string{a, b}]; ok {
			return st
		}
		return graph.StateNone
	}
	return appgraph.Relationship{
		AccountID: accountID, TargetAccountID: targetID,
		Following: state(accountID, targetID), FollowedBy: state(targetID, accountID),
		Blocking: s.blocks[[2]string{accountID, targetID}], BlockedBy: s.blocks[[2]string{targetID, accountID}],
	}, nil
}

func interactionRejectsDuplicates(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author := e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	b := content.Boost{
		ID: e.ids.NewID(), BoosterAccountID: author, StatusID: s.ID, Visibility: s.Visibility, IsLocal: true, BoostedAt: e.clock.Now(),
	}
	b.URI = content.ActivityURI(contractHost, b.ID)
	tx := func(fn func(tx uow.Tx) error) error { return e.h.UnitOfWork.Tx(ctx, fn) }
	if err := tx(func(tx uow.Tx) error { return e.h.Interactions.InsertBoost(ctx, tx, b) }); err != nil {
		t.Fatalf("InsertBoost = %v", err)
	}
	again := b
	again.ID = e.ids.NewID()
	if err := tx(func(tx uow.Tx) error { return e.h.Interactions.InsertBoost(ctx, tx, again) }); !errors.Is(err, appcontent.ErrBoostExists) {
		t.Errorf("InsertBoost(same URI) = %v, want ErrBoostExists", err)
	}
	r := content.BoostRetraction{BoostID: b.ID, Origin: content.RetractedByBooster, RetractedAt: e.clock.Now()}
	if err := tx(func(tx uow.Tx) error { return e.h.Interactions.InsertBoostRetraction(ctx, tx, r) }); err != nil {
		t.Fatalf("InsertBoostRetraction = %v", err)
	}
	if err := tx(func(tx uow.Tx) error { return e.h.Interactions.InsertBoostRetraction(ctx, tx, r) }); !errors.Is(err, appcontent.ErrBoostRetracted) {
		t.Errorf("InsertBoostRetraction(again) = %v, want ErrBoostRetracted", err)
	}
	if got, err := e.h.Interactions.FindBoostByURI(ctx, b.URI); err != nil || !reflect.DeepEqual(got, b) {
		t.Errorf("FindBoostByURI = %+v, %v; want %+v", got, err, b)
	}
	if got, err := e.h.Interactions.FindBoostRetraction(ctx, b.ID); err != nil || !reflect.DeepEqual(got, r) {
		t.Errorf("FindBoostRetraction = %+v, %v; want %+v", got, err, r)
	}

	f := content.Favourite{ID: e.ids.NewID(), AccountID: author, StatusID: s.ID, URI: "https://remote.example/likes/1", CreatedAt: e.clock.Now()}
	other := e.post(t, author, content.VisibilityPublic)
	g := content.Favourite{ID: e.ids.NewID(), AccountID: author, StatusID: other.ID, URI: f.URI, CreatedAt: e.clock.Now()}
	if err := tx(func(tx uow.Tx) error { return second(e.h.Interactions.InsertFavourite(ctx, tx, f)) }); err != nil {
		t.Fatalf("InsertFavourite = %v", err)
	}
	if err := tx(func(tx uow.Tx) error { return second(e.h.Interactions.InsertFavourite(ctx, tx, g)) }); !errors.Is(err, appcontent.ErrFavouriteURIUsed) {
		t.Errorf("InsertFavourite(same URI) = %v, want ErrFavouriteURIUsed", err)
	}

	for name, err := range map[string]error{
		"FindBoost":           second(e.h.Interactions.FindBoost(ctx, "missing")),
		"FindBoostRetraction": second(e.h.Interactions.FindBoostRetraction(ctx, s.ID)),
		"FindBoostEdge":       second(e.h.Interactions.FindBoostEdge(ctx, author, s.ID)),
		"FindFavourite":       second(e.h.Interactions.FindFavourite(ctx, author, "missing")),
		"FindBookmark":        second(e.h.Interactions.FindBookmark(ctx, author, s.ID)),
		"FindPin":             second(e.h.Interactions.FindPin(ctx, author, s.ID)),
	} {
		if !errors.Is(err, appcontent.ErrNotFound) {
			t.Errorf("%s(missing) = %v, want ErrNotFound", name, err)
		}
	}
}

func interactionBoosts(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, booster := e.localAccount(), e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	e.takeEvents()

	b, err := e.svc.Boost(ctx, booster, s.ID)
	if err != nil {
		t.Fatalf("Boost = %v", err)
	}
	want := content.Boost{
		ID: b.ID, URI: "https://" + contractHost + "/ap/activities/" + b.ID, BoosterAccountID: booster, StatusID: s.ID,
		Visibility: content.VisibilityPublic, IsLocal: true, BoostedAt: b.BoostedAt,
	}
	if !reflect.DeepEqual(b, want) || !b.Federates() {
		t.Errorf("Boost = %+v, want %+v that federates", b, want)
	}
	if got, err := e.h.Interactions.FindBoost(ctx, b.ID); err != nil || !reflect.DeepEqual(got, b) {
		t.Errorf("FindBoost = %+v, %v; want %+v", got, err, b)
	}
	if got, err := e.h.Interactions.FindBoostEdge(ctx, booster, s.ID); err != nil || got != content.EdgeOf(b) {
		t.Errorf("FindBoostEdge = %+v, %v; want %+v", got, err, content.EdgeOf(b))
	}
	if c := e.counters(t, s.ID); c.BoostsCount != 1 {
		t.Errorf("BoostsCount = %d, want 1", c.BoostsCount)
	}
	if got := e.takeEvents(); !reflect.DeepEqual(got, []string{appcontent.EventBoosted}) {
		t.Errorf("events = %v, want one %s", got, appcontent.EventBoosted)
	}

	again, err := e.svc.Boost(ctx, booster, s.ID)
	if err != nil || again != b {
		t.Errorf("Boost(again) = %+v, %v; want the existing %+v", again, err, b)
	}
	if c := e.counters(t, s.ID); c.BoostsCount != 1 {
		t.Errorf("BoostsCount after a repeated boost = %d, want 1", c.BoostsCount)
	}
	if got := e.takeEvents(); len(got) != 0 {
		t.Errorf("a repeated boost emitted %v", got)
	}

	if err := e.svc.Unboost(ctx, booster, s.ID); err != nil {
		t.Fatalf("Unboost = %v", err)
	}
	if r, err := e.h.Interactions.FindBoostRetraction(ctx, b.ID); err != nil || r.Origin != content.RetractedByBooster {
		t.Errorf("FindBoostRetraction = %+v, %v; want one by the booster", r, err)
	}
	if _, err := e.h.Interactions.FindBoostEdge(ctx, booster, s.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindBoostEdge after Unboost = %v, want ErrNotFound", err)
	}
	if err := e.svc.Unboost(ctx, booster, s.ID); err != nil {
		t.Errorf("Unboost(again) = %v", err)
	}
	if c := e.counters(t, s.ID); c.BoostsCount != 0 {
		t.Errorf("BoostsCount after Unboost = %d, want 0", c.BoostsCount)
	}
	if got := e.takeEvents(); !reflect.DeepEqual(got, []string{appcontent.EventBoostRetracted}) {
		t.Errorf("events = %v, want one %s", got, appcontent.EventBoostRetracted)
	}

	// Boosting again after a retraction is a new boost.
	re, err := e.svc.Boost(ctx, booster, s.ID)
	if err != nil || re.ID == b.ID {
		t.Errorf("Boost after Unboost = %+v, %v; want a new boost", re, err)
	}
	if c := e.counters(t, s.ID); c.BoostsCount != 1 {
		t.Errorf("BoostsCount after boosting again = %d, want 1", c.BoostsCount)
	}
	if err := e.svc.Unboost(ctx, booster, "missing"); err != nil {
		t.Errorf("Unboost(missing status) = %v", err)
	}
}

func interactionBoostsCopyVisibility(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, follower := e.localAccount(), e.localAccount()
	e.follow(follower, author, graph.StateActive)
	for _, tc := range []struct {
		v         content.Visibility
		booster   string
		err       error
		federates bool
	}{
		{content.VisibilityPublic, follower, nil, true},
		{content.VisibilityLocal, follower, nil, false},
		{content.VisibilityUnlisted, follower, nil, true},
		{content.VisibilityPrivate, follower, content.ErrNotBoostable, false},
		{content.VisibilityPrivate, author, nil, true},
		{content.VisibilityDirect, author, content.ErrNotBoostable, false},
	} {
		s := e.post(t, author, tc.v)
		b, err := e.svc.Boost(ctx, tc.booster, s.ID)
		if !errors.Is(err, tc.err) {
			t.Errorf("Boost(%s, author=%v) = %v, want %v", tc.v, tc.booster == author, err, tc.err)
			continue
		}
		if err != nil {
			continue
		}
		if b.Visibility != tc.v || b.Federates() != tc.federates {
			t.Errorf("Boost(%s) = visibility %s, federates %v; want %s, %v", tc.v, b.Visibility, b.Federates(), tc.v, tc.federates)
		}
	}
}

func interactionFavourites(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, fan := e.localAccount(), e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	t2 := e.post(t, author, content.VisibilityUnlisted)
	e.takeEvents()

	f, err := e.svc.Favourite(ctx, fan, s.ID)
	if err != nil {
		t.Fatalf("Favourite = %v", err)
	}
	if f.AccountID != fan || f.StatusID != s.ID || f.URI != content.ActivityURI(contractHost, f.ID) {
		t.Errorf("Favourite = %+v", f)
	}
	again, err := e.svc.Favourite(ctx, fan, s.ID)
	if err != nil || again != f {
		t.Errorf("Favourite(again) = %+v, %v; want the existing %+v", again, err, f)
	}
	if c := e.counters(t, s.ID); c.FavouritesCount != 1 {
		t.Errorf("FavouritesCount = %d, want 1", c.FavouritesCount)
	}
	if got := e.takeEvents(); !reflect.DeepEqual(got, []string{appcontent.EventFavourited}) {
		t.Errorf("events = %v, want one %s", got, appcontent.EventFavourited)
	}
	g, err := e.svc.Favourite(ctx, fan, t2.ID)
	if err != nil {
		t.Fatalf("Favourite(second status) = %v", err)
	}
	if got, err := e.h.Interactions.ListFavourites(ctx, fan, "", 10); err != nil || !reflect.DeepEqual(got, []content.Favourite{g, f}) {
		t.Errorf("ListFavourites = %+v, %v; want newest first", got, err)
	}
	if got, err := e.h.Interactions.ListFavourites(ctx, fan, g.ID, 10); err != nil || !reflect.DeepEqual(got, []content.Favourite{f}) {
		t.Errorf("ListFavourites(before) = %+v, %v; want the older one", got, err)
	}
	e.takeEvents()

	for range 2 {
		if err := e.svc.Unfavourite(ctx, fan, s.ID); err != nil {
			t.Fatalf("Unfavourite = %v", err)
		}
	}
	if c := e.counters(t, s.ID); c.FavouritesCount != 0 {
		t.Errorf("FavouritesCount after Unfavourite = %d, want 0", c.FavouritesCount)
	}
	if got := e.takeEvents(); !reflect.DeepEqual(got, []string{appcontent.EventUnfavourited}) {
		t.Errorf("events = %v, want one %s", got, appcontent.EventUnfavourited)
	}
	if _, err := e.h.Interactions.FindFavourite(ctx, fan, s.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindFavourite after Unfavourite = %v, want ErrNotFound", err)
	}
}

func interactionBookmarksAndPins(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, reader := e.localAccount(), e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	e.takeEvents()

	for range 2 {
		if err := e.svc.Bookmark(ctx, reader, s.ID); err != nil {
			t.Fatalf("Bookmark = %v", err)
		}
	}
	bm, err := e.h.Interactions.FindBookmark(ctx, reader, s.ID)
	if err != nil {
		t.Fatalf("FindBookmark = %v", err)
	}
	if got, err := e.h.Interactions.ListBookmarks(ctx, reader, "", 10); err != nil || !reflect.DeepEqual(got, []content.Bookmark{bm}) {
		t.Errorf("ListBookmarks = %+v, %v; want [%+v]", got, err, bm)
	}
	for range 2 {
		if err := e.svc.Unbookmark(ctx, reader, s.ID); err != nil {
			t.Fatalf("Unbookmark = %v", err)
		}
	}
	if _, err := e.h.Interactions.FindBookmark(ctx, reader, s.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindBookmark after Unbookmark = %v, want ErrNotFound", err)
	}

	if err := e.svc.Pin(ctx, reader, s.ID); !errors.Is(err, content.ErrNotPinnable) {
		t.Errorf("Pin(someone else's) = %v, want ErrNotPinnable", err)
	}
	if err := e.svc.Pin(ctx, author, e.post(t, author, content.VisibilityDirect).ID); !errors.Is(err, content.ErrNotPinnable) {
		t.Errorf("Pin(direct) = %v, want ErrNotPinnable", err)
	}
	var pinned []string
	for range content.MaxPins {
		p := e.post(t, author, content.VisibilityPublic)
		if err := e.svc.Pin(ctx, author, p.ID); err != nil {
			t.Fatalf("Pin = %v", err)
		}
		pinned = append(pinned, p.ID)
	}
	if err := e.svc.Pin(ctx, author, pinned[0]); err != nil {
		t.Errorf("Pin(already pinned, at the limit) = %v", err)
	}
	if err := e.svc.Pin(ctx, author, s.ID); !errors.Is(err, content.ErrPinLimit) {
		t.Errorf("Pin(beyond the limit) = %v, want ErrPinLimit", err)
	}
	pins, err := e.h.Interactions.ListPins(ctx, author)
	if err != nil || len(pins) != content.MaxPins || pins[0].StatusID != pinned[len(pinned)-1] {
		t.Errorf("ListPins = %+v, %v; want %d, newest first", pins, err, content.MaxPins)
	}
	for range 2 {
		if err := e.svc.Unpin(ctx, author, pinned[0]); err != nil {
			t.Fatalf("Unpin = %v", err)
		}
	}
	if _, err := e.h.Interactions.FindPin(ctx, author, pinned[0]); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindPin after Unpin = %v, want ErrNotFound", err)
	}
	if err := e.svc.Pin(ctx, author, s.ID); err != nil {
		t.Errorf("Pin after Unpin = %v", err)
	}
	if got := e.takeEvents(); len(got) != 0 {
		t.Errorf("bookmarks and pins emitted %v", got)
	}
}

// interactionLimitsConcurrentPins checks that concurrent pins of one account never exceed
// content.MaxPins: exactly MaxPins of them succeed and the rest fail with content.ErrPinLimit.
func interactionLimitsConcurrentPins(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author := e.localAccount()
	const writers = 2 * content.MaxPins
	posts := make([]string, writers)
	for i := range posts {
		posts[i] = e.post(t, author, content.VisibilityPublic).ID
	}
	pinAll := func(ids []string) (ok int) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Go(func() {
				err := e.svc.Pin(ctx, author, id)
				switch {
				case err == nil:
					mu.Lock()
					ok++
					mu.Unlock()
				case !errors.Is(err, content.ErrPinLimit):
					t.Errorf("concurrent Pin = %v, want nil or ErrPinLimit", err)
				}
			})
		}
		wg.Wait()
		return ok
	}
	if ok := pinAll(posts); ok != content.MaxPins {
		t.Errorf("%d concurrent pins: %d succeeded, want %d", writers, ok, content.MaxPins)
	}
	pins, err := e.h.Interactions.ListPins(ctx, author)
	if err != nil || len(pins) != content.MaxPins {
		t.Fatalf("ListPins = %d pins, %v; want %d", len(pins), err, content.MaxPins)
	}

	// Freeing one pin lets exactly one of the concurrent pins that follow take its place.
	if err := e.svc.Unpin(ctx, author, pins[0].StatusID); err != nil {
		t.Fatalf("Unpin = %v", err)
	}
	pinnedSet := map[string]bool{}
	for _, p := range pins[1:] {
		pinnedSet[p.StatusID] = true
	}
	var rest []string
	for _, id := range posts {
		if !pinnedSet[id] {
			rest = append(rest, id)
		}
	}
	if ok := pinAll(rest); ok != 1 {
		t.Errorf("%d concurrent pins with one free slot: %d succeeded, want 1", len(rest), ok)
	}
	if pins, err := e.h.Interactions.ListPins(ctx, author); err != nil || len(pins) != content.MaxPins {
		t.Errorf("ListPins = %d pins, %v; want %d", len(pins), err, content.MaxPins)
	}
}

// interactionChecksVisibility checks that every command that acts on a status goes through
// content.CanView: a status the account cannot see is reported as missing.
func interactionChecksVisibility(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, stranger, follower, pending, mentioned := e.localAccount(), e.localAccount(), e.localAccount(), e.localAccount(), e.localAccount()
	blocker, blocked := e.localAccount(), e.localAccount()
	e.follow(follower, author, graph.StateActive)
	e.follow(blocker, author, graph.StateActive)
	e.relations.block(blocker, author)
	e.relations.block(author, blocked)
	e.follow(pending, author, graph.StatePending)
	remote := e.account(false, account.StateActive)
	inactive := e.account(true, account.StateDeactivated)
	gone := e.account(true, account.StateActive)

	private := e.post(t, author, content.VisibilityPrivate, mentioned)
	direct := e.post(t, author, content.VisibilityDirect, mentioned)
	public := e.post(t, author, content.VisibilityPublic)
	byGone := e.post(t, gone, content.VisibilityPublic)
	e.actors.set(appaccount.Actor{
		Account: account.Account{ID: gone, Domain: account.LocalDomain}, Lifecycle: account.LifecycleState{AccountID: gone, State: account.StateDeleted},
	})
	deleted := e.post(t, author, content.VisibilityPublic)
	e.mustRemove(t, deleted)

	commands := map[string]func(accountID, statusID string) error{
		"Favourite": func(a, s string) error { return second(e.svc.Favourite(ctx, a, s)) },
		"Boost":     func(a, s string) error { return second(e.svc.Boost(ctx, a, s)) },
		"Bookmark":  func(a, s string) error { return e.svc.Bookmark(ctx, a, s) },
	}
	for _, tc := range []struct {
		name     string
		viewer   string
		status   content.Status
		want     error
		boostErr error // when the account sees the status but may not boost it
	}{
		{"a stranger on a private status", stranger, private, appcontent.ErrNotFound, nil},
		{"a pending follower on a private status", pending, private, appcontent.ErrNotFound, nil},
		{"a follower on a private status", follower, private, nil, content.ErrNotBoostable},
		{"a mentioned account on a private status", mentioned, private, nil, content.ErrNotBoostable},
		{"a follower on a direct status", follower, direct, appcontent.ErrNotFound, nil},
		{"a mentioned account on a direct status", mentioned, direct, nil, content.ErrNotBoostable},
		{"a stranger on a public status", stranger, public, nil, nil},
		{"a follower who blocks the author", blocker, public, appcontent.ErrNotFound, nil},
		{"an account the author blocks", blocked, public, appcontent.ErrNotFound, nil},
		{"anyone on a status of a deleted author", stranger, byGone, appcontent.ErrNotFound, nil},
		{"anyone on a deleted status", stranger, deleted, appcontent.ErrNotFound, nil},
		{"a remote account", remote, public, appcontent.ErrActorUnavailable, nil},
		{"an inactive account", inactive, public, appcontent.ErrActorUnavailable, nil},
		{"an unknown account", "unknown", public, appcontent.ErrActorUnavailable, nil},
	} {
		for name, cmd := range commands {
			want := tc.want
			if name == "Boost" && want == nil {
				want = tc.boostErr
			}
			if err := cmd(tc.viewer, tc.status.ID); !errors.Is(err, want) {
				t.Errorf("%s by %s = %v, want %v", name, tc.name, err, want)
			}
		}
	}
	if err := second(e.svc.Favourite(ctx, stranger, "missing")); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Favourite(missing) = %v, want ErrNotFound", err)
	}
	if err := e.svc.Pin(ctx, author, deleted.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Pin(deleted) = %v, want ErrNotFound", err)
	}
	// Only the visible interactions counted.
	if c := e.counters(t, private.ID); c.FavouritesCount != 2 || c.BoostsCount != 0 {
		t.Errorf("counters of the private status = %+v, want 2 favourites and no boosts", c)
	}
	if c := e.counters(t, direct.ID); c.FavouritesCount != 1 {
		t.Errorf("counters of the direct status = %+v, want 1 favourite", c)
	}
}

// interactionCountsConcurrentOnce runs the same command from many goroutines at once and checks
// that the counters move once.
func interactionCountsConcurrentOnce(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, actor := e.localAccount(), e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	const n = 8
	run := func(name string, fn func() error) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() { errs[i] = fn() })
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("concurrent %s = %v", name, err)
			}
		}
	}
	var (
		mu    sync.Mutex
		boost = map[string]bool{}
	)
	run("Boost", func() error {
		b, err := e.svc.Boost(ctx, actor, s.ID)
		mu.Lock()
		boost[b.ID] = true
		mu.Unlock()
		return err
	})
	run("Favourite", func() error { return second(e.svc.Favourite(ctx, actor, s.ID)) })
	if len(boost) != 1 {
		t.Errorf("concurrent boosts returned %d boosts, want 1", len(boost))
	}
	if c := e.counters(t, s.ID); c.BoostsCount != 1 || c.FavouritesCount != 1 {
		t.Errorf("counters after concurrent commands = %+v, want 1 boost and 1 favourite", c)
	}
	run("Unboost", func() error { return e.svc.Unboost(ctx, actor, s.ID) })
	run("Unfavourite", func() error { return e.svc.Unfavourite(ctx, actor, s.ID) })
	if c := e.counters(t, s.ID); c.BoostsCount != 0 || c.FavouritesCount != 0 {
		t.Errorf("counters after concurrent undos = %+v, want zero", c)
	}
}

func interactionForgetsDeletedStatuses(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	author, actor := e.localAccount(), e.localAccount()
	s := e.post(t, author, content.VisibilityPublic)
	if _, err := e.svc.Boost(ctx, actor, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Favourite(ctx, actor, s.ID); err != nil {
		t.Fatal(err)
	}
	e.mustRemove(t, s)
	if _, err := e.h.Interactions.FindBoostEdge(ctx, actor, s.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindBoostEdge of a deleted status = %v, want ErrNotFound", err)
	}
	e.takeEvents()
	if err := e.svc.Unboost(ctx, actor, s.ID); err != nil {
		t.Errorf("Unboost(deleted) = %v", err)
	}
	if err := e.svc.Unfavourite(ctx, actor, s.ID); err != nil {
		t.Errorf("Unfavourite(deleted) = %v", err)
	}
	if got := e.takeEvents(); !reflect.DeepEqual(got, []string{appcontent.EventUnfavourited}) {
		t.Errorf("events = %v, want only %s (the boost is no longer live)", got, appcontent.EventUnfavourited)
	}
	if _, err := e.h.Statuses.FindCounters(ctx, s.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("FindCounters of a deleted status = %v, want ErrNotFound", err)
	}
}

// interactionProjectionEqualsRebuild runs random boosts, favourites, their undos, and status
// deletions, and checks that rebuilding status_counters and boost_edges from their sources
// yields exactly the rows that the synchronous projectors wrote (data-model §9).
func interactionProjectionEqualsRebuild(t *testing.T, e *interactionEnv) {
	ctx := t.Context()
	rng := rand.New(rand.NewPCG(100, 20261016)) //nolint:gosec // deterministic test input, not a secret
	accounts := make([]string, 5)
	for i := range accounts {
		accounts[i] = e.localAccount()
	}
	visibilities := []content.Visibility{content.VisibilityPublic, content.VisibilityLocal, content.VisibilityUnlisted}
	var statuses []content.Status
	for range 12 {
		statuses = append(statuses, e.post(t, accounts[rng.IntN(len(accounts))], visibilities[rng.IntN(len(visibilities))]))
	}
	live := map[string]bool{}
	for _, s := range statuses {
		live[s.ID] = true
	}
	for round := range 3 {
		for range 80 {
			e.clock.Advance(time.Duration(rng.IntN(5000)+1) * time.Millisecond)
			a, s := accounts[rng.IntN(len(accounts))], statuses[rng.IntN(len(statuses))]
			var err error
			switch op := rng.IntN(20); {
			case op < 6:
				_, err = e.svc.Boost(ctx, a, s.ID)
			case op < 9:
				err = e.svc.Unboost(ctx, a, s.ID)
			case op < 15:
				_, err = e.svc.Favourite(ctx, a, s.ID)
			case op < 19:
				err = e.svc.Unfavourite(ctx, a, s.ID)
			default:
				if live[s.ID] {
					e.mustRemove(t, s)
					live[s.ID] = false
				}
			}
			// Commands on a deleted status fail with ErrNotFound; nothing else may fail.
			if err != nil && (!errors.Is(err, appcontent.ErrNotFound) || live[s.ID]) {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		incremental := snapshotInteractionProjections(t, e.h)
		if len(incremental.Edges) == 0 {
			t.Fatalf("round %d: boost_edges is too sparse to compare", round)
		}
		if err := appcontent.RebuildProjections(ctx, e.h.UnitOfWork, e.h.Statuses); err != nil {
			t.Fatalf("RebuildProjections = %v", err)
		}
		if err := appcontent.RebuildInteractionProjections(ctx, e.h.UnitOfWork, e.h.Interactions); err != nil {
			t.Fatalf("RebuildInteractionProjections = %v", err)
		}
		rebuilt := snapshotInteractionProjections(t, e.h)
		if !reflect.DeepEqual(incremental, rebuilt) {
			t.Fatalf("round %d: incremental projections differ from a rebuild:\nincremental %+v\nrebuilt     %+v", round, incremental, rebuilt)
		}
	}
}

type interactionProjections struct {
	Counters []content.Counters
	Edges    []content.BoostEdge
}

func snapshotInteractionProjections(t *testing.T, h InteractionHarness) interactionProjections {
	t.Helper()
	var (
		p   interactionProjections
		err error
	)
	if p.Counters, err = h.Statuses.ListCounters(t.Context()); err != nil {
		t.Fatalf("ListCounters = %v", err)
	}
	if p.Edges, err = h.Interactions.ListBoostEdges(t.Context()); err != nil {
		t.Fatalf("ListBoostEdges = %v", err)
	}
	return p
}
