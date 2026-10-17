// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	appfederation "github.com/best-friends/my/internal/app/federation"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/app/queue"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/federation"
	"github.com/best-friends/my/internal/domain/notification"
	"github.com/best-friends/my/internal/domain/timeline"
)

// RemoteInteractionHarness is one database under test: the harness of the remote posts, with the
// interaction and federation repositories on the same database.
type RemoteInteractionHarness struct {
	RemoteStatusHarness
	Interactions appcontent.InteractionRepository
	Federation   appfederation.Repository
}

// RemoteInteractionFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type RemoteInteractionFactory func(t *testing.T, opts uow.Options) RemoteInteractionHarness

// RemoteInteractionContract verifies the boosts and favourites across servers (federation.md
// §3.4, §7.1) on a database adapter, through the inbox processor with the real Announce, Like,
// EmojiReact, and Undo handlers, the real content, account, graph, timeline, notification, and
// federation modules, and the outbound projectors and fan-out: the Announce, Like, Misskey
// reaction, and Akkoma EmojiReact documents of the major implementations (the federation
// fixtures) boost and favourite local posts with counters and notifications, and their Undo takes
// them back; a remote post that a followed account boosts is stored, from the Announce when it
// embeds the post from the booster's origin and fetched again from its origin otherwise; a boost
// takes the narrower of its addressing and the post's visibility; local-only posts and posts the
// booster may not see or boost are not boosted, and local boosts of local-only posts do not
// federate (ADR-0017); a local favourite of a remote post sends a Like, and its removal an Undo
// Like, to the post's author, both read back as the activity they are; and the final rows do not
// depend on the order or repetition of the activities (NFR-REL-1). The actors and the fetches are
// in-memory stand-ins.
func RemoteInteractionContract(t *testing.T, newHarness RemoteInteractionFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *remoteInteractionEnv)
	}{
		{"BoostsAndFavouritesLocalPostsFromFixtures", remoteInteractsWithLocalPosts},
		{"BoostsRemotePostsOfFollowedBoosters", remoteBoostsRemotePosts},
		{"RefusesBoostsTheBoosterMayNotMake", remoteRefusesBoosts},
		{"SendsLikesAndUndoLikes", remoteSendsLikes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newRemoteInteractionEnv(t, newHarness)) })
	}
	t.Run("OrderAndDuplicates", func(t *testing.T) { remoteInteractionOrder(t, newHarness) })
}

type remoteInteractionEnv struct {
	*remoteStatusEnv
	ih           RemoteInteractionHarness
	interactions *appcontent.Interactions
	jobs         *fedJobs
	handlers     *queue.JobHandlerRegistry
}

func newRemoteInteractionEnv(t *testing.T, newHarness RemoteInteractionFactory) *remoteInteractionEnv {
	t.Helper()
	var ih RemoteInteractionHarness
	rs := newRemoteStatusEnv(t, func(t *testing.T, opts uow.Options) RemoteStatusHarness {
		ih = newHarness(t, opts)
		return ih.RemoteStatusHarness
	})
	e := &remoteInteractionEnv{remoteStatusEnv: rs, ih: ih, jobs: &fedJobs{seen: map[string]bool{}}, handlers: &queue.JobHandlerRegistry{}}
	appcontent.RegisterInteractionProjectors(rs.projectors, ih.Interactions)
	e.interactions = appcontent.NewInteractions(appcontent.InteractionDeps{
		UnitOfWork: ih.UnitOfWork, Statuses: ih.Statuses, Interactions: ih.Interactions, Accounts: rs.accounts,
		Relationships: rs.graph, Clock: rs.clock, IDs: rs.ids, Host: contractHost,
	})
	appfederation.RegisterInteractionHandlers(rs.processor, appfederation.InteractionHandlersDeps{
		Interactions: e.interactions, Statuses: rs.statuses,
	})
	outbox := queue.AfterCommit(rs.projectors, e.jobs, nil)
	appfederation.RegisterOutboundProjectors(rs.projectors, appfederation.RecorderDeps{
		Repository: ih.Federation, Outbox: outbox, IDs: rs.ids, Host: contractHost,
	})
	appfederation.NewFanOut(appfederation.FanOutDeps{
		UnitOfWork: ih.UnitOfWork, Repository: ih.Federation, Outbox: outbox,
		Statuses: appfederation.ContentStatuses{Service: rs.content, Interactions: e.interactions},
		Accounts: rs.accounts, Graph: rs.graph, Clock: rs.clock, IDs: rs.ids, Host: contractHost,
	}).RegisterJobs(e.handlers)
	return e
}

