// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"errors"
	"net/http"
	"testing"

	appidentity "github.com/best-friends/my/internal/app/identity"
	appinstance "github.com/best-friends/my/internal/app/instance"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	domainidentity "github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/domain/moderation"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/internal/platform"
)

// TestSignupBlocksThroughTheAPI drives the sign-up block list on the real database (FR-719): the
// blocks that moderators add through the API refuse the sign-ups they match or make them wait for
// a moderator, a lifted or expired block no longer matches, a ban blocks the banned user's email
// address, and every addition and lift is audited.
func TestSignupBlocksThroughTheAPI(t *testing.T) {
	c := newStatusesClientWith(t, nil, platform.AppOptions{}, "carol")
	ctx := t.Context()
	if _, err := c.app.Instance.SetSetting(ctx, appinstance.SetSettingInput{
		Key: instance.SettingRegistrationMode, Value: string(instance.RegistrationInvite),
	}); err != nil {
		t.Fatal(err)
	}
	invite, err := c.app.Identity.CreateInvite(ctx, appidentity.CreateInviteInput{})
	if err != nil {
		t.Fatal(err)
	}
	signUp := func(name, email, ip string) (appidentity.SignedUp, error) {
		return c.app.Identity.SignUp(ctx, appidentity.SignUpInput{
			Username: name, Email: email, Password: "correct horse battery staple", InviteCode: invite.Code, IP: ip,
		})
	}
	add := func(body map[string]any, want int) clientapi.AdminSignupBlock {
		t.Helper()
		var b clientapi.AdminSignupBlock
		if res := c.do("carol", http.MethodPost, "/admin/signup_blocks", body, &b); res.StatusCode != want {
			t.Fatalf("POST /admin/signup_blocks %v = %d, want %d", body, res.StatusCode, want)
		}
		return b
	}

	domain := add(map[string]any{"kind": "email_domain", "value": "例え.jp", "severity": "reject", "comment": "spam wave"}, http.StatusOK)
	if domain.Value != "xn--r8jz45g.jp" || domain.State != clientapi.AdminSignupBlockStateActive || domain.Comment == nil || *domain.Comment != "spam wave" {
		t.Errorf("domain block = %+v", domain)
	}
	ip := add(map[string]any{"kind": "ip_range", "value": "192.0.2.7/24", "severity": "require_approval", "expires_in": 3600}, http.StatusOK)
	if ip.Value != "192.0.2.0/24" || ip.ExpiresAt == nil {
		t.Errorf("ip block = %+v", ip)
	}
	add(map[string]any{"kind": "ip_range", "value": "not a range", "severity": "reject"}, http.StatusUnprocessableEntity)

	// reject refuses the sign-up and writes nothing; require_approval starts it pending in invite
	// mode, which otherwise approves at once.
	if _, err := signUp("dave", "dave@mx.xn--r8jz45g.jp", ""); !errors.Is(err, appidentity.ErrSignUpRefused) {
		t.Errorf("sign-up from the blocked domain = %v, want ErrSignUpRefused", err)
	}
	pending, err := signUp("erin", "erin@example.com", "192.0.2.99")
	if err != nil || pending.State != domainidentity.RegistrationPending {
		t.Errorf("sign-up from the blocked range = %+v, %v; want pending", pending.State, err)
	}
	approved, err := signUp("frank", "frank@example.com", "198.51.100.1")
	if err != nil || approved.State != domainidentity.RegistrationApproved {
		t.Errorf("sign-up from elsewhere = %+v, %v; want approved", approved.State, err)
	}

	// A lifted block no longer matches; the username of the refused sign-up was never taken.
	var lifted clientapi.AdminSignupBlock
	if res := c.do("carol", http.MethodPost, "/admin/signup_blocks/"+domain.Id+"/lift", nil, &lifted); res.StatusCode != http.StatusOK ||
		lifted.State != clientapi.AdminSignupBlockStateLifted || lifted.LiftedAt == nil {
		t.Errorf("lift = %d %+v, want lifted", res.StatusCode, lifted)
	}
	if _, err := signUp("dave", "dave@mx.xn--r8jz45g.jp", ""); err != nil {
		t.Errorf("sign-up after the lift = %v", err)
	}
	var page clientapi.AdminSignupBlockPage
	if res := c.do("carol", http.MethodGet, "/admin/signup_blocks?active=true", nil, &page); res.StatusCode != http.StatusOK ||
		len(page.Items) != 1 || page.Items[0].Id != ip.Id {
		t.Errorf("active blocks = %d %+v, want the ip block", res.StatusCode, page.Items)
	}
	if res := c.do("carol", http.MethodGet, "/admin/signup_blocks/"+ip.Id, nil, nil); res.StatusCode != http.StatusOK {
		t.Errorf("GET a block = %d", res.StatusCode)
	}
	if res := c.do("carol", http.MethodGet, "/admin/signup_blocks/"+c.ids["carol"], nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET an unknown block = %d, want 404", res.StatusCode)
	}

	// A ban with block_email rejects new sign-ups with the banned user's address, +tags included.
	var action clientapi.AdminAccountAction
	if res := c.do("carol", http.MethodPost, "/admin/accounts/"+approved.AccountID+"/actions",
		map[string]any{"kind": "ban", "reason_category": "spam", "block_email": true}, &action); res.StatusCode != http.StatusOK {
		t.Fatalf("ban with block_email = %d", res.StatusCode)
	}
	if res := c.do("carol", http.MethodPost, "/admin/accounts/"+approved.AccountID+"/actions",
		map[string]any{"kind": "suspend", "block_email": true}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("suspend with block_email = %d, want 422", res.StatusCode)
	}
	if _, err := signUp("frank2", "Frank+again@example.com", ""); !errors.Is(err, appidentity.ErrSignUpRefused) {
		t.Errorf("sign-up with the banned address = %v, want ErrSignUpRefused", err)
	}
	if res := c.do("carol", http.MethodGet, "/admin/signup_blocks?kind=email_hash", nil, &page); res.StatusCode != http.StatusOK ||
		len(page.Items) != 1 || page.Items[0].ModerationActionId == nil || *page.Items[0].ModerationActionId != action.Id ||
		page.Items[0].Value != moderation.EmailHash("frank@example.com") {
		t.Errorf("email_hash blocks = %d %+v, want the ban's", res.StatusCode, page.Items)
	}

	me, err := c.app.Accounts.FindByID(ctx, c.ids["carol"])
	if err != nil {
		t.Fatal(err)
	}
	audit, err := c.app.Moderation.AuditLog(ctx, appmoderation.AuditQuery{TargetKind: moderation.AuditTargetSignupBlock}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []moderation.AuditAction{moderation.AuditSignupBlockAdded, moderation.AuditSignupBlockLifted, moderation.AuditSignupBlockAdded, moderation.AuditSignupBlockAdded}
	if len(audit) != len(want) {
		t.Fatalf("audit log of the blocks = %+v, want %v", audit, want)
	}
	for i, a := range want {
		if audit[i].Action != a || audit[i].ActorUserID != me.Local.UserID {
			t.Errorf("entry %d = %+v, want %s by carol", i, audit[i], a)
		}
	}
	if audit[1].TargetID != domain.Id || audit[1].TargetLabel != "email_domain xn--r8jz45g.jp" {
		t.Errorf("lift entry = %+v", audit[1])
	}
}
