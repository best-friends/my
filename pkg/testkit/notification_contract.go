// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/graph"
	"github.com/best-friends/my/internal/domain/notification"
)

// NotificationHarness is one database under test: an empty database with the product schema,
// its UnitOfWork, and the identity, account, graph, content, and notification repositories on it.
type NotificationHarness struct {
	AccountHarness
	Graph         appgraph.Repository
	Statuses      appcontent.Repository
	Interactions  appcontent.InteractionRepository
	Notifications appnotification.Repository
}

// NotificationFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type NotificationFactory func(t *testing.T, opts uow.Options) NotificationHarness

// NotificationContract verifies the notification repository of a database adapter together
// with the notification module's projectors and use cases, driven by the real follow, posting,
// and interaction use cases: follow, follow request, and follow acceptance notifications;
// mentions, also those an edit adds; boosts and favourites; never notifying an account of its
// own actions or a remote recipient; projecting an event again, also concurrently, without
// duplicates; hiding notifications across blocks and from accounts muted with
// hide_notifications; listing with a type filter and keyset paging; and the read marker,
// unread count, and dismissals.
func NotificationContract(t *testing.T, newHarness NotificationFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *notificationEnv)
	}{
		{"NotifiesFollows", notificationFollows},
		{"NotifiesMentions", notificationMentions},
		{"NotifiesBoostsAndFavouritesOnce", notificationInteractions},
		{"NotifiesLocalRecipientsOnly", notificationLocalOnly},
		{"ProjectsAnEventOnce", notificationProjectsOnce},
		{"AgreesOnConcurrentProjections", notificationConcurrent},
		{"HidesBlockedAndMutedSources", notificationHides},
		{"ListsWithTypesAndPages", notificationLists},
		{"TracksReadMarkersAndDismissals", notificationReads},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newNotificationEnv(t, newHarness)) })
	}
}

type notificationEnv struct {
	*accountEnv
	nh           NotificationHarness
	graph        *appgraph.Service
	content      *appcontent.Service
	interactions *appcontent.Interactions
	svc          *appnotification.Service
	// only holds just the notification projectors, for projecting an event again.
	only *uow.Projectors
}

func newNotificationEnv(t *testing.T, newHarness NotificationFactory) *notificationEnv {
	t.Helper()
	var (
		nh       NotificationHarness
		captured uow.Options
	)
	ae := newAccountEnv(t, func(t *testing.T, opts uow.Options) AccountHarness {
		captured = opts
		nh = newHarness(t, opts)
		return nh.AccountHarness
	})
	e := &notificationEnv{accountEnv: ae, nh: nh, only: &uow.Projectors{}}
	e.graph = appgraph.NewService(appgraph.Deps{
		UnitOfWork: nh.UnitOfWork, Repository: nh.Graph, Accounts: ae.accounts, Clock: ae.clock, IDs: ae.ids, Host: contractHost,
	})
	appgraph.RegisterProjectors(captured.Projectors, nh.Graph)
	appcontent.RegisterProjectors(captured.Projectors, nh.Statuses)
	appcontent.RegisterInteractionProjectors(captured.Projectors, nh.Interactions)
	deps := appnotification.ProjectorDeps{
		Repository: nh.Notifications, Accounts: ae.accounts, Relationships: e.graph, IDs: ae.ids,
	}
	appnotification.RegisterProjectors(captured.Projectors, deps)
	appnotification.RegisterProjectors(e.only, deps)
	e.content = appcontent.NewService(appcontent.Deps{
		UnitOfWork: nh.UnitOfWork, Repository: nh.Statuses, Accounts: ae.accounts, Relationships: e.graph,
		MaxStatusLength: func(context.Context) (int, error) { return 500, nil },
		Clock:           ae.clock, IDs: ae.ids, Host: contractHost,
	})
	e.interactions = appcontent.NewInteractions(appcontent.InteractionDeps{
		UnitOfWork: nh.UnitOfWork, Statuses: nh.Statuses, Interactions: nh.Interactions,
		Accounts: ae.accounts, Relationships: e.graph, Clock: ae.clock, IDs: ae.ids, Host: contractHost,
	})
	e.svc = appnotification.NewService(appnotification.Deps{
		UnitOfWork: nh.UnitOfWork, Repository: nh.Notifications, Clock: ae.clock,
	})
	return e
}

