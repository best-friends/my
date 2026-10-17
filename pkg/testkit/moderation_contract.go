// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
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

// ModerationHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the moderation repository on it.
type ModerationHarness struct {
	UnitOfWork uow.UnitOfWork
	Moderation appmoderation.Repository
}

// ModerationFactory returns a harness on a new, empty database whose UnitOfWork uses opts. The
// factory registers the cleanup of everything it creates with t.Cleanup.
type ModerationFactory func(t *testing.T, opts uow.Options) ModerationHarness

// ModerationContract verifies the moderation repository of a database adapter through the
// moderation use cases and moderation.FederationPolicy (federation.md §11): a domain action
// restricts the domain and its subdomains until it expires or is lifted, a lift ends only its
// own action's restrictions, actions on remote accounts restrict them and actions on local
// accounts are refused, invalid actions are refused, each action is lifted once, every change
// emits the restrictions in effect afterwards, and a received report is filed once, open, with
// its statuses and its comment cut to the limit. The accounts are in-memory stand-ins.
func ModerationContract(t *testing.T, newHarness ModerationFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *moderationEnv)
	}{
		{"DomainActionsCoverSubdomainsUntilTheyEnd", moderationDomainActions},
		{"LiftEndsOnlyItsOwnAction", moderationLifts},
		{"RemoteAccountActions", moderationAccountActions},
		{"RejectsInvalidActions", moderationRejectsInvalid},
		{"RecordsEachLiftOnce", moderationLiftsOnce},
		{"EmitsTheRestrictionsInEffect", moderationEmits},
		{"FilesARemoteReportOnce", moderationReports},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newModerationEnv(t, newHarness)) })
	}
}

type moderationEnv struct {
	h       ModerationHarness
	clock   *Clock
	ids     *IDGenerator
	actors  *stubActors
	svc     *appmoderation.Service
	policy  *appmoderation.FederationPolicy
	events  *eventLog
	remote  string
	local   string
	reports string
}

func newModerationEnv(t *testing.T, newHarness ModerationFactory) *moderationEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	clock := NewClock(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	ids := NewIDGenerator(clock)
	e := &moderationEnv{
		h: h, clock: clock, ids: ids, actors: &stubActors{actors: map[string]appaccount.Actor{}}, events: &eventLog{},
	}
	for _, typ := range []string{appmoderation.EventDomainRestrictionsChanged, appmoderation.EventAccountRestrictionsChanged, appmoderation.EventReportFiled} {
		projectors.Register(typ, e.events.record)
	}
	e.svc = appmoderation.NewService(appmoderation.Deps{
		UnitOfWork: h.UnitOfWork, Repository: h.Moderation, Accounts: e.actors, Clock: clock, IDs: ids,
	})
	e.policy = appmoderation.NewFederationPolicy(h.Moderation, clock)
	e.remote, e.local, e.reports = ids.NewID(), ids.NewID(), ids.NewID()
	e.actors.set(appaccount.Actor{Account: account.Account{ID: e.remote, Domain: "remote.example", URI: "https://remote.example/users/bob"}})
	e.actors.set(appaccount.Actor{Account: account.Account{ID: e.local, Domain: account.LocalDomain}})
	e.actors.set(appaccount.Actor{Account: account.Account{ID: e.reports, Domain: "remote.example", URI: "https://remote.example/actor"}})
	return e
}

func (e *moderationEnv) domainAction(t *testing.T, domain string, kind moderation.DomainActionKind, d time.Duration) moderation.DomainAction {
	t.Helper()
	a, err := e.svc.IssueDomainAction(t.Context(), appmoderation.DomainActionInput{Domain: domain, Kind: kind, Duration: d})
	if err != nil {
		t.Fatalf("IssueDomainAction(%s, %s) = %v", domain, kind, err)
	}
	return a
}

// checkDomain checks every domain predicate of the policy for domain.
func (e *moderationEnv) checkDomain(t *testing.T, domain string, suspended, media, reports bool) {
	t.Helper()
	ctx := t.Context()
	for _, c := range []struct {
		name string
		f    func() (bool, error)
		want bool
	}{
		{"FederationSuspended", func() (bool, error) { return e.policy.FederationSuspended(ctx, domain) }, suspended},
		{"RejectsMedia", func() (bool, error) { return e.policy.RejectsMedia(ctx, domain) }, media},
		{"RejectsReports", func() (bool, error) { return e.policy.RejectsReports(ctx, domain) }, reports},
	} {
		got, err := c.f()
		if err != nil || got != c.want {
			t.Errorf("%s(%q) = %v, %v; want %v", c.name, domain, got, err, c.want)
		}
	}
}

