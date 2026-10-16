// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/instance"
)

// PostingHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the identity, account, graph, instance, and status repositories on it.
type PostingHarness struct {
	AccountHarness
	Graph    appgraph.Repository
	Instance appinstance.Repository
	Statuses appcontent.Repository
}

// PostingFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type PostingFactory func(t *testing.T, opts uow.Options) PostingHarness

// PostingContract verifies the content module's posting use cases on a database adapter:
// posting with mentions, hashtags, and links rendered to HTML; returning the same status for a
// repeated Idempotency-Key, also under concurrency; the max_status_length setting; rejecting
// invalid input; capping the visibility through the RestrictionPolicy; replies to statuses the
// author may see; the default post language; editing into new revisions that move the head;
// deleting into a tombstone that removes the status from the projections; and the emitted
// domain events.
func PostingContract(t *testing.T, newHarness PostingFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *postingEnv)
	}{
		{"PostsAStatus", postingPosts},
		{"ReturnsTheSameStatusForAnIdempotencyKey", postingIdempotent},
		{"AgreesOnConcurrentIdempotencyKeys", postingIdempotentConcurrent},
		{"EnforcesMaxStatusLength", postingMaxLength},
		{"RejectsInvalidInput", postingRejectsInvalid},
		{"AppliesRestrictions", postingRestrictions},
		{"RepliesToVisibleStatuses", postingReplies},
		{"UsesTheDefaultLanguage", postingDefaultLanguage},
		{"EditsIntoNewRevisions", postingEdits},
		{"DeletesIntoATombstone", postingDeletes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newPostingEnv(t, newHarness)) })
	}
}

type postingEnv struct {
	*accountEnv
	repo         appcontent.Repository
	graph        *appgraph.Service
	instance     *appinstance.Service
	content      *appcontent.Service
	restrictions *fakeRestrictions
	languages    map[string]string // default post language by user ID
	events       *eventLog
}

// fakeRestrictions is a RestrictionPolicy whose restrictions the test sets per account.
type fakeRestrictions struct {
	mu   sync.Mutex
	byID map[string]appcontent.Restrictions
}

func (f *fakeRestrictions) Restrictions(_ context.Context, accountID string) (appcontent.Restrictions, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byID[accountID], nil
}

func (f *fakeRestrictions) set(accountID string, r appcontent.Restrictions) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[accountID] = r
}

func newPostingEnv(t *testing.T, newHarness PostingFactory) *postingEnv {
	t.Helper()
	var (
		ph       PostingHarness
		captured uow.Options
	)
	ae := newAccountEnv(t, func(t *testing.T, opts uow.Options) AccountHarness {
		captured = opts
		ph = newHarness(t, opts)
		return ph.AccountHarness
	})
	appgraph.RegisterProjectors(captured.Projectors, ph.Graph)
	appinstance.RegisterProjectors(captured.Projectors, ph.Instance)
	appcontent.RegisterProjectors(captured.Projectors, ph.Statuses)
	log := &eventLog{}
	for _, typ := range []string{appcontent.EventStatusCreated, appcontent.EventStatusRevised, appcontent.EventStatusDeleted} {
		captured.Projectors.Register(typ, func(_ context.Context, _ uow.Tx, e uow.Event) error {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.events = append(log.events, e)
			return nil
		})
	}
	e := &postingEnv{
		accountEnv:   ae,
		repo:         ph.Statuses,
		restrictions: &fakeRestrictions{byID: map[string]appcontent.Restrictions{}},
		languages:    map[string]string{},
		events:       log,
		graph: appgraph.NewService(appgraph.Deps{
			UnitOfWork: ph.UnitOfWork, Repository: ph.Graph, Accounts: ae.accounts, Clock: ae.clock, IDs: ae.ids, Host: contractHost,
		}),
		instance: appinstance.NewService(appinstance.Deps{
			UnitOfWork: ph.UnitOfWork, Repository: ph.Instance, Clock: ae.clock, IDs: ae.ids,
		}),
	}
	e.content = appcontent.NewService(appcontent.Deps{
		UnitOfWork:      ph.UnitOfWork,
		Repository:      ph.Statuses,
		Accounts:        ae.accounts,
		Follows:         e.graph,
		Restrictions:    e.restrictions,
		MaxStatusLength: e.instance.MaxStatusLength,
		DefaultLanguage: func(_ context.Context, userID string) (string, error) { return e.languages[userID], nil },
		Clock:           ae.clock,
		IDs:             ae.ids,
		Host:            contractHost,
		AccountDomain:   "example.org",
	})
	return e
}