func (e *notificationEnv) user(t *testing.T, username string) string {
	t.Helper()
	return e.register(t, username).AccountID
}

func (e *notificationEnv) lock(t *testing.T, id string) {
	t.Helper()
	if _, err := e.accounts.ReviseProfile(t.Context(), id, appaccount.ProfileInput{Locked: true}); err != nil {
		t.Fatalf("ReviseProfile(%s, locked) = %v", id, err)
	}
}

func (e *notificationEnv) follow(t *testing.T, follower, followee string) {
	t.Helper()
	e.clock.Advance(time.Second)
	if _, err := e.graph.Follow(t.Context(), follower, followee); err != nil {
		t.Fatalf("Follow(%s, %s) = %v", follower, followee, err)
	}
}

func (e *notificationEnv) post(t *testing.T, author, text string) appcontent.Post {
	t.Helper()
	e.clock.Advance(time.Second)
	p, err := e.content.Create(t.Context(), appcontent.CreateInput{
		AuthorAccountID: author, TextInput: appcontent.TextInput{Text: text}, Visibility: content.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create(%q) = %v", text, err)
	}
	return p
}

func (e *notificationEnv) favourite(t *testing.T, accountID, statusID string) {
	t.Helper()
	e.clock.Advance(time.Second)
	if _, err := e.interactions.Favourite(t.Context(), accountID, statusID); err != nil {
		t.Fatalf("Favourite(%s, %s) = %v", accountID, statusID, err)
	}
}

// list returns all of accountID's notifications of the given types, newest first.
func (e *notificationEnv) list(t *testing.T, accountID string, types ...notification.Type) []notification.Notification {
	t.Helper()
	ns, err := e.svc.List(t.Context(), accountID, appnotification.Query{Types: types, Limit: appnotification.MaxPageLimit})
	if err != nil {
		t.Fatalf("List(%s, %v) = %v", accountID, types, err)
	}
	return ns
}

// summary is a notification without its generated ID and time, for comparing.
type summary struct {
	Type     notification.Type
	Source   string
	StatusID string
}

func (e *notificationEnv) summaries(t *testing.T, accountID string) []summary {
	t.Helper()
	out := []summary{}
	for _, n := range e.list(t, accountID) {
		out = append(out, summary{Type: n.Type, Source: n.SourceAccountID, StatusID: n.StatusID})
	}
	return out
}

func checkSummaries(t *testing.T, e *notificationEnv, accountID string, want ...summary) {
	t.Helper()
	if want == nil {
		want = []summary{}
	}
	if got := e.summaries(t, accountID); !reflect.DeepEqual(got, want) {
		t.Errorf("notifications of %s = %+v, want %+v", accountID, got, want)
	}
}

// project runs only the notification projectors on events, in one transaction.
func (e *notificationEnv) project(t *testing.T, events ...uow.Event) {
	t.Helper()
	err := e.nh.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		for _, ev := range events {
			if err := e.only.Project(t.Context(), tx, ev); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("project %v = %v", eventTypes(events), err)
	}
}

// favourited returns a favourite event of accountID on a local status of author.
func (e *notificationEnv) favourited(accountID, author, statusID string) appcontent.Favourited {
	return appcontent.Favourited{
		Favourite: content.Favourite{ID: e.ids.NewID(), AccountID: accountID, StatusID: statusID, URI: "https://" + contractHost + "/ap/activities/" + e.ids.NewID(), CreatedAt: e.clock.Now()},
		Status:    appcontent.StatusRef{ID: statusID, AuthorAccountID: author, IsLocal: true, Visibility: content.VisibilityPublic},
	}
}

func notificationFollows(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice, bob, carol := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol")
	e.lock(t, carol)

	e.follow(t, alice, bob)
	checkSummaries(t, e, bob, summary{Type: notification.TypeFollow, Source: alice})
	e.follow(t, alice, carol)
	checkSummaries(t, e, carol, summary{Type: notification.TypeFollowRequest, Source: alice})
	checkSummaries(t, e, alice)

	if _, err := e.graph.AcceptFollowRequest(ctx, carol, alice); err != nil {
		t.Fatalf("AcceptFollowRequest = %v", err)
	}
	checkSummaries(t, e, alice, summary{Type: notification.TypeFollowAccepted, Source: carol})
	checkSummaries(t, e, carol, summary{Type: notification.TypeFollowRequest, Source: alice})

	// Following again after unfollowing is a new follow and notifies again.
	if _, err := e.graph.Unfollow(ctx, alice, bob); err != nil {
		t.Fatalf("Unfollow = %v", err)
	}
	e.follow(t, alice, bob)
	checkSummaries(t, e, bob, summary{Type: notification.TypeFollow, Source: alice}, summary{Type: notification.TypeFollow, Source: alice})
	ns := e.list(t, bob)
	if ns[0].CreatedAt.Before(ns[1].CreatedAt) || ns[0].ID <= ns[1].ID || ns[0].RecipientAccountID != bob {
		t.Errorf("notifications of bob = %+v, want newest first", ns)
	}
}

func notificationMentions(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice, bob, carol, dave := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol"), e.user(t, "dave")
	p := e.post(t, alice, "hi @bob @carol and @alice")
	mention := func(source string) summary {
		return summary{Type: notification.TypeMention, Source: source, StatusID: p.Status.ID}
	}
	checkSummaries(t, e, bob, mention(alice))
	checkSummaries(t, e, carol, mention(alice))
	checkSummaries(t, e, alice) // never of one's own actions

	// An edit notifies only the accounts it newly mentions.
	e.clock.Advance(time.Second)
	_, err := e.content.Edit(ctx, appcontent.EditInput{
		AccountID: alice, StatusID: p.Status.ID, TextInput: appcontent.TextInput{Text: "hi @bob and @dave"},
	})
	if err != nil {
		t.Fatalf("Edit = %v", err)
	}
	checkSummaries(t, e, bob, mention(alice))
	checkSummaries(t, e, dave, mention(alice))
	checkSummaries(t, e, carol, mention(alice))
}

func notificationInteractions(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice, bob := e.user(t, "alice"), e.user(t, "bob")
	p := e.post(t, alice, "hello")
	id := p.Status.ID

	e.favourite(t, bob, id)
	e.favourite(t, alice, id) // one's own
	if err := e.interactions.Unfavourite(ctx, bob, id); err != nil {
		t.Fatalf("Unfavourite = %v", err)
	}
	e.favourite(t, bob, id) // undoing and repeating does not notify again
	for range 2 {
		e.clock.Advance(time.Second)
		if _, err := e.interactions.Boost(ctx, bob, id); err != nil {
			t.Fatalf("Boost = %v", err)
		}
		if err := e.interactions.Unboost(ctx, bob, id); err != nil {
			t.Fatalf("Unboost = %v", err)
		}
	}
	if _, err := e.interactions.Boost(ctx, alice, id); err != nil {
		t.Fatalf("Boost(own) = %v", err)
	}
	checkSummaries(t, e, alice,
		summary{Type: notification.TypeBoost, Source: bob, StatusID: id},
		summary{Type: notification.TypeFavourite, Source: bob, StatusID: id},
	)
	checkSummaries(t, e, bob)
}

func notificationLocalOnly(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice := e.user(t, "alice")
	remote := account.Account{
		ID: e.ids.NewID(), URI: "https://remote.example/users/rem", Domain: "remote.example",
		ActorType: account.ActorPerson, CreatedAt: e.clock.Now(),
	}
	err := e.nh.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return e.nh.Accounts.InsertAccount(ctx, tx, remote) })
	if err != nil {
		t.Fatalf("InsertAccount(remote) = %v", err)
	}
	missing := e.ids.NewID()
	follow := graph.Follow{ID: e.ids.NewID(), FollowerAccountID: alice, FolloweeAccountID: remote.ID, RequestedAt: e.clock.Now()}
	remoteStatus := e.favourited(alice, remote.ID, e.ids.NewID())
	remoteStatus.Status.IsLocal = false
	e.project(t,
		appgraph.Followed{Follow: follow, Acceptance: graph.Acceptance{FollowID: follow.ID, AcceptedAt: e.clock.Now()}},
		appcontent.StatusCreated{
			Status:   content.Status{ID: e.ids.NewID(), AuthorAccountID: alice, IsLocal: true, Visibility: content.VisibilityPublic},
			Revision: content.Revision{MentionedAccountIDs: []string{remote.ID, missing}, RevisedAt: e.clock.Now()},
		},
		remoteStatus,
	)
	for _, id := range []string{remote.ID, missing} {
		checkSummaries(t, e, id)
	}
	checkSummaries(t, e, alice)
}

