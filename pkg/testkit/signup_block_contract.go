// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
	"github.com/best-friends/my/internal/domain/moderation"
)

// SignupBlockContract verifies the sign-up block list of a database adapter's moderation
// repository (FR-719, data-model §4.3) through the moderation use cases: blocks are stored in the
// canonical form of their kind and invalid ones are refused, a sign-up is screened against the
// blocks in effect only (neither lifted nor expired) by the domain of its email address and its
// parents, the hash of its canonical address, and the IP range of its client, the strongest
// severity wins, a lift is recorded once, a ban can add the block of the banned user's email
// address in its transaction, the blocks list newest first with their lifts and filters, and
// every addition and lift is audited once.
func SignupBlockContract(t *testing.T, newHarness ModerationFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *signupBlockEnv)
	}{
		{"StoresCanonicalValues", signupBlocksCanonical},
		{"ScreensSignUpsWithTheBlocksInEffect", signupBlocksScreen},
		{"LiftsOnceAndAudits", signupBlocksLiftAndAudit},
		{"BanBlocksTheEmailAddress", signupBlocksFromBan},
		{"ListsWithFiltersAndPages", signupBlocksList},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newSignupBlockEnv(t, newHarness)) })
	}
}

type signupBlockEnv struct {
	h      ModerationHarness
	clock  *Clock
	ids    *IDGenerator
	svc    *appmoderation.Service
	emails stubEmails
	// local is a local account of the user localUser; remote is a remote account.
	local, localUser, remote string
}

// stubEmails stands in for the identity module: the primary email address of each user.
type stubEmails map[string]string

// PrimaryEmailAddress implements appmoderation.Emails.
func (s stubEmails) PrimaryEmailAddress(_ context.Context, userID string) (string, error) {
	return s[userID], nil
}

func newSignupBlockEnv(t *testing.T, newHarness ModerationFactory) *signupBlockEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	appmoderation.RegisterAuditProjector(projectors, h.Moderation)
	clock := NewClock(time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	ids := NewIDGenerator(clock)
	actors := &stubActors{actors: map[string]appaccount.Actor{}}
	e := &signupBlockEnv{h: h, clock: clock, ids: ids, emails: stubEmails{}, local: ids.NewID(), localUser: ids.NewID(), remote: ids.NewID()}
	actors.set(appaccount.Actor{
		Account: account.Account{ID: e.local, Domain: account.LocalDomain},
		Local:   &account.LocalAccount{AccountID: e.local, UserID: e.localUser, Username: "spammer"},
	})
	actors.set(appaccount.Actor{Account: account.Account{ID: e.remote, Domain: "remote.example", URI: "https://remote.example/users/bob"}})
	e.emails[e.localUser] = "Spam.Mer+tag@Mail.Example"
	e.svc = appmoderation.NewService(appmoderation.Deps{
		UnitOfWork: h.UnitOfWork, Repository: h.Moderation, Accounts: actors, Clock: clock, IDs: ids,
		Lifecycles: &stubLifecycles{due: map[string]time.Time{}}, Emails: e.emails,
	})
	return e
}

func (e *signupBlockEnv) add(t *testing.T, in appmoderation.SignupBlockInput) moderation.SignupBlock {
	t.Helper()
	if in.Severity == "" {
		in.Severity = moderation.SignupReject
	}
	b, err := e.svc.AddSignupBlock(t.Context(), in)
	if err != nil {
		t.Fatalf("AddSignupBlock(%+v) = %v", in, err)
	}
	return b
}

func (e *signupBlockEnv) screen(t *testing.T, email, ip string, want moderation.SignupBlockSeverity) {
	t.Helper()
	got, err := e.svc.ScreenSignup(t.Context(), email, ip)
	if err != nil || got != want {
		t.Errorf("ScreenSignup(%q, %q) = %q, %v; want %q", email, ip, got, err, want)
	}
}