func (e *postingEnv) user(t *testing.T, username string) string {
	t.Helper()
	return e.register(t, username).AccountID
}

func (e *postingEnv) post(t *testing.T, in appcontent.CreateInput) appcontent.Post {
	t.Helper()
	e.clock.Advance(time.Second)
	p, err := e.content.Create(t.Context(), in)
	if err != nil {
		t.Fatalf("Create(%+v) = %v", in, err)
	}
	return p
}

func (e *postingEnv) postText(t *testing.T, author, text string, v content.Visibility) appcontent.Post {
	t.Helper()
	return e.post(t, appcontent.CreateInput{AuthorAccountID: author, TextInput: appcontent.TextInput{Text: text}, Visibility: v})
}

func (e *postingEnv) setMaxLength(t *testing.T, n string) {
	t.Helper()
	if _, err := e.instance.SetSetting(t.Context(), appinstance.SetSettingInput{Key: instance.SettingMaxStatusLength, Value: n}); err != nil {
		t.Fatalf("SetSetting(max_status_length, %s) = %v", n, err)
	}
}

// head returns the head of a status.
func (e *postingEnv) head(t *testing.T, statusID string) content.Head {
	t.Helper()
	var h content.Head
	err := e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		var err error
		h, err = e.repo.Head(t.Context(), tx, statusID)
		return err
	})
	if err != nil {
		t.Fatalf("Head(%s) = %v", statusID, err)
	}
	return h
}

// mentionsOf returns the accounts that status_mention_index holds for a status.
func (e *postingEnv) mentionsOf(t *testing.T, statusID string) []string {
	t.Helper()
	all, err := e.repo.ListMentions(t.Context())
	if err != nil {
		t.Fatalf("ListMentions = %v", err)
	}
	var out []string
	for _, m := range all {
		if m.StatusID == statusID {
			out = append(out, m.AccountID)
		}
	}
	return out
}

func eventTypes(events []uow.Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.EventType())
	}
	return out
}