func notificationProjectsOnce(t *testing.T, e *notificationEnv) {
	alice, bob := e.user(t, "alice"), e.user(t, "bob")
	p := e.post(t, alice, "hi @bob")
	follow := graph.Follow{ID: e.ids.NewID(), FollowerAccountID: alice, FolloweeAccountID: bob, RequestedAt: e.clock.Now()}
	events := []uow.Event{
		appcontent.StatusCreated(p),
		appgraph.FollowRequested{Follow: follow},
		e.favourited(bob, alice, p.Status.ID),
	}
	e.project(t, events...)
	e.project(t, events...)
	e.project(t, e.favourited(bob, alice, p.Status.ID)) // another favourite of the same pair
	checkSummaries(t, e, bob,
		summary{Type: notification.TypeFollowRequest, Source: alice},
		summary{Type: notification.TypeMention, Source: alice, StatusID: p.Status.ID},
	)
	checkSummaries(t, e, alice, summary{Type: notification.TypeFavourite, Source: bob, StatusID: p.Status.ID})
}

func notificationConcurrent(t *testing.T, e *notificationEnv) {
	alice, bob := e.user(t, "alice"), e.user(t, "bob")
	p := e.post(t, alice, "hello")
	ev := e.favourited(bob, alice, p.Status.ID)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			errs[i] = e.nh.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error { return e.only.Project(t.Context(), tx, ev) })
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("project %d = %v", i, err)
		}
	}
	checkSummaries(t, e, alice, summary{Type: notification.TypeFavourite, Source: bob, StatusID: p.Status.ID})
}