func moderationDomainActions(t *testing.T, e *moderationEnv) {
	e.checkDomain(t, "bad.example", false, false, false)
	e.domainAction(t, "bad.example", moderation.DomainSuspend, 0)
	e.checkDomain(t, "bad.example", true, false, false)
	e.checkDomain(t, "sub.bad.example", true, false, false)
	e.checkDomain(t, "example", false, false, false)
	e.checkDomain(t, "notbad.example", false, false, false)
	e.checkDomain(t, "", false, false, false)

	// A parent domain's action covers its subdomains; an expiring one ends by itself.
	e.domainAction(t, "media.example", moderation.DomainRejectMedia, time.Hour)
	e.domainAction(t, "reports.example", moderation.DomainRejectReports, 0)
	e.checkDomain(t, "cdn.media.example", false, true, false)
	e.checkDomain(t, "reports.example", false, false, true)
	e.clock.Advance(time.Hour)
	e.checkDomain(t, "cdn.media.example", false, false, false)

	rs, err := e.policy.DomainRestrictions(t.Context(), "x.bad.example")
	if want := []moderation.Restriction{moderation.RestrictFederation, moderation.RestrictProfileView}; err != nil || !reflect.DeepEqual(rs, want) {
		t.Errorf("DomainRestrictions = %v, %v; want %v", rs, err, want)
	}
}

func moderationLifts(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	first := e.domainAction(t, "bad.example", moderation.DomainSuspend, 0)
	second := e.domainAction(t, "bad.example", moderation.DomainSuspend, 24*time.Hour)
	limit := e.domainAction(t, "bad.example", moderation.DomainLimit, 0)
	if err := e.svc.LiftDomainAction(ctx, first.ID, ""); err != nil {
		t.Fatalf("LiftDomainAction = %v", err)
	}
	e.checkDomain(t, "bad.example", true, false, false)
	if err := e.svc.LiftDomainAction(ctx, second.ID, ""); err != nil {
		t.Fatalf("LiftDomainAction = %v", err)
	}
	e.checkDomain(t, "bad.example", false, false, false)
	// Lifting again changes nothing; the limit stays.
	if err := e.svc.LiftDomainAction(ctx, second.ID, ""); err != nil {
		t.Errorf("LiftDomainAction again = %v, want nil", err)
	}
	rs, err := e.policy.DomainRestrictions(ctx, "bad.example")
	if want := []moderation.Restriction{moderation.RestrictPublicDiscovery, moderation.RestrictUnsolicitedReach}; err != nil || !reflect.DeepEqual(rs, want) {
		t.Errorf("DomainRestrictions after the lifts = %v, %v; want %v", rs, err, want)
	}
	if err := e.svc.LiftDomainAction(ctx, limit.Domain, ""); !errors.Is(err, appmoderation.ErrNotFound) {
		t.Errorf("LiftDomainAction(unknown) = %v, want ErrNotFound", err)
	}
}

