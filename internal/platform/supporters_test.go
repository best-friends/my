// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/payment/fake"
	"github.com/best-friends/my/internal/app/supporters"
	domain "github.com/best-friends/my/internal/domain/supporters"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/internal/interface/clientapi/apitest"
	"github.com/best-friends/my/internal/platform"
)

// newSupportersEnv is an App on SQLite with the inline queue, email (to a server that nobody
// answers), and the fake payment provider.
func newSupportersEnv(t *testing.T) (*accountsEnv, *fake.Provider) {
	t.Helper()
	cfg := loadConfig(t, map[string]string{
		"MY_DATABASE_URL": newDatabase(t, true), "MY_WEB_ORIGIN": webOrigin, "MY_QUEUE": "inline",
		"MY_MAIL": "smtp", "MY_MAIL_FROM": "noreply@example.test", "MY_SMTP_HOST": "127.0.0.1", "MY_SMTP_PORT": "1",
	})
	prov := fake.New(fake.Options{Name: "stripe"})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler), PaymentProvider: prov})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return &accountsEnv{t: t, app: app, h: app.Handler(), spec: apitest.Load(t)}, prov
}

// TestSupporterProgram drives the supporter program on SQLite (ADR-0072): nothing shows while it
// is disabled; an administrator creates a plan on a role that then nobody grants by hand,
// enables the program, and sees the subscriptions; disabling it is refused while one goes on;
// the deletion of the subscriber's account ends the subscription at once, at the provider too;
// and every administrative step is in the audit log.
func TestSupporterProgram(t *testing.T) {
	t.Parallel()
	e, prov := newSupportersEnv(t)
	ctx := t.Context()
	admin := e.user("alice")

	var inst clientapi.Instance
	e.want(e.do(http.MethodGet, "/instance", "", nil, &inst), http.StatusOK, "GET /instance")
	if inst.SupportersEnabled || inst.CommerceDisclosureUrl != nil {
		t.Errorf("a new instance: supporters_enabled %v, commerce_disclosure_url %v", inst.SupportersEnabled, inst.CommerceDisclosureUrl)
	}
	e.want(e.do(http.MethodGet, "/instance/supporter_plans", "", nil, nil), http.StatusNotFound, "the plans while disabled")
	var prog clientapi.AdminSupporterProgram
	e.want(e.do(http.MethodGet, "/admin/supporter_program", admin.token, nil, &prog), http.StatusOK, "GET /admin/supporter_program")
	if prog.Enabled || !prog.CanEnable || prog.ConfiguredProvider == nil || *prog.ConfiguredProvider != "stripe" ||
		!slices.Contains(prog.Warnings, clientapi.SupporterProgramWarningNoCommerceDisclosure) {
		t.Errorf("the program by default: %+v", prog)
	}

	e.want(e.do(http.MethodPost, "/admin/roles", admin.token, map[string]any{"key": "supporter", "position": 10, "permissions": []string{}}, nil),
		http.StatusOK, "POST /admin/roles")
	plan := map[string]any{
		"role": "moderator", "name": "Supporter", "description": "Thank you", "monthly_price": map[string]string{"amount": "500", "currency": "JPY"},
		"annual_discount_percent": 10, "emoji_limit": 5, "rank": 1, "visibility": "public",
	}
	res := e.do(http.MethodPost, "/admin/supporter_plans", admin.token, plan, nil)
	if typ := e.want(res, http.StatusUnprocessableEntity, "a plan on the moderator role"); typ != clientapi.ProblemTypeInvalidSupporterPlan {
		t.Errorf("problem %s", typ)
	}
	plan["role"] = "supporter"
	var created clientapi.AdminSupporterPlan
	e.want(e.do(http.MethodPost, "/admin/supporter_plans", admin.token, plan, &created), http.StatusOK, "POST /admin/supporter_plans")
	if created.Name != "Supporter" || created.YearlyPrice != (clientapi.Money{Amount: "5400", Currency: "JPY"}) || created.Role.Key != "supporter" {
		t.Errorf("created %+v", created)
	}
	var plans clientapi.AdminSupporterPlanList
	e.want(e.do(http.MethodGet, "/admin/supporter_plans", admin.token, nil, &plans), http.StatusOK, "GET /admin/supporter_plans")
	if len(plans.Items) != 1 || !plans.Items[0].PricesReady {
		t.Fatalf("plans %+v", plans)
	}
	// Nobody grants the plan's role by hand.
	res = e.do(http.MethodPut, "/admin/accounts/"+admin.id+"/roles/supporter", admin.token, nil, nil)
	if typ := e.want(res, http.StatusConflict, "a grant of the plan's role"); typ != clientapi.ProblemTypeRoleManagedByPlan {
		t.Errorf("problem %s", typ)
	}

	e.want(e.do(http.MethodPut, "/admin/supporter_program", admin.token, map[string]any{"enabled": true}, &prog), http.StatusOK, "enable")
	if !prog.Enabled || prog.Provider == nil {
		t.Fatalf("enabled %+v", prog)
	}
	e.want(e.do(http.MethodGet, "/instance", "", nil, &inst), http.StatusOK, "GET /instance")
	var offered clientapi.SupporterPlanList
	e.want(e.do(http.MethodGet, "/instance/supporter_plans", "", nil, &offered), http.StatusOK, "the plans")
	if !inst.SupportersEnabled || len(offered.Items) != 1 || offered.Items[0].MonthlyPrice.Amount != "500" {
		t.Fatalf("enabled: %v, offered %+v", inst.SupportersEnabled, offered)
	}

	// Bob subscribes; the provider completes the checkout.
	bob := e.user("bob")
	subID := subscribeOnFake(t, e.app, prov, bob.id, created.Id, "sub_1")
	var subs clientapi.AdminSupporterSubscriptionPage
	e.want(e.do(http.MethodGet, "/admin/supporter_subscriptions?state=active", admin.token, nil, &subs), http.StatusOK, "the subscriptions")
	if len(subs.Items) != 1 || subs.Items[0].Username == nil || *subs.Items[0].Username != "bob" || subs.Items[0].PeriodEndsAt == nil {
		t.Fatalf("subscriptions %+v", subs)
	}
	if roles := e.roleKeys(admin, bob); !slices.Contains(roles, "supporter") {
		t.Errorf("bob's roles %v", roles)
	}
	res = e.do(http.MethodPut, "/admin/supporter_program", admin.token, map[string]any{"enabled": false}, nil)
	if typ := e.want(res, http.StatusConflict, "disable with a subscription"); typ != clientapi.ProblemTypeLiveSubscriptions {
		t.Errorf("problem %s", typ)
	}

	// Bob deletes his account: the subscription ends at once, at the provider too.
	if err := e.app.Accounts.DeleteBySelf(ctx, bob.id); err != nil {
		t.Fatalf("DeleteBySelf = %v", err)
	}
	if v, err := e.app.Supporters.Subscription(ctx, subID); err != nil || v.Current.State != domain.StateExpired ||
		v.Current.Reason != domain.ReasonAccountClosed {
		t.Fatalf("after the deletion: %+v, %v", v.Current, err)
	}
	if s, _ := prov.Subscription("sub_1"); s.Status != supporters.StatusCanceled {
		t.Errorf("the provider's subscription after the deletion: %s", s.Status)
	}

	e.want(e.do(http.MethodPost, "/admin/supporter_plans/"+created.Id+"/retire", admin.token, nil, nil), http.StatusNoContent, "retire")
	res = e.do(http.MethodPost, "/admin/supporter_plans/"+created.Id+"/retire", admin.token, nil, nil)
	if typ := e.want(res, http.StatusConflict, "retire again"); typ != clientapi.ProblemTypeSupporterPlanRetired {
		t.Errorf("problem %s", typ)
	}
	e.want(e.do(http.MethodPut, "/admin/supporter_program", admin.token, map[string]any{"enabled": false}, &prog), http.StatusOK, "disable")
	e.want(e.do(http.MethodGet, "/instance/supporter_plans", "", nil, nil), http.StatusNotFound, "the plans after disabling")

	var log clientapi.AdminAuditEntryPage
	e.want(e.do(http.MethodGet, "/admin/audit_log?limit=40", admin.token, nil, &log), http.StatusOK, "GET /admin/audit_log")
	var actions []clientapi.AdminAuditAction
	for _, entry := range log.Items {
		if strings.HasPrefix(string(entry.Action), "supporter_") {
			actions = append(actions, entry.Action)
		}
	}
	want := []clientapi.AdminAuditAction{
		clientapi.AdminAuditActionSupporterProgramDisabled, clientapi.AdminAuditActionSupporterPlanRetired,
		clientapi.AdminAuditActionSupporterProgramEnabled, clientapi.AdminAuditActionSupporterPlanCreated,
	}
	if !slices.Equal(actions, want) {
		t.Errorf("audit log %v, want %v", actions, want)
	}
}