func notificationHides(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice, bob, carol, dave, erin := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol"), e.user(t, "dave"), e.user(t, "erin")

	// A mute with hide_notifications hides them; one without does not.
	if _, err := e.graph.Mute(ctx, bob, alice, appgraph.MuteInput{HideNotifications: true}); err != nil {
		t.Fatalf("Mute = %v", err)
	}
	if _, err := e.graph.Mute(ctx, carol, alice, appgraph.MuteInput{}); err != nil {
		t.Fatalf("Mute = %v", err)
	}
	e.follow(t, alice, bob)
	e.follow(t, alice, carol)
	p := e.post(t, alice, "hi @bob @carol")
	checkSummaries(t, e, bob)
	checkSummaries(t, e, carol,
		summary{Type: notification.TypeMention, Source: alice, StatusID: p.Status.ID},
		summary{Type: notification.TypeFollow, Source: alice},
	)

	// An expired mute hides nothing.
	if _, err := e.graph.Mute(ctx, dave, alice, appgraph.MuteInput{HideNotifications: true, Duration: time.Hour}); err != nil {
		t.Fatalf("Mute = %v", err)
	}
	e.follow(t, alice, dave)
	checkSummaries(t, e, dave)
	e.clock.Advance(2 * time.Hour)
	q := e.post(t, alice, "hi @dave")
	checkSummaries(t, e, dave, summary{Type: notification.TypeMention, Source: alice, StatusID: q.Status.ID})

	// A block in either direction hides them.
	mine := e.post(t, erin, "mine")
	if _, err := e.graph.Block(ctx, erin, alice); err != nil {
		t.Fatalf("Block = %v", err)
	}
	if _, err := e.graph.Block(ctx, dave, erin); err != nil {
		t.Fatalf("Block = %v", err)
	}
	e.project(t, e.favourited(alice, erin, mine.Status.ID), e.favourited(dave, erin, mine.Status.ID))
	checkSummaries(t, e, erin)
}