func moderationAccountActions(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	suspended := func(id string, want bool) {
		t.Helper()
		got, err := e.policy.AccountFederationSuspended(ctx, id)
		if err != nil || got != want {
			t.Errorf("AccountFederationSuspended(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
	suspended(e.remote, false)
	a, err := e.svc.IssueRemoteAccountAction(ctx, appmoderation.AccountActionInput{
		TargetAccountID: e.remote, Kind: moderation.ActionSuspend, ReasonCategory: moderation.CategorySpam, Duration: time.Hour,
	})
	if err != nil {
		t.Fatalf("IssueRemoteAccountAction = %v", err)
	}
	suspended(e.remote, true)
	suspended(e.local, false)
	e.clock.Advance(time.Hour)
	suspended(e.remote, false)

	a, err = e.svc.IssueRemoteAccountAction(ctx, appmoderation.AccountActionInput{
		TargetAccountID: e.remote, Kind: moderation.ActionSuspend, ReasonCategory: moderation.CategoryOther,
	})
	if err != nil {
		t.Fatalf("IssueRemoteAccountAction = %v", err)
	}
	suspended(e.remote, true)
	if err := e.svc.LiftAccountAction(ctx, a.ID, "", "appealed"); err != nil {
		t.Fatalf("LiftAccountAction = %v", err)
	}
	suspended(e.remote, false)
	if err := e.svc.LiftAccountAction(ctx, a.ID, "", ""); err != nil {
		t.Errorf("LiftAccountAction again = %v, want nil", err)
	}

	for _, in := range []appmoderation.AccountActionInput{
		{TargetAccountID: e.local, Kind: moderation.ActionSuspend, ReasonCategory: moderation.CategoryOther},
		{TargetAccountID: e.remote, Kind: moderation.ActionWarn, ReasonCategory: moderation.CategoryOther},
		{TargetAccountID: e.remote, Kind: moderation.ActionBan, ReasonCategory: moderation.CategoryOther},
	} {
		if _, err := e.svc.IssueRemoteAccountAction(ctx, in); !errors.Is(err, appmoderation.ErrUnsupportedAction) {
			t.Errorf("IssueRemoteAccountAction(%s of %s) = %v, want ErrUnsupportedAction", in.Kind, in.TargetAccountID, err)
		}
	}
	missing := appmoderation.AccountActionInput{TargetAccountID: e.ids.NewID(), Kind: moderation.ActionLimit, ReasonCategory: moderation.CategoryOther}
	if _, err := e.svc.IssueRemoteAccountAction(ctx, missing); !errors.Is(err, appmoderation.ErrAccountNotFound) {
		t.Errorf("IssueRemoteAccountAction(missing) = %v, want ErrAccountNotFound", err)
	}
}

func moderationRejectsInvalid(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	for _, in := range []appmoderation.DomainActionInput{
		{Domain: "", Kind: moderation.DomainSuspend},
		{Domain: "https://bad.example", Kind: moderation.DomainSuspend},
		{Domain: "Bad.Example", Kind: moderation.DomainSuspend},
		{Domain: "bad..example", Kind: moderation.DomainSuspend},
		{Domain: "bad.example", Kind: "silence"},
	} {
		if _, err := e.svc.IssueDomainAction(ctx, in); !errors.Is(err, moderation.ErrInvalidAction) {
			t.Errorf("IssueDomainAction(%q, %q) = %v, want ErrInvalidAction", in.Domain, in.Kind, err)
		}
	}
	in := appmoderation.AccountActionInput{TargetAccountID: e.remote, Kind: moderation.ActionLimit, ReasonCategory: "rude"}
	if _, err := e.svc.IssueRemoteAccountAction(ctx, in); !errors.Is(err, moderation.ErrInvalidAction) {
		t.Errorf("IssueRemoteAccountAction(category rude) = %v, want ErrInvalidAction", err)
	}
	e.checkDomain(t, "bad.example", false, false, false)
}

func moderationLiftsOnce(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	d := e.domainAction(t, "bad.example", moderation.DomainSuspend, 0)
	a, err := e.svc.IssueRemoteAccountAction(ctx, appmoderation.AccountActionInput{
		TargetAccountID: e.remote, Kind: moderation.ActionLimit, ReasonCategory: moderation.CategoryOther,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		err := e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			return e.h.Moderation.InsertDomainActionLift(ctx, tx, moderation.DomainLift{ActionID: d.ID, LiftedAt: e.clock.Now()})
		})
		if dup := i == 1; dup != errors.Is(err, appmoderation.ErrAlreadyRecorded) {
			t.Errorf("domain lift #%d = %v; want ErrAlreadyRecorded only for the second", i+1, err)
		}
		err = e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
			return e.h.Moderation.InsertActionLift(ctx, tx, moderation.Lift{ActionID: a.ID, LiftedAt: e.clock.Now()})
		})
		if dup := i == 1; dup != errors.Is(err, appmoderation.ErrAlreadyRecorded) {
			t.Errorf("account lift #%d = %v; want ErrAlreadyRecorded only for the second", i+1, err)
		}
	}
	// The stored actions read back as issued.
	err = e.h.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		got, err := e.h.Moderation.DomainAction(ctx, tx, d.ID)
		if err == nil && !reflect.DeepEqual(got, d) {
			t.Errorf("DomainAction = %+v, want %+v", got, d)
		}
		if err != nil {
			return err
		}
		gotA, err := e.h.Moderation.Action(ctx, tx, a.ID)
		if err == nil && !reflect.DeepEqual(gotA, a) {
			t.Errorf("Action = %+v, want %+v", gotA, a)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func moderationEmits(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	a := e.domainAction(t, "bad.example", moderation.DomainLimit, 2*time.Hour)
	e.domainAction(t, "bad.example", moderation.DomainLimit, time.Hour)
	until := e.clock.Now().Add(2 * time.Hour)
	events := e.events.take()
	last, ok := events[len(events)-1].(appmoderation.DomainRestrictionsChanged)
	want := appmoderation.DomainRestrictionsChanged{Domain: "bad.example", Effects: []moderation.Effect{
		{Restriction: moderation.RestrictPublicDiscovery, Until: &until},
		{Restriction: moderation.RestrictUnsolicitedReach, Until: &until},
	}}
	if len(events) != 2 || !ok || !reflect.DeepEqual(last, want) {
		t.Errorf("events = %+v, want 2 ending with %+v", events, want)
	}
	if err := e.svc.LiftDomainAction(ctx, a.ID, ""); err != nil {
		t.Fatal(err)
	}
	soon := e.clock.Now().Add(time.Hour)
	events = e.events.take()
	want.Effects = []moderation.Effect{
		{Restriction: moderation.RestrictPublicDiscovery, Until: &soon},
		{Restriction: moderation.RestrictUnsolicitedReach, Until: &soon},
	}
	if len(events) != 1 || !reflect.DeepEqual(events[0], want) {
		t.Errorf("events after the lift = %+v, want %+v", events, want)
	}

	s, err := e.svc.IssueRemoteAccountAction(ctx, appmoderation.AccountActionInput{
		TargetAccountID: e.remote, Kind: moderation.ActionSuspend, ReasonCategory: moderation.CategoryOther,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.LiftAccountAction(ctx, s.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	events = e.events.take()
	if len(events) != 2 {
		t.Fatalf("account events = %+v, want 2", events)
	}
	if ev, ok := events[0].(appmoderation.AccountRestrictionsChanged); !ok || ev.AccountID != e.remote || len(ev.Effects) != 5 {
		t.Errorf("account event = %+v, want the 5 restrictions of a suspension", events[0])
	}
	if ev, ok := events[1].(appmoderation.AccountRestrictionsChanged); !ok || len(ev.Effects) != 0 {
		t.Errorf("account event after the lift = %+v, want no restrictions", events[1])
	}
}

func moderationReports(t *testing.T, e *moderationEnv) {
	ctx := t.Context()
	in := appmoderation.RemoteReportInput{
		URI: "https://remote.example/flags/1", ReporterAccountID: e.reports, TargetAccountID: e.local,
		StatusIDs: []string{"s2", "s1", "s2"}, Comment: strings.Repeat("あ", moderation.MaxReportComment+10),
	}
	r, created, err := e.svc.FileRemoteReport(ctx, in)
	if err != nil || !created {
		t.Fatalf("FileRemoteReport = %v, %v; want created", created, err)
	}
	if r.Comment != strings.Repeat("あ", moderation.MaxReportComment) || !reflect.DeepEqual(r.StatusIDs, []string{"s1", "s2"}) || r.Category != moderation.CategoryOther {
		t.Errorf("report = %+v, want the comment cut to the limit and the statuses s1 and s2", r)
	}
	again, created, err := e.svc.FileRemoteReport(ctx, in)
	if err != nil || created || again.ID != r.ID {
		t.Errorf("FileRemoteReport again = %s, %v, %v; want the first report", again.ID, created, err)
	}
	got, err := e.svc.Report(ctx, r.ID)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Errorf("Report = %+v, %v; want %+v", got, err, r)
	}
	st, err := e.svc.ReportStatus(ctx, r.ID)
	if err != nil || st.State != moderation.ReportOpen || !st.Since.Equal(r.FiledAt) {
		t.Errorf("ReportStatus = %+v, %v; want open since filing", st, err)
	}
	// Another Flag without statuses is another report.
	second, created, err := e.svc.FileRemoteReport(ctx, appmoderation.RemoteReportInput{
		URI: "https://remote.example/flags/2", ReporterAccountID: e.reports, TargetAccountID: e.local,
	})
	if err != nil || !created || len(second.StatusIDs) != 0 {
		t.Fatalf("FileRemoteReport(second) = %+v, %v, %v", second, created, err)
	}
	all, err := e.svc.ReportsOf(ctx, e.local)
	if err != nil || len(all) != 2 || all[0].ID != r.ID || all[1].ID != second.ID || len(all[1].StatusIDs) != 0 {
		t.Errorf("ReportsOf = %+v, %v; want the two reports in order", all, err)
	}
	if events := e.events.take(); len(events) != 2 {
		t.Errorf("events = %+v, want one ReportFiled per report", events)
	}
	if _, _, err := e.svc.FileRemoteReport(ctx, appmoderation.RemoteReportInput{ReporterAccountID: e.reports, TargetAccountID: e.local}); !errors.Is(err, moderation.ErrInvalidReport) {
		t.Errorf("FileRemoteReport without a URI = %v, want ErrInvalidReport", err)
	}
}