// localPost makes the local account author post text with visibility v.
func (e *remoteInteractionEnv) localPost(t *testing.T, author string, v content.Visibility) content.Status {
	t.Helper()
	p, err := e.content.Create(t.Context(), appcontent.CreateInput{
		AuthorAccountID: author, TextInput: appcontent.TextInput{Text: "post"}, Visibility: v,
	})
	if err != nil {
		t.Fatalf("Create(%s): %v", v, err)
	}
	return p.Status
}

// counters returns the boosts and favourites counts of a status.
func (e *remoteInteractionEnv) counters(t *testing.T, statusID string) (boosts, favourites int64) {
	t.Helper()
	c, err := e.h.Statuses.FindCounters(t.Context(), statusID)
	if err != nil {
		t.Fatalf("FindCounters(%s): %v", statusID, err)
	}
	return c.BoostsCount, c.FavouritesCount
}

// liveBoost returns the live boost of a status by an account, false when there is none.
func (e *remoteInteractionEnv) liveBoost(t *testing.T, booster, statusID string) (content.Boost, bool) {
	t.Helper()
	edge, err := e.ih.Interactions.FindBoostEdge(t.Context(), booster, statusID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return content.Boost{}, false
	}
	if err != nil {
		t.Fatalf("FindBoostEdge: %v", err)
	}
	b, err := e.ih.Interactions.FindBoost(t.Context(), edge.BoostID)
	if err != nil {
		t.Fatalf("FindBoost: %v", err)
	}
	return b, true
}

// favourited reports whether an account favourites a status.
func (e *remoteInteractionEnv) favourited(t *testing.T, accountID, statusID string) bool {
	t.Helper()
	_, err := e.ih.Interactions.FindFavourite(t.Context(), accountID, statusID)
	if err != nil && !errors.Is(err, appcontent.ErrNotFound) {
		t.Fatalf("FindFavourite: %v", err)
	}
	return err == nil
}

// notified returns the notifications of type typ that a local account got from source.
func (e *remoteInteractionEnv) notified(t *testing.T, recipient, source string, typ notification.Type) int {
	t.Helper()
	ns, err := e.notifications.List(t.Context(), recipient, appnotification.Query{Limit: appnotification.MaxPageLimit})
	if err != nil {
		t.Fatalf("List notifications: %v", err)
	}
	n := 0
	for _, x := range ns {
		if x.Type == typ && x.SourceAccountID == source {
			n++
		}
	}
	return n
}

// fixtureDocument returns a federation fixture as it is, with http IRIs made https (the inbox
// takes only https actors; the older fixtures were taken from servers on http) and each pair of
// replace applied.
func fixtureDocument(t *testing.T, implementation, version, name string, replace ...string) []byte {
	t.Helper()
	fixtures, err := FederationFixtures()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(fixtures, func(f FederationFixture) bool {
		return f.Implementation == implementation && f.Version == version && f.Name == name
	})
	if i < 0 {
		t.Fatalf("no fixture %s/%s/%s", implementation, version, name)
	}
	s := strings.ReplaceAll(string(fixtures[i].JSON), `"http://`, `"https://`)
	return []byte(strings.NewReplacer(replace...).Replace(s))
}