func postingPosts(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice, bob := e.user(t, "alice"), e.user(t, "Bob")
	e.events.take()
	p := e.post(t, appcontent.CreateInput{
		AuthorAccountID: alice,
		TextInput: appcontent.TextInput{
			Text:        "hi @bob and @bob@example.org, @carol and @dave@remote.example\n#Go #ＧＯ <b>https://example.com</b>",
			SpoilerText: "cw", Sensitive: true, Language: "ja_jp",
		},
		Visibility:     content.VisibilityUnlisted,
		ClientID:       "client-1",
		IdempotencyKey: "key-1",
	})
	st, rev := p.Status, p.Revision
	if st.URI != content.StatusURI(contractHost, st.ID) || st.URL != "https://"+contractHost+"/@alice/"+st.ID {
		t.Errorf("status URIs = %q, %q", st.URI, st.URL)
	}
	if st.AuthorAccountID != alice || !st.IsLocal || st.Visibility != content.VisibilityUnlisted || st.InReplyToURI != "" ||
		st.ConversationURI != st.URI || !st.PublishedAt.Equal(e.clock.Now()) {
		t.Errorf("status = %+v", st)
	}
	if rev.Seq != 1 || rev.SpoilerText != "cw" || !rev.Sensitive || rev.Language != "ja-JP" || !rev.RevisedAt.Equal(st.PublishedAt) {
		t.Errorf("revision = %+v", rev)
	}
	if rev.Source == nil || rev.Source.Format != content.SourceTextPlain || !strings.HasPrefix(rev.Source.Text, "hi @bob") {
		t.Errorf("source = %+v, want the text as typed", rev.Source)
	}
	if !reflect.DeepEqual(rev.MentionedAccountIDs, []string{bob}) {
		t.Errorf("mentions = %v, want bob once", rev.MentionedAccountIDs)
	}
	bobLink := `<a href="https://` + contractHost + `/@Bob" class="u-url mention"`
	if len(rev.Bodies) != 1 || rev.Bodies[0].Language != "ja-JP" {
		t.Fatalf("bodies = %+v, want one in ja-JP", rev.Bodies)
	}
	html := rev.Bodies[0].ContentHTML
	for _, want := range []string{
		bobLink, "@<span>Bob</span>", "@carol", "@dave@remote.example", "&lt;b&gt;",
		`href="https://` + contractHost + `/tags/go" class="mention hashtag"`, `<a href="https://example.com"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML %s does not contain %s", html, want)
		}
	}
	if strings.Count(html, bobLink) != 2 {
		t.Errorf("HTML %s should link both mentions of bob", html)
	}
	if len(rev.TagIDs) != 1 {
		t.Fatalf("tags = %v, want one for #Go and #ＧＯ", rev.TagIDs)
	}
	tag, err := e.repo.FindTagByName(ctx, "go")
	if err != nil || tag.ID != rev.TagIDs[0] || tag.Name != "Go" {
		t.Errorf("tag = %+v, %v; want #Go", tag, err)
	}

	got, err := e.content.Find(ctx, st.ID)
	if err != nil || got.Status != st || got.Revision.ID != rev.ID || got.Revision.Bodies[0].ContentHTML != html {
		t.Errorf("Find = %+v, %v; want the posted status", got, err)
	}
	ls, err := e.repo.FindLocalStatus(ctx, st.ID)
	if err != nil || ls != (content.LocalStatus{StatusID: st.ID, AuthorAccountID: alice, ClientID: "client-1", IdempotencyKey: "key-1"}) {
		t.Errorf("local status = %+v, %v", ls, err)
	}
	if h := e.head(t, st.ID); h.RevisionID != rev.ID || h.Edited() {
		t.Errorf("head = %+v, want revision 1", h)
	}
	if m := e.mentionsOf(t, st.ID); !reflect.DeepEqual(m, []string{bob}) {
		t.Errorf("mention index = %v, want bob", m)
	}
	if c, err := e.repo.FindPostCounter(ctx, alice); err != nil || c.StatusesCount != 1 {
		t.Errorf("post counter = %+v, %v; want 1", c, err)
	}
	events := e.events.take()
	if len(events) != 1 {
		t.Fatalf("events = %v, want one", eventTypes(events))
	}
	if ev, ok := events[0].(appcontent.StatusCreated); !ok || ev.Status != st || ev.Revision.ID != rev.ID || !reflect.DeepEqual(ev.Revision.TagIDs, rev.TagIDs) {
		t.Errorf("event = %+v, want StatusCreated of the status", events[0])
	}

	// A later post reuses the stored tag.
	p2 := e.postText(t, bob, "#go again", content.VisibilityPublic)
	if !reflect.DeepEqual(p2.Revision.TagIDs, rev.TagIDs) {
		t.Errorf("tags of the second post = %v, want %v", p2.Revision.TagIDs, rev.TagIDs)
	}
}

func postingIdempotent(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice, bob := e.user(t, "alice"), e.user(t, "bob")
	in := appcontent.CreateInput{AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "once"}, Visibility: content.VisibilityPublic, IdempotencyKey: "k"}
	first := e.post(t, in)
	e.events.take()
	in.Text = "a retry that differs"
	again := e.post(t, in)
	if again.Status != first.Status || again.Revision.ID != first.Revision.ID {
		t.Errorf("retry = %+v, want the first post %+v", again.Status, first.Status)
	}
	if ev := e.events.take(); len(ev) != 0 {
		t.Errorf("retry emitted %v, want nothing", eventTypes(ev))
	}
	if c, _ := e.repo.FindPostCounter(ctx, alice); c.StatusesCount != 1 {
		t.Errorf("statuses = %d after a retry, want 1", c.StatusesCount)
	}
	// The key is per author, and posts without a key are never merged.
	in.AuthorAccountID = bob
	if other := e.post(t, in); other.Status.ID == first.Status.ID {
		t.Error("another author's post with the same key returned the first post")
	}
	a := e.postText(t, alice, "no key", content.VisibilityPublic)
	b := e.postText(t, alice, "no key", content.VisibilityPublic)
	if a.Status.ID == b.Status.ID {
		t.Error("two posts without a key are the same post")
	}
	// A retry after the deletion does not post again.
	if _, err := e.content.Delete(ctx, alice, first.Status.ID); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	in.AuthorAccountID = alice
	if _, err := e.content.Create(ctx, in); !errors.Is(err, appcontent.ErrAlreadyDeleted) {
		t.Errorf("retry after deletion = %v, want ErrAlreadyDeleted", err)
	}
}

func postingIdempotentConcurrent(t *testing.T, e *postingEnv) {
	alice := e.user(t, "alice")
	const n = 8
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			p, err := e.content.Create(t.Context(), appcontent.CreateInput{
				AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "race"}, Visibility: content.VisibilityPublic, IdempotencyKey: "same",
			})
			ids[i], errs[i] = p.Status.ID, err
		})
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Errorf("request %d = %s, %v; want %s", i, ids[i], errs[i], ids[0])
		}
	}
	if c, _ := e.repo.FindPostCounter(t.Context(), alice); c.StatusesCount != 1 {
		t.Errorf("statuses = %d, want 1", c.StatusesCount)
	}
}

func postingMaxLength(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice := e.user(t, "alice")
	create := func(text, cw string) error {
		_, err := e.content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: text, SpoilerText: cw}, Visibility: content.VisibilityPublic,
		})
		return err
	}
	if err := create(strings.Repeat("あ", 500), ""); err != nil {
		t.Errorf("500 characters with the default limit = %v", err)
	}
	if err := create(strings.Repeat("a", 501), ""); !errors.Is(err, appcontent.ErrTextTooLong) {
		t.Errorf("501 characters = %v, want ErrTextTooLong", err)
	}
	e.setMaxLength(t, "10")
	if err := create("0123456789", ""); err != nil {
		t.Errorf("10 characters with limit 10 = %v", err)
	}
	if err := create("01234", "567890"); !errors.Is(err, appcontent.ErrTextTooLong) {
		t.Errorf("text and content warning of 11 characters = %v, want ErrTextTooLong", err)
	}
	p := e.postText(t, alice, "short", content.VisibilityPublic)
	if _, err := e.content.Edit(ctx, appcontent.EditInput{AccountID: alice, StatusID: p.Status.ID, TextInput: appcontent.TextInput{Text: "much too long"}}); !errors.Is(err, appcontent.ErrTextTooLong) {
		t.Errorf("Edit over the limit = %v, want ErrTextTooLong", err)
	}
}

func postingRejectsInvalid(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice := e.user(t, "alice")
	deactivated := e.user(t, "gone")
	if _, err := e.accounts.RecordLifecycleEvent(ctx, appaccount.LifecycleInput{
		AccountID: deactivated, Kind: account.LifecycleDeactivated, InitiatedBy: account.InitiatedBySelf,
	}); err != nil {
		t.Fatalf("RecordLifecycleEvent = %v", err)
	}
	ok := appcontent.CreateInput{AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "ok"}, Visibility: content.VisibilityPublic}
	for _, tc := range []struct {
		name   string
		modify func(*appcontent.CreateInput)
		want   error
	}{
		{"empty text", func(in *appcontent.CreateInput) { in.Text = "" }, appcontent.ErrEmptyText},
		{"blank text", func(in *appcontent.CreateInput) { in.Text = " \n\t" }, appcontent.ErrEmptyText},
		{"NUL", func(in *appcontent.CreateInput) { in.Text = "a\x00b" }, appcontent.ErrInvalidText},
		{"invalid UTF-8", func(in *appcontent.CreateInput) { in.Text = "a\xffb" }, appcontent.ErrInvalidText},
		{"NUL in content warning", func(in *appcontent.CreateInput) { in.SpoilerText = "\x00" }, appcontent.ErrInvalidText},
		{"visibility", func(in *appcontent.CreateInput) { in.Visibility = "everyone" }, content.ErrInvalidVisibility},
		{"language", func(in *appcontent.CreateInput) { in.Language = "not a tag" }, appcontent.ErrInvalidLanguage},
		{"unknown author", func(in *appcontent.CreateInput) { in.AuthorAccountID = "missing" }, appcontent.ErrAccountUnavailable},
		{"deactivated author", func(in *appcontent.CreateInput) { in.AuthorAccountID = deactivated }, appcontent.ErrAccountUnavailable},
		{"unknown reply target", func(in *appcontent.CreateInput) { in.InReplyToID = "missing" }, appcontent.ErrReplyTargetNotFound},
	} {
		in := ok
		tc.modify(&in)
		if _, err := e.content.Create(ctx, in); !errors.Is(err, tc.want) {
			t.Errorf("%s: Create = %v, want %v", tc.name, err, tc.want)
		}
	}
	if c, err := e.repo.FindPostCounter(ctx, alice); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("post counter = %+v, %v; want no statuses", c, err)
	}
}

func postingRestrictions(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice := e.user(t, "alice")
	e.restrictions.set(alice, appcontent.Restrictions{Author: content.AuthorRestrictions{VisibilityCeilingPrivate: true}})
	for _, tc := range []struct{ requested, want content.Visibility }{
		{content.VisibilityPublic, content.VisibilityPrivate},
		{content.VisibilityLocal, content.VisibilityPrivate},
		{content.VisibilityUnlisted, content.VisibilityPrivate},
		{content.VisibilityPrivate, content.VisibilityPrivate},
		{content.VisibilityDirect, content.VisibilityDirect},
	} {
		if p := e.postText(t, alice, "silenced", tc.requested); p.Status.Visibility != tc.want {
			t.Errorf("visibility %s under the ceiling = %s, want %s", tc.requested, p.Status.Visibility, tc.want)
		}
	}
	p := e.postText(t, alice, "before", content.VisibilityPublic)
	e.restrictions.set(alice, appcontent.Restrictions{Post: true})
	if _, err := e.content.Create(ctx, appcontent.CreateInput{AuthorAccountID: alice, TextInput: appcontent.TextInput{Text: "x"}, Visibility: content.VisibilityPublic}); !errors.Is(err, appcontent.ErrPostingRestricted) {
		t.Errorf("Create under the post restriction = %v, want ErrPostingRestricted", err)
	}
	if _, err := e.content.Edit(ctx, appcontent.EditInput{AccountID: alice, StatusID: p.Status.ID, TextInput: appcontent.TextInput{Text: "x"}}); !errors.Is(err, appcontent.ErrPostingRestricted) {
		t.Errorf("Edit under the post restriction = %v, want ErrPostingRestricted", err)
	}
	if _, err := e.content.Delete(ctx, alice, p.Status.ID); err != nil {
		t.Errorf("Delete under the post restriction = %v, want allowed", err)
	}
}

func postingReplies(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice, bob, carol := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol")
	root := e.postText(t, alice, "root", content.VisibilityPublic)
	reply := e.post(t, appcontent.CreateInput{AuthorAccountID: bob, TextInput: appcontent.TextInput{Text: "reply"}, Visibility: content.VisibilityPublic, InReplyToID: root.Status.ID})
	if reply.Status.InReplyToURI != root.Status.URI || reply.Status.ConversationURI != root.Status.URI {
		t.Errorf("reply = %+v, want in reply to and in the conversation of %s", reply.Status, root.Status.URI)
	}
	nested := e.post(t, appcontent.CreateInput{AuthorAccountID: carol, TextInput: appcontent.TextInput{Text: "nested"}, Visibility: content.VisibilityPublic, InReplyToID: reply.Status.ID})
	if nested.Status.InReplyToURI != reply.Status.URI || nested.Status.ConversationURI != root.Status.URI {
		t.Errorf("nested reply = %+v, want the root's conversation", nested.Status)
	}
	if c, err := e.repo.FindCounters(ctx, root.Status.ID); err != nil || c.RepliesCount != 1 {
		t.Errorf("root counters = %+v, %v; want 1 reply", c, err)
	}

	replyTo := func(author, target string) error {
		_, err := e.content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: author, TextInput: appcontent.TextInput{Text: "re"}, Visibility: content.VisibilityPrivate, InReplyToID: target,
		})
		return err
	}
	private := e.postText(t, alice, "followers only", content.VisibilityPrivate)
	if err := replyTo(bob, private.Status.ID); !errors.Is(err, appcontent.ErrReplyTargetNotFound) {
		t.Errorf("reply to a hidden private status = %v, want ErrReplyTargetNotFound", err)
	}
	if err := replyTo(alice, private.Status.ID); err != nil {
		t.Errorf("reply to one's own private status = %v", err)
	}
	if _, err := e.graph.Follow(ctx, bob, alice); err != nil {
		t.Fatalf("Follow = %v", err)
	}
	if err := replyTo(bob, private.Status.ID); err != nil {
		t.Errorf("follower's reply to a private status = %v", err)
	}
	direct := e.postText(t, alice, "@carol secret", content.VisibilityDirect)
	if err := replyTo(bob, direct.Status.ID); !errors.Is(err, appcontent.ErrReplyTargetNotFound) {
		t.Errorf("reply to a direct status without the mention = %v, want ErrReplyTargetNotFound", err)
	}
	if err := replyTo(carol, direct.Status.ID); err != nil {
		t.Errorf("mentioned account's reply to a direct status = %v", err)
	}
	if _, err := e.content.Delete(ctx, alice, root.Status.ID); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	if err := replyTo(bob, root.Status.ID); !errors.Is(err, appcontent.ErrReplyTargetNotFound) {
		t.Errorf("reply to a deleted status = %v, want ErrReplyTargetNotFound", err)
	}
}

func postingDefaultLanguage(t *testing.T, e *postingEnv) {
	reg := e.register(t, "alice")
	if p := e.postText(t, reg.AccountID, "no preference", content.VisibilityPublic); p.Revision.Language != appcontent.UndeterminedLanguage {
		t.Errorf("language without a default = %q, want und", p.Revision.Language)
	}
	e.languages[reg.User.ID] = "ja"
	if p := e.postText(t, reg.AccountID, "default", content.VisibilityPublic); p.Revision.Language != "ja" || p.Revision.Bodies[0].Language != "ja" {
		t.Errorf("language = %q, want the default ja", p.Revision.Language)
	}
	p := e.post(t, appcontent.CreateInput{AuthorAccountID: reg.AccountID, TextInput: appcontent.TextInput{Text: "explicit", Language: "en"}, Visibility: content.VisibilityPublic})
	if p.Revision.Language != "en" {
		t.Errorf("language = %q, want the requested en", p.Revision.Language)
	}
}

func postingEdits(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice, bob, carol := e.user(t, "alice"), e.user(t, "bob"), e.user(t, "carol")
	p := e.postText(t, alice, "hello @bob #one", content.VisibilityPublic)
	e.events.take()

	e.clock.Advance(time.Minute)
	edited, err := e.content.Edit(ctx, appcontent.EditInput{
		AccountID: alice, StatusID: p.Status.ID,
		TextInput: appcontent.TextInput{Text: "hello @carol #two", SpoilerText: "cw", Sensitive: true, Language: "en"},
	})
	if err != nil {
		t.Fatalf("Edit = %v", err)
	}
	rev := edited.Revision
	if edited.Status != p.Status || rev.Seq != 2 || rev.ID == p.Revision.ID || rev.SpoilerText != "cw" || !rev.Sensitive ||
		rev.Language != "en" || !rev.RevisedAt.Equal(e.clock.Now()) {
		t.Errorf("Edit = %+v", edited)
	}
	if !reflect.DeepEqual(rev.MentionedAccountIDs, []string{carol}) || !strings.Contains(rev.Bodies[0].ContentHTML, "#<span>two</span>") {
		t.Errorf("edited revision = %+v, want carol and #two", rev)
	}
	if h := e.head(t, p.Status.ID); h.RevisionID != rev.ID || h.Seq != 2 || !h.Edited() || !h.LastRevisedAt.Equal(rev.RevisedAt) {
		t.Errorf("head = %+v, want revision 2", h)
	}
	if m := e.mentionsOf(t, p.Status.ID); !reflect.DeepEqual(m, []string{carol}) {
		t.Errorf("mention index = %v, want carol instead of bob", m)
	}
	if got, err := e.content.Find(ctx, p.Status.ID); err != nil || got.Revision.ID != rev.ID {
		t.Errorf("Find = %+v, %v; want the new revision", got.Revision, err)
	}
	history, err := e.content.Revisions(ctx, p.Status.ID)
	if err != nil || len(history) != 2 || history[0].ID != p.Revision.ID || history[1].ID != rev.ID {
		t.Errorf("Revisions = %+v, %v; want both, oldest first", history, err)
	}
	events := e.events.take()
	if len(events) != 1 {
		t.Fatalf("events = %v, want one", eventTypes(events))
	}
	if ev, ok := events[0].(appcontent.StatusRevised); !ok || ev.Status != p.Status || ev.Revision.ID != rev.ID {
		t.Errorf("event = %+v, want StatusRevised of the status", events[0])
	}

	if _, err := e.content.Edit(ctx, appcontent.EditInput{AccountID: bob, StatusID: p.Status.ID, TextInput: appcontent.TextInput{Text: "mine"}}); !errors.Is(err, appcontent.ErrNotAuthor) {
		t.Errorf("Edit by another account = %v, want ErrNotAuthor", err)
	}
	if _, err := e.content.Edit(ctx, appcontent.EditInput{AccountID: alice, StatusID: "missing", TextInput: appcontent.TextInput{Text: "x"}}); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Edit of a missing status = %v, want ErrNotFound", err)
	}
	if _, err := e.content.Delete(ctx, alice, p.Status.ID); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	if _, err := e.content.Edit(ctx, appcontent.EditInput{AccountID: alice, StatusID: p.Status.ID, TextInput: appcontent.TextInput{Text: "x"}}); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Edit of a deleted status = %v, want ErrNotFound", err)
	}
}

func postingDeletes(t *testing.T, e *postingEnv) {
	ctx := t.Context()
	alice, bob := e.user(t, "alice"), e.user(t, "bob")
	p := e.postText(t, alice, "bye @bob", content.VisibilityPublic)
	keep := e.postText(t, alice, "stays", content.VisibilityPublic)
	e.events.take()

	if _, err := e.content.Delete(ctx, bob, p.Status.ID); !errors.Is(err, appcontent.ErrNotAuthor) {
		t.Errorf("Delete by another account = %v, want ErrNotAuthor", err)
	}
	e.clock.Advance(time.Minute)
	d, err := e.content.Delete(ctx, alice, p.Status.ID)
	if err != nil {
		t.Fatalf("Delete = %v", err)
	}
	want := content.Deletion{StatusID: p.Status.ID, URI: p.Status.URI, AuthorAccountID: alice, Origin: content.DeletedByAuthor, DeletedAt: e.clock.Now()}
	if d != want {
		t.Errorf("Delete = %+v, want %+v", d, want)
	}
	if got, err := e.repo.FindDeletionByURI(ctx, p.Status.URI); err != nil || got.StatusID != p.Status.ID {
		t.Errorf("tombstone = %+v, %v", got, err)
	}
	if _, err := e.content.Find(ctx, p.Status.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Find after deletion = %v, want ErrNotFound", err)
	}
	if m := e.mentionsOf(t, p.Status.ID); len(m) != 0 {
		t.Errorf("mention index = %v after deletion, want none", m)
	}
	if c, err := e.repo.FindPostCounter(ctx, alice); err != nil || c.StatusesCount != 1 || !c.LastStatusAt.Equal(keep.Status.PublishedAt) {
		t.Errorf("post counter = %+v, %v; want the remaining status", c, err)
	}
	events := e.events.take()
	if len(events) != 1 {
		t.Fatalf("events = %v, want one", eventTypes(events))
	}
	if ev, ok := events[0].(appcontent.StatusDeleted); !ok || ev.Status != p.Status || ev.Deletion != d {
		t.Errorf("event = %+v, want StatusDeleted of the status", events[0])
	}
	if _, err := e.content.Delete(ctx, alice, p.Status.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("Delete again = %v, want ErrNotFound", err)
	}
	if ev := e.events.take(); len(ev) != 0 {
		t.Errorf("a second deletion emitted %v", eventTypes(ev))
	}
}