// subscribeOnFake subscribes the user of a local account to a plan, completes the checkout at
// the fake provider with its subscription ref, and applies it as the return from the checkout
// does. It returns our subscription.
func subscribeOnFake(t *testing.T, app *platform.App, prov *fake.Provider, accountID, planID string, ref supporters.SubscriptionRef) string {
	t.Helper()
	ctx := t.Context()
	co, err := app.Supporters.StartCheckout(ctx, supporters.CheckoutInput{UserID: userOfAccount(t, app, accountID), PlanID: planID, Interval: domain.IntervalMonth})
	if err != nil {
		t.Fatalf("StartCheckout = %v", err)
	}
	sub, err := app.Supporters.Subscription(ctx, co.SubscriptionID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := app.Supporters.Plan(ctx, planID)
	if err != nil {
		t.Fatal(err)
	}
	customer := supporters.CustomerRef("cus_" + string(ref))
	prov.PutCheckout(supporters.CheckoutResult{Ref: sub.Checkout.Ref, Status: supporters.CheckoutCompleted, LocalID: co.SubscriptionID, Customer: customer, Subscription: ref})
	prov.PutSubscription(supporters.SubscriptionSnapshot{
		Ref: ref, Customer: customer, LocalID: co.SubscriptionID, Price: p.Prices[domain.IntervalMonth].Ref, Status: supporters.StatusActive,
		PeriodStart: time.Now(), PeriodEnd: time.Now().Add(30 * 24 * time.Hour),
	})
	if r := app.Supporters.Sync(ctx, co.SubscriptionID); !r.IsDone() {
		t.Fatalf("Sync = %v", r.Err())
	}
	return co.SubscriptionID
}

// TestSupporterSuspension cancels the subscription of a suspended account at the end of its
// period, keeping its role until then (ADR-0072 §10, S1).
func TestSupporterSuspension(t *testing.T) {
	t.Parallel()
	e, prov := newSupportersEnv(t)
	ctx := t.Context()
	admin := e.user("alice")
	e.want(e.do(http.MethodPost, "/admin/roles", admin.token, map[string]any{"key": "supporter", "position": 10, "permissions": []string{}}, nil),
		http.StatusOK, "POST /admin/roles")
	var plan clientapi.AdminSupporterPlan
	e.want(e.do(http.MethodPost, "/admin/supporter_plans", admin.token, map[string]any{
		"role": "supporter", "name": "Supporter", "monthly_price": map[string]string{"amount": "500", "currency": "JPY"},
		"annual_discount_percent": 0, "emoji_limit": 0, "rank": 1, "visibility": "public",
	}, &plan), http.StatusOK, "POST /admin/supporter_plans")
	e.want(e.do(http.MethodPut, "/admin/supporter_program", admin.token, map[string]any{"enabled": true}, nil), http.StatusOK, "enable")
	carol := e.user("carol")
	subID := subscribeOnFake(t, e.app, prov, carol.id, plan.Id, "sub_9")
	e.want(e.do(http.MethodPost, "/admin/accounts/"+carol.id+"/actions", admin.token, map[string]any{"kind": "suspend", "public_reason": "rules"}, nil),
		http.StatusOK, "suspend")
	v, err := e.app.Supporters.Subscription(ctx, subID)
	if err != nil || v.Current.State != domain.StateCanceling || v.Current.Reason != domain.ReasonAccountSuspended {
		t.Fatalf("after the suspension: %+v, %v", v.Current, err)
	}
	if s, _ := prov.Subscription("sub_9"); !s.CancelAtPeriodEnd || s.Status != supporters.StatusActive {
		t.Errorf("the provider's subscription: %+v", s)
	}
	if roles := e.roleKeys(admin, carol); !slices.Contains(roles, "supporter") {
		t.Errorf("carol's roles %v", roles)
	}
}

// userOfAccount returns the user of a local account.
func userOfAccount(t *testing.T, app *platform.App, accountID string) string {
	t.Helper()
	a, err := app.Accounts.FindByID(t.Context(), accountID)
	if err != nil || a.Local == nil {
		t.Fatalf("account %s: %v", accountID, err)
	}
	return a.Local.UserID
}

// roleKeys returns the roles of a local account as an administrator reads them.
func (e *accountsEnv) roleKeys(admin, of testUser) []string {
	e.t.Helper()
	var list clientapi.AdminRoleList
	e.want(e.do(http.MethodGet, "/admin/accounts/"+of.id+"/roles", admin.token, nil, &list), http.StatusOK, "GET roles")
	var out []string
	for _, r := range list.Items {
		out = append(out, r.Key)
	}
	return out
}

// TestCommerceDisclosurePage serves the commerce disclosures at /about/commerce once an
// administrator writes them, and links to them from GET /instance (ADR-0072 §13).
func TestCommerceDisclosurePage(t *testing.T) {
	t.Parallel()
	e, _ := newSupportersEnv(t)
	admin := e.user("alice")
	page := func() (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/about/commerce", nil)
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		b, _ := io.ReadAll(rec.Body)
		return rec.Code, string(b)
	}
	if code, _ := page(); code != http.StatusNotFound {
		t.Errorf("without disclosures: %d", code)
	}
	e.want(e.do(http.MethodPut, "/admin/instance_texts/commerce_disclosure", admin.token, map[string]any{"body": "# Seller\n\nExample Inc."}, nil),
		http.StatusOK, "PUT commerce_disclosure")
	code, body := page()
	if code != http.StatusOK || !strings.Contains(body, "Example Inc.") {
		t.Errorf("with disclosures: %d %q", code, body)
	}
	var inst clientapi.Instance
	e.want(e.do(http.MethodGet, "/instance", "", nil, &inst), http.StatusOK, "GET /instance")
	if inst.CommerceDisclosureUrl == nil || !strings.HasSuffix(*inst.CommerceDisclosureUrl, "/about/commerce") {
		t.Errorf("commerce_disclosure_url %v", inst.CommerceDisclosureUrl)
	}
}