func notificationLists(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice := e.user(t, "alice")
	var followers []string
	for _, name := range []string{"u1", "u2", "u3"} {
		id := e.user(t, name)
		followers = append(followers, id)
		e.follow(t, id, alice)
	}
	p := e.post(t, alice, "hello")
	e.favourite(t, followers[0], p.Status.ID)
	e.favourite(t, followers[1], p.Status.ID)

	all := e.list(t, alice)
	if len(all) != 5 {
		t.Fatalf("notifications = %d, want 5", len(all))
	}
	follows := e.list(t, alice, notification.TypeFollow)
	if len(follows) != 3 || follows[0].SourceAccountID != followers[2] || follows[2].SourceAccountID != followers[0] {
		t.Errorf("follow notifications = %+v, want the three follows newest first", follows)
	}
	if got := e.list(t, alice, notification.TypeFollow, notification.TypeFavourite); len(got) != 5 {
		t.Errorf("follow and favourite notifications = %d, want 5", len(got))
	}
	if got := e.list(t, alice, notification.TypeMention); len(got) != 0 {
		t.Errorf("mention notifications = %+v, want none", got)
	}
	if _, err := e.svc.List(ctx, alice, appnotification.Query{Types: []notification.Type{"poke"}}); !errors.Is(err, notification.ErrInvalidType) {
		t.Errorf("List(poke) = %v, want ErrInvalidType", err)
	}

	var paged []notification.Notification
	before := ""
	for {
		page, err := e.svc.List(ctx, alice, appnotification.Query{Before: before, Limit: 2})
		if err != nil {
			t.Fatalf("List = %v", err)
		}
		paged = append(paged, page...)
		if len(page) < 2 {
			break
		}
		before = page[len(page)-1].ID
	}
	if !reflect.DeepEqual(paged, all) {
		t.Errorf("pages = %+v, want %+v", paged, all)
	}
	page, err := e.svc.List(ctx, alice, appnotification.Query{Types: []notification.Type{notification.TypeFollow}, Before: follows[0].ID, Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != follows[1].ID {
		t.Errorf("List(follow, before the newest, 1) = %+v, %v; want the second follow", page, err)
	}
}

func notificationReads(t *testing.T, e *notificationEnv) {
	ctx := t.Context()
	alice, bob, carol, dave := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol"), e.user(t, "dave")
	e.follow(t, bob, alice)
	e.follow(t, carol, alice)
	e.follow(t, dave, alice)
	e.follow(t, alice, bob)
	ns := e.list(t, alice) // dave, carol, bob
	other := e.list(t, bob)[0]

	checkUnread := func(want int64) {
		t.Helper()
		if n, err := e.svc.UnreadCount(ctx, alice); err != nil || n != want {
			t.Errorf("UnreadCount = %d, %v; want %d", n, err, want)
		}
	}
	if m, err := e.svc.ReadMarker(ctx, alice); err != nil || m != (notification.ReadMarker{AccountID: alice}) {
		t.Errorf("ReadMarker = %+v, %v; want none", m, err)
	}
	checkUnread(3)

	m, err := e.svc.MarkRead(ctx, alice, ns[1].ID)
	if err != nil || m != (notification.ReadMarker{AccountID: alice, LastReadID: ns[1].ID}) {
		t.Errorf("MarkRead = %+v, %v", m, err)
	}
	checkUnread(1)
	if m, err := e.svc.MarkRead(ctx, alice, ns[2].ID); err != nil || m.LastReadID != ns[1].ID {
		t.Errorf("MarkRead(older) = %+v, %v; want the marker to stay", m, err)
	}
	if _, err := e.svc.MarkRead(ctx, alice, other.ID); !errors.Is(err, appnotification.ErrNotFound) {
		t.Errorf("MarkRead(bob's) = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.MarkRead(ctx, alice, e.ids.NewID()); !errors.Is(err, appnotification.ErrNotFound) {
		t.Errorf("MarkRead(missing) = %v, want ErrNotFound", err)
	}
	checkUnread(1)

	// Dismissing leaves the list and the unread count, once.
	for range 2 {
		if err := e.svc.Dismiss(ctx, alice, ns[0].ID); err != nil {
			t.Fatalf("Dismiss = %v", err)
		}
	}
	checkUnread(0)
	if err := e.svc.Dismiss(ctx, alice, other.ID); !errors.Is(err, appnotification.ErrNotFound) {
		t.Errorf("Dismiss(bob's) = %v, want ErrNotFound", err)
	}
	if got := e.list(t, alice); !reflect.DeepEqual(got, ns[1:]) {
		t.Errorf("notifications = %+v, want %+v", got, ns[1:])
	}
	if n, err := e.svc.UnreadCount(ctx, bob); err != nil || n != 1 {
		t.Errorf("UnreadCount(bob) = %d, %v; want 1", n, err)
	}
}