func signupBlocksCanonical(t *testing.T, e *signupBlockEnv) {
	hash := moderation.EmailHash("alice@example.com")
	for _, c := range []struct {
		kind       moderation.SignupBlockKind
		value, out string
	}{
		{moderation.SignupBlockEmailDomain, "@Spam.Example.", "spam.example"},
		{moderation.SignupBlockEmailHash, "Alice+x@example.com", hash},
		{moderation.SignupBlockEmailHash, strings.ToUpper(hash), hash},
		{moderation.SignupBlockIPRange, "192.0.2.77/24", "192.0.2.0/24"},
		{moderation.SignupBlockIPRange, "2001:db8::1", "2001:db8::1/128"},
		{moderation.SignupBlockIPRange, "::ffff:198.51.100.0/120", "198.51.100.0/24"},
	} {
		b := e.add(t, appmoderation.SignupBlockInput{Kind: c.kind, Value: c.value})
		if b.Value != c.out {
			t.Errorf("AddSignupBlock(%s %q).Value = %q, want %q", c.kind, c.value, b.Value, c.out)
		}
		v, err := e.svc.SignupBlock(t.Context(), b.ID)
		if err != nil || !reflect.DeepEqual(v.Block, b) || v.State != moderation.ActionActive || v.Lift != nil {
			t.Errorf("SignupBlock(%s) = %+v, %v; want %+v, active", b.ID, v, err, b)
		}
	}
	for _, in := range []appmoderation.SignupBlockInput{
		{Kind: "email", Value: "spam.example", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockEmailDomain, Value: "spam.example", Severity: "maybe"},
		{Kind: moderation.SignupBlockEmailDomain, Value: "", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockEmailDomain, Value: "192.0.2.1", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockEmailHash, Value: "abc", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockEmailHash, Value: "@", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/33", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockIPRange, Value: "spam.example", Severity: moderation.SignupReject},
		{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24", Severity: moderation.SignupReject, Duration: -time.Hour},
		{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24", Severity: moderation.SignupReject, Comment: strings.Repeat("x", moderation.MaxSignupBlockComment+1)},
	} {
		if _, err := e.svc.AddSignupBlock(t.Context(), in); !errors.Is(err, appmoderation.ErrInvalidSignupBlock) {
			t.Errorf("AddSignupBlock(%+v) = %v, want ErrInvalidSignupBlock", in, err)
		}
	}
}

func signupBlocksScreen(t *testing.T, e *signupBlockEnv) {
	e.screen(t, "alice@spam.example", "192.0.2.1", "")
	e.add(t, appmoderation.SignupBlockInput{Kind: moderation.SignupBlockEmailDomain, Value: "spam.example", Severity: moderation.SignupRequireApproval})
	e.add(t, appmoderation.SignupBlockInput{Kind: moderation.SignupBlockEmailHash, Value: "carol@good.example"})
	e.add(t, appmoderation.SignupBlockInput{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24", Duration: time.Hour})
	e.add(t, appmoderation.SignupBlockInput{Kind: moderation.SignupBlockIPRange, Value: "2001:db8::/32", Severity: moderation.SignupRequireApproval})

	// The domain covers its subdomains but not look-alikes; the hash ignores case and +tags.
	e.screen(t, "alice@spam.example", "", moderation.SignupRequireApproval)
	e.screen(t, "alice@mx.Spam.Example", "", moderation.SignupRequireApproval)
	e.screen(t, "alice@notspam.example", "", "")
	e.screen(t, "Carol+again@GOOD.example", "", moderation.SignupReject)
	e.screen(t, "carol2@good.example", "", "")
	// The client address matches by range; IPv4-mapped addresses count as IPv4; an unknown or
	// unparsable address matches no range.
	e.screen(t, "dave@good.example", "192.0.2.200", moderation.SignupReject)
	e.screen(t, "dave@good.example", "::ffff:192.0.2.9", moderation.SignupReject)
	e.screen(t, "dave@good.example", "192.0.3.1", "")
	e.screen(t, "dave@good.example", "2001:db8:1::5", moderation.SignupRequireApproval)
	e.screen(t, "dave@good.example", "not an address", "")
	// The strongest severity wins.
	e.screen(t, "dave@spam.example", "192.0.2.1", moderation.SignupReject)

	// An expired block no longer matches.
	e.clock.Advance(time.Hour)
	e.screen(t, "dave@good.example", "192.0.2.200", "")
	e.screen(t, "dave@spam.example", "192.0.2.1", moderation.SignupRequireApproval)
}

func signupBlocksLiftAndAudit(t *testing.T, e *signupBlockEnv) {
	ctx := t.Context()
	b := e.add(t, appmoderation.SignupBlockInput{Kind: moderation.SignupBlockEmailDomain, Value: "spam.example", Comment: "  waves  ", CreatedByUserID: "mod-1"})
	if b.Comment != "waves" || b.CreatedByUserID != "mod-1" {
		t.Errorf("AddSignupBlock = %+v, want the trimmed comment and the moderator", b)
	}
	e.screen(t, "a@spam.example", "", moderation.SignupReject)
	e.clock.Advance(time.Second)
	if err := e.svc.LiftSignupBlock(ctx, b.ID, ""); err != nil {
		t.Fatalf("LiftSignupBlock = %v", err)
	}
	liftedAt := e.clock.Now()
	e.screen(t, "a@spam.example", "", "")
	// Lifting again changes nothing and records nothing.
	e.clock.Advance(time.Second)
	if err := e.svc.LiftSignupBlock(ctx, b.ID, "mod-1"); err != nil {
		t.Errorf("LiftSignupBlock again = %v, want nil", err)
	}
	v, err := e.svc.SignupBlock(ctx, b.ID)
	if err != nil || v.State != moderation.ActionLifted || v.Lift == nil || !v.Lift.LiftedAt.Equal(liftedAt) || v.Lift.LiftedByUserID != "" {
		t.Errorf("SignupBlock after the lifts = %+v, %v; want lifted once by the CLI at %v", v, err, liftedAt)
	}
	if err := e.svc.LiftSignupBlock(ctx, e.ids.NewID(), ""); !errors.Is(err, appmoderation.ErrNotFound) {
		t.Errorf("LiftSignupBlock(unknown) = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.SignupBlock(ctx, e.ids.NewID()); !errors.Is(err, appmoderation.ErrNotFound) {
		t.Errorf("SignupBlock(unknown) = %v, want ErrNotFound", err)
	}
	got, err := e.svc.AuditLog(ctx, appmoderation.AuditQuery{TargetKind: moderation.AuditTargetSignupBlock, TargetID: b.ID}, 10)
	if err != nil {
		t.Fatal(err)
	}
	checkAudit(t, got, []auditWant{
		{moderation.AuditActorCLI, "", moderation.AuditSignupBlockLifted, moderation.AuditTargetSignupBlock, b.ID, "email_domain spam.example", b.ID, liftedAt},
		{moderation.AuditActorUser, "mod-1", moderation.AuditSignupBlockAdded, moderation.AuditTargetSignupBlock, b.ID, "email_domain spam.example", b.ID, b.CreatedAt},
	})
}

func signupBlocksFromBan(t *testing.T, e *signupBlockEnv) {
	ctx := t.Context()
	ban := func(target string, kind moderation.ActionKind) (moderation.Action, error) {
		return e.svc.IssueAccountAction(ctx, appmoderation.AccountActionInput{
			TargetAccountID: target, Kind: kind, ReasonCategory: moderation.CategorySpam, IssuedByUserID: "mod-1", BlockEmail: true,
		})
	}
	// Only a ban of a local account with an email address blocks it; a refusal writes nothing.
	if _, err := ban(e.local, moderation.ActionSuspend); !errors.Is(err, appmoderation.ErrCannotBlockEmail) {
		t.Errorf("suspend with BlockEmail = %v, want ErrCannotBlockEmail", err)
	}
	if _, err := ban(e.remote, moderation.ActionBan); !errors.Is(err, appmoderation.ErrUnsupportedAction) {
		t.Errorf("ban of a remote account = %v, want ErrUnsupportedAction", err)
	}
	address := e.emails[e.localUser]
	delete(e.emails, e.localUser)
	if _, err := ban(e.local, moderation.ActionBan); !errors.Is(err, appmoderation.ErrCannotBlockEmail) {
		t.Errorf("ban of a user without an email address = %v, want ErrCannotBlockEmail", err)
	}
	if vs, err := e.svc.AccountActions(ctx, appmoderation.AccountActionFilter{AccountID: e.local}, 10); err != nil || len(vs) != 0 {
		t.Errorf("actions after the refusals = %+v, %v; want none", vs, err)
	}
	e.emails[e.localUser] = address

	a, err := ban(e.local, moderation.ActionBan)
	if err != nil {
		t.Fatalf("ban with BlockEmail = %v", err)
	}
	blocks, err := e.svc.SignupBlocks(ctx, appmoderation.SignupBlockFilter{}, 10)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("SignupBlocks = %+v, %v; want the ban's block", blocks, err)
	}
	b := blocks[0].Block
	want := moderation.SignupBlock{
		ID: b.ID, Kind: moderation.SignupBlockEmailHash, Value: moderation.EmailHash("spam.mer@mail.example"), Severity: moderation.SignupReject,
		ModerationActionID: a.ID, CreatedByUserID: "mod-1", CreatedAt: a.IssuedAt,
	}
	if !reflect.DeepEqual(b, want) {
		t.Errorf("ban's block = %+v, want %+v", b, want)
	}
	e.screen(t, "spam.mer+new@mail.example", "", moderation.SignupReject)
	got, err := e.svc.AuditLog(ctx, appmoderation.AuditQuery{TargetKind: moderation.AuditTargetSignupBlock}, 10)
	if err != nil {
		t.Fatal(err)
	}
	checkAudit(t, got, []auditWant{
		{moderation.AuditActorUser, "mod-1", moderation.AuditSignupBlockAdded, moderation.AuditTargetSignupBlock, b.ID, "email_hash " + b.Value, b.ID, a.IssuedAt},
	})
}

func signupBlocksList(t *testing.T, e *signupBlockEnv) {
	ctx := t.Context()
	var all []moderation.SignupBlock
	for _, in := range []appmoderation.SignupBlockInput{
		{Kind: moderation.SignupBlockEmailDomain, Value: "a.example"},
		{Kind: moderation.SignupBlockIPRange, Value: "192.0.2.0/24", Duration: time.Minute},
		{Kind: moderation.SignupBlockEmailDomain, Value: "b.example", Severity: moderation.SignupRequireApproval},
		{Kind: moderation.SignupBlockEmailHash, Value: "x@c.example"},
	} {
		e.clock.Advance(time.Second)
		all = append(all, e.add(t, in))
	}
	if err := e.svc.LiftSignupBlock(ctx, all[2].ID, ""); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Hour)
	ids := func(vs []appmoderation.SignupBlockView) []string {
		out := []string{}
		for _, v := range vs {
			out = append(out, v.Block.ID)
		}
		return out
	}
	check := func(name string, f appmoderation.SignupBlockFilter, limit int, want ...moderation.SignupBlock) []appmoderation.SignupBlockView {
		t.Helper()
		vs, err := e.svc.SignupBlocks(ctx, f, limit)
		w := []string{}
		for _, b := range want {
			w = append(w, b.ID)
		}
		if err != nil || !reflect.DeepEqual(ids(vs), w) {
			t.Errorf("%s: SignupBlocks = %v, %v; want %v", name, ids(vs), err, w)
		}
		return vs
	}
	vs := check("all", appmoderation.SignupBlockFilter{}, 10, all[3], all[2], all[1], all[0])
	if len(vs) == 4 {
		for i, state := range []moderation.ActionState{moderation.ActionActive, moderation.ActionLifted, moderation.ActionExpired, moderation.ActionActive} {
			if vs[i].State != state {
				t.Errorf("state of %s = %s, want %s", vs[i].Block.ID, vs[i].State, state)
			}
		}
	}
	check("active", appmoderation.SignupBlockFilter{Active: true}, 10, all[3], all[0])
	check("email_domain", appmoderation.SignupBlockFilter{Kind: moderation.SignupBlockEmailDomain}, 10, all[2], all[0])
	check("page", appmoderation.SignupBlockFilter{Before: all[2].ID}, 1, all[1])
	if _, err := e.svc.SignupBlocks(ctx, appmoderation.SignupBlockFilter{Kind: "email"}, 10); !errors.Is(err, appmoderation.ErrInvalidQuery) {
		t.Errorf("SignupBlocks(unknown kind) = %v, want ErrInvalidQuery", err)
	}
}