// remoteAddressed returns an activity of typ as JSON, addressed to the public and the actor's
// followers as Mastodon addresses a public boost.
func remoteAddressed(t *testing.T, typ, id, actor string, object any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams", "id": id, "type": typ, "actor": actor, "object": object,
		"to": []string{string(federation.PublicIRI)}, "cc": []string{actor + "/followers"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// remoteAnnounce returns a public Announce as JSON.
func remoteAnnounce(t *testing.T, id, actor string, object any) []byte {
	t.Helper()
	return remoteAddressed(t, "Announce", id, actor, object)
}

func remoteInteractsWithLocalPosts(t *testing.T, e *remoteInteractionEnv) {
	author := e.user(t, "author")
	post := e.localPost(t, author, content.VisibilityPublic)
	at := func(uri string) []string { return []string{uri, post.URI} }

	for _, tc := range []struct {
		implementation, version, name string
		actor, object                 string
		visibility                    content.Visibility
	}{
		{"mastodon", "circa-2018", "announce", "https://mastodon.example.org/users/admin", "https://mastodon.example.org/@admin/99541947525187367", content.VisibilityPublic},
		{"gotosocial", "0.22", "announce", "https://gotosocial.example/users/the_mighty_zork", "https://gotosocial.example/users/the_mighty_zork/statuses/01FCTA44PW9H1TB328S9AQXKDS", content.VisibilityPrivate},
	} {
		t.Run("Announce/"+tc.implementation, func(t *testing.T) {
			booster := e.actor(t, tc.actor)
			doc := fixtureDocument(t, tc.implementation, tc.version, tc.name, at(tc.object)...)
			e.expect(t, doc, appfederation.InboxHandled)
			b, ok := e.liveBoost(t, booster, post.ID)
			if !ok || b.IsLocal || b.Visibility != tc.visibility || b.Federates() {
				t.Fatalf("boost = %+v (%t), want a remote %s boost that does not federate", b, ok, tc.visibility)
			}
			if boosts, _ := e.counters(t, post.ID); boosts != 1 {
				t.Errorf("boosts = %d, want 1", boosts)
			}
			if n := e.notified(t, author, booster, notification.TypeBoost); n != 1 {
				t.Errorf("boost notifications = %d, want 1", n)
			}
			// Another Announce of the same post while the boost is live changes nothing.
			again := remoteAnnounce(t, tc.actor+"/announces/again", tc.actor, post.URI)
			e.expect(t, again, appfederation.InboxHandled)
			if got, _ := e.liveBoost(t, booster, post.ID); got.ID != b.ID {
				t.Errorf("boost after a second Announce = %+v, want %+v", got, b)
			}
			// The Undo by the Announce's IRI takes it back, once.
			e.expect(t, remoteActivity(t, "Undo", b.URI+"#undo", tc.actor, b.URI), appfederation.InboxHandled)
			e.expect(t, remoteActivity(t, "Undo", b.URI+"#undo-again", tc.actor, b.URI), appfederation.InboxHandled)
			if _, ok := e.liveBoost(t, booster, post.ID); ok {
				t.Error("boost live after its Undo")
			}
			if r, err := e.ih.Interactions.FindBoostRetraction(t.Context(), b.ID); err != nil || r.Origin != content.RetractedByRemote {
				t.Errorf("retraction = %+v, %v; want one by the remote booster", r, err)
			}
			if boosts, _ := e.counters(t, post.ID); boosts != 0 {
				t.Errorf("boosts after the Undo = %d, want 0", boosts)
			}
		})
	}

	const mastodonPost = "https://remote.example/objects/eb92579d-3417-42a8-8652-2492c2d4f454"
	for _, tc := range []struct {
		name, actor string
		like, undo  []byte
	}{
		{
			"mastodon", "https://mastodon.example.org/users/admin",
			fixtureDocument(t, "mastodon", "circa-2018", "like", at(mastodonPost)...),
			fixtureDocument(t, "mastodon", "circa-2018", "undo-like", at(mastodonPost)...),
		},
		{
			"misskey reaction", "https://misskey.example/users/8x8yep20u2",
			fixtureDocument(t, "misskey", "circa-2022", "like-reaction", at("https://remote.example/objects/89937a53-2692-4631-bb62-770091267391")...),
			remoteActivity(t, "Undo", "https://misskey.example/likes/917ocsybgp/undo", "https://misskey.example/users/8x8yep20u2", map[string]any{
				"type": "Like", "id": "https://misskey.example/likes/917ocsybgp", "actor": "https://misskey.example/users/8x8yep20u2", "object": post.URI,
			}),
		},
		{
			"akkoma EmojiReact", "https://example.org/users/akko",
			fixtureDocument(t, "akkoma", "3.20", "emoji-react", at("https://remote.example/objects/9f0e93499d8314a9")...),
			// Akkoma takes a reaction back by its IRI alone.
			fixtureDocument(t, "akkoma", "3.20", "undo-emoji-react"),
		},
	} {
		t.Run("Like/"+tc.name, func(t *testing.T) {
			fan := e.actor(t, tc.actor)
			e.expect(t, tc.like, appfederation.InboxHandled)
			if !e.favourited(t, fan, post.ID) {
				t.Fatal("no favourite after the Like")
			}
			if _, favourites := e.counters(t, post.ID); favourites != 1 {
				t.Errorf("favourites = %d, want 1", favourites)
			}
			if n := e.notified(t, author, fan, notification.TypeFavourite); n != 1 {
				t.Errorf("favourite notifications = %d, want 1", n)
			}
			e.expect(t, tc.undo, appfederation.InboxHandled)
			if e.favourited(t, fan, post.ID) {
				t.Error("favourite after the Undo")
			}
			if _, favourites := e.counters(t, post.ID); favourites != 0 {
				t.Errorf("favourites after the Undo = %d, want 0", favourites)
			}
		})
	}

	// A Like of a post this server does not have, and the Undo of a Like it never got (which
	// leaves a tombstone that drops the Like arriving after it).
	const bob = "https://remote.example/users/bob"
	e.actor(t, bob)
	e.expect(t, remoteActivity(t, "Like", bob+"#likes/1", bob, "https://"+contractHost+"/ap/statuses/missing"), appfederation.InboxDropped)
	e.expect(t, remoteActivity(t, "Undo", bob+"#likes/2/undo", bob, bob+"#likes/2"), appfederation.InboxTombstoned)
	e.expect(t, remoteActivity(t, "Like", bob+"#likes/2", bob, post.URI), appfederation.InboxDropped)
	if _, favourites := e.counters(t, post.ID); favourites != 0 {
		t.Errorf("favourites after a Like that was taken back first = %d, want 0", favourites)
	}
}

func remoteBoostsRemotePosts(t *testing.T, e *remoteInteractionEnv) {
	const (
		alice = "https://remote.example/users/alice"
		carol = "https://other.example/users/carol"
	)
	reader := e.user(t, "reader")
	aliceID, carolID := e.actor(t, alice), e.actor(t, carol)
	published := remoteNow.Add(-time.Hour)

	// alice is not followed: her boost of a post this server does not have stores nothing, and
	// fetches nothing.
	unknown := carol + "/notes/1"
	e.fetcher.serve(t, unknown, remoteNote(unknown, carol, published, nil))
	e.expect(t, remoteAnnounce(t, alice+"/announces/1", alice, unknown), appfederation.InboxDropped)
	if e.stored(t, unknown) || slices.Contains(e.fetcher.fetched, unknown) {
		t.Fatalf("%s stored or fetched for a booster no local account follows", unknown)
	}

	e.follow(t, reader, aliceID)
	// A post by its IRI is fetched from its origin.
	announce := remoteAnnounce(t, alice+"/announces/2", alice, unknown)
	var v map[string]any
	if err := json.Unmarshal(announce, &v); err != nil {
		t.Fatal(err)
	}
	v["published"] = remoteNow.Add(-time.Minute).Format(time.RFC3339)
	v["to"], v["cc"] = []string{string(federation.PublicIRI)}, []string{alice + "/followers", carol}
	announce, _ = json.Marshal(v)
	e.expect(t, announce, appfederation.InboxHandled)
	st := e.post(t, unknown).Status
	if st.AuthorAccountID != carolID || st.IsLocal {
		t.Fatalf("boosted post = %+v, want carol's", st)
	}
	b, ok := e.liveBoost(t, aliceID, st.ID)
	if !ok || b.Visibility != content.VisibilityPublic || !b.BoostedAt.Equal(remoteNow.Add(-time.Minute)) {
		t.Fatalf("boost = %+v (%t), want alice's public boost at the Announce's time", b, ok)
	}
	home, err := e.timeline.Home(t.Context(), appcontent.Viewer{Kind: appcontent.ViewerLocal, AccountID: reader}, apptimeline.Query{Limit: apptimeline.MaxLimit})
	if err != nil || !slices.ContainsFunc(home.Entries, func(en apptimeline.Entry) bool { return en.Kind == timeline.KindBoost && en.ID == b.ID }) {
		t.Errorf("home of alice's follower = %+v, %v; want the boost", home.Entries, err)
	}

	// A post embedded from another origin is fetched again: the embedded copy is not trusted.
	forged := carol + "/notes/2"
	e.fetcher.serve(t, forged, remoteNote(forged, carol, published, map[string]any{"content": "<p>real</p>"}))
	e.expect(t, remoteAnnounce(t, alice+"/announces/3", alice,
		remoteNote(forged, carol, published, map[string]any{"content": "<p>forged</p>"})), appfederation.InboxHandled)
	if rev := e.post(t, forged).Revision; !strings.Contains(rev.Bodies[0].ContentHTML, "real") {
		t.Errorf("boosted post embedded from another origin = %q, want the one its origin serves", rev.Bodies[0].ContentHTML)
	}

	// A post embedded from the booster's origin is taken as it is.
	own := alice + "/notes/1"
	e.expect(t, remoteAnnounce(t, alice+"/announces/4", alice,
		remoteNote(own, alice, published, map[string]any{"content": "<p>embedded</p>"})), appfederation.InboxHandled)
	if slices.Contains(e.fetcher.fetched, own) {
		t.Errorf("%s fetched although the Announce embeds it from the booster's origin", own)
	}
	if rev := e.post(t, own).Revision; !strings.Contains(rev.Bodies[0].ContentHTML, "embedded") {
		t.Errorf("boosted own post = %q, want the embedded one", rev.Bodies[0].ContentHTML)
	}

	// An Announce to the followers only makes a followers-only boost of a public post; one of
	// someone else's followers-only post is refused.
	toFollowers := carol + "/notes/3"
	e.fetcher.serve(t, toFollowers, remoteNote(toFollowers, carol, published, nil))
	doc := remoteAnnounce(t, alice+"/announces/5", alice, toFollowers)
	if err := json.Unmarshal(doc, &v); err != nil {
		t.Fatal(err)
	}
	v["to"] = []string{alice + "/followers"}
	doc, _ = json.Marshal(v)
	e.expect(t, doc, appfederation.InboxHandled)
	if b, ok := e.liveBoost(t, aliceID, e.post(t, toFollowers).Status.ID); !ok || b.Visibility != content.VisibilityPrivate {
		t.Errorf("boost addressed to the followers = %+v (%t), want private", b, ok)
	}
	private := carol + "/notes/4"
	e.fetcher.serve(t, private, remoteNote(private, carol, published, map[string]any{"to": []string{carol + "/followers"}, "cc": []string{}}))
	e.expect(t, remoteAnnounce(t, alice+"/announces/6", alice, private), appfederation.InboxDropped)
	if st := e.post(t, private).Status; st.Visibility != content.VisibilityPrivate {
		t.Errorf("visibility = %s, want private", st.Visibility)
	} else if _, ok := e.liveBoost(t, aliceID, st.ID); ok {
		t.Error("boost of someone else's followers-only post")
	}
}

func remoteRefusesBoosts(t *testing.T, e *remoteInteractionEnv) {
	const alice = "https://remote.example/users/alice"
	author := e.user(t, "author")
	aliceID := e.actor(t, alice)
	localOnly := e.localPost(t, author, content.VisibilityLocal)
	private := e.localPost(t, author, content.VisibilityPrivate)

	// A local-only post stays on this server: remote accounts do not boost it (ADR-0017), even
	// those that follow its author.
	e.expect(t, remoteAnnounce(t, alice+"/announces/1", alice, localOnly.URI), appfederation.InboxDropped)
	e.expect(t, remoteAnnounce(t, alice+"/announces/2", alice, private.URI), appfederation.InboxDropped)
	// Nor does a remote account favourite a followers-only post it cannot see.
	e.expect(t, remoteActivity(t, "Like", alice+"#likes/1", alice, private.URI), appfederation.InboxDropped)
	for _, st := range []content.Status{localOnly, private} {
		if _, ok := e.liveBoost(t, aliceID, st.ID); ok {
			t.Errorf("boost of a %s post by a remote account", st.Visibility)
		}
	}
	if e.favourited(t, aliceID, private.ID) {
		t.Error("favourite of a followers-only post by a remote account that does not follow its author")
	}

	// A local account's boost of a local-only post records no activity (ADR-0017); its boost of
	// a public post records the Announce.
	booster := e.user(t, "booster")
	b, err := e.interactions.Boost(t.Context(), booster, localOnly.ID)
	if err != nil {
		t.Fatalf("Boost(local) = %v", err)
	}
	if got, err := e.ih.Federation.ListOutboundActivitiesOf(t.Context(), federation.OutboundBoost, b.ID); err != nil || len(got) != 0 {
		t.Errorf("activities of the boost of a local-only post = %+v, %v; want none", got, err)
	}
	public := e.localPost(t, author, content.VisibilityPublic)
	b, err = e.interactions.Boost(t.Context(), booster, public.ID)
	if err != nil {
		t.Fatalf("Boost(public) = %v", err)
	}
	if got, err := e.ih.Federation.ListOutboundActivitiesOf(t.Context(), federation.OutboundBoost, b.ID); err != nil || len(got) != 1 || got[0].Type != federation.ActivityAnnounce {
		t.Errorf("activities of the boost of a public post = %+v, %v; want the Announce", got, err)
	}
}

func remoteSendsLikes(t *testing.T, e *remoteInteractionEnv) {
	const alice = "https://remote.example/users/alice"
	fan := e.user(t, "fan")
	aliceID := e.actor(t, alice)
	e.follow(t, fan, aliceID)
	id := alice + "/notes/1"
	e.expect(t, remoteActivity(t, "Create", id+"/activity", alice, remoteNote(id, alice, remoteNow.Add(-time.Hour), nil)), appfederation.InboxHandled)
	st := e.post(t, id).Status

	f, err := e.interactions.Favourite(t.Context(), fan, st.ID)
	if err != nil {
		t.Fatalf("Favourite = %v", err)
	}
	e.clock.Advance(time.Minute)
	if err := e.interactions.Unfavourite(t.Context(), fan, st.ID); err != nil {
		t.Fatalf("Unfavourite = %v", err)
	}
	sent := e.sent(t, st.ID)
	if len(sent) != 2 || sent[0].Type != federation.ActivityLike || sent[1].Type != federation.ActivityUndo {
		t.Fatalf("activities = %+v, want a Like and its Undo", sent)
	}
	fanIRI := federation.IRI(e.localIRI(fan))
	for _, s := range sent {
		if !slices.Equal(s.Inboxes, []string{alice + "/inbox"}) {
			t.Errorf("%s inboxes = %v, want the author's", s.Type, s.Inboxes)
		}
		// The documents read back as what they are.
		doc, err := appfederation.Normalize(s.Raw, e.clock.Now())
		if err != nil || doc.Activity == nil || doc.Activity.Actor != fanIRI {
			t.Fatalf("Normalize(%s) = %+v, %v; want an activity of the fan", s.Raw, doc, err)
		}
		like := doc.Activity
		if s.Type == federation.ActivityUndo {
			embedded := like.Object().Untrusted()
			if embedded == nil || embedded.Activity == nil {
				t.Fatalf("Undo %s does not embed the Like", s.Raw)
			}
			like = embedded.Activity
		}
		if like.Type != federation.ActivityLike || like.ID != federation.IRI(f.URI) || like.Object().ID != federation.IRI(st.URI) {
			t.Errorf("Like = %+v, want %s of %s", like, f.URI, st.URI)
		}
	}
	// A favourite of a local post sends nothing.
	local := e.localPost(t, e.user(t, "author"), content.VisibilityPublic)
	if _, err := e.interactions.Favourite(t.Context(), fan, local.ID); err != nil {
		t.Fatalf("Favourite(local) = %v", err)
	}
	if got := e.sent(t, local.ID); len(got) != 0 {
		t.Errorf("activities of the favourite of a local post = %+v, want none", got)
	}
}

type remoteInteractionSent struct {
	Type    federation.ActivityType
	Raw     []byte
	Inboxes []string
}

// sent runs the pending fan-out jobs of boosts and favourites and returns the activities
// recorded about the favourites of a status, oldest first, with their documents and inboxes.
func (e *remoteInteractionEnv) sent(t *testing.T, statusID string) []remoteInteractionSent {
	t.Helper()
	ctx := t.Context()
	fanOut, _, ok := e.handlers.Lookup(appfederation.JobFanOut)
	if !ok {
		t.Fatal("no handler for " + appfederation.JobFanOut)
	}
	for {
		nj, ok := e.jobs.pop()
		if !ok {
			break
		}
		if nj.Type != appfederation.JobFanOut {
			continue
		}
		var p struct {
			ID string `json:"outbound_activity_id"`
		}
		if err := json.Unmarshal(nj.Payload, &p); err != nil {
			t.Fatal(err)
		}
		act, err := e.ih.Federation.FindOutboundActivity(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if act.ObjectKind != federation.OutboundFavourite && act.ObjectKind != federation.OutboundBoost {
			continue // the Create of a local post
		}
		j, err := nj.Build(e.ids.NewID(), e.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		if r := fanOut(ctx, j); !r.IsDone() {
			t.Fatalf("fan-out %s = %+v (%v), want Done", j.Payload, r, r.Err())
		}
	}
	acts, err := e.ih.Federation.ListOutboundActivitiesOf(ctx, federation.OutboundFavourite, statusID)
	if err != nil {
		t.Fatalf("ListOutboundActivitiesOf: %v", err)
	}
	out := []remoteInteractionSent{}
	for _, a := range acts {
		s := remoteInteractionSent{Type: a.Type}
		if s.Raw, err = e.ih.Federation.FindOutboundDocument(ctx, a.ID); err != nil {
			t.Fatalf("FindOutboundDocument(%s): %v", a.ID, err)
		}
		ds, err := e.ih.Federation.ListDeliveries(ctx, a.ID, "", 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			s.Inboxes = append(s.Inboxes, string(d.InboxURL))
		}
		out = append(out, s)
	}
	return out
}

// remoteInteractionOrder processes an Announce and a Like with their Undo, and their repeats, in
// every order of inboxSequences, each on a new database: whatever the order, the boost and the
// favourite end up taken back and the counters at zero (NFR-REL-1).
func remoteInteractionOrder(t *testing.T, newHarness RemoteInteractionFactory) {
	const alice = "https://remote.example/users/alice"
	for _, kind := range []string{"Announce", "Like"} {
		t.Run(kind, func(t *testing.T) {
			for _, seq := range inboxSequences(2) {
				e := newRemoteInteractionEnv(t, newHarness)
				post := e.localPost(t, e.user(t, "author"), content.VisibilityPublic)
				aliceID := e.actor(t, alice)
				act := alice + "/" + strings.ToLower(kind) + "s/1"
				docs := [][]byte{
					remoteAddressed(t, kind, act, alice, post.URI),
					remoteActivity(t, "Undo", act+"#undo", alice, act),
				}
				uris := make([]federation.IRI, len(docs))
				for i, doc := range docs {
					uris[i] = e.record(t, doc)
				}
				for _, i := range seq {
					if _, err := e.processor.Process(t.Context(), uris[i], federation.TrustSigned); err != nil {
						t.Fatalf("order %v: Process(%s) = %v", seq, uris[i], err)
					}
				}
				boosts, favourites := e.counters(t, post.ID)
				_, boosted := e.liveBoost(t, aliceID, post.ID)
				if boosted || e.favourited(t, aliceID, post.ID) || boosts != 0 || favourites != 0 {
					t.Errorf("order %v: boosted %t, favourited %t, counters %d/%d; want everything taken back",
						seq, boosted, e.favourited(t, aliceID, post.ID), boosts, favourites)
				}
			}
		})
	}
}
