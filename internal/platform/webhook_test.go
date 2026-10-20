// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/identity"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/webhook"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/internal/interface/clientapi/apitest"
	"github.com/best-friends/my/internal/platform"
)

// webhookReceiver is an endpoint on loopback that answers with the next queued status code
// (204 when none) and keeps what it receives.
type webhookReceiver struct {
	mu       sync.Mutex
	statuses []int
	got      []*http.Request
	bodies   [][]byte
}

func (r *webhookReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got, r.bodies = append(r.got, req), append(r.bodies, body)
	status := http.StatusNoContent
	if len(r.statuses) > 0 {
		status, r.statuses = r.statuses[0], r.statuses[1:]
	}
	w.WriteHeader(status)
}

func (r *webhookReceiver) take() ([]*http.Request, [][]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	got, bodies := r.got, r.bodies
	r.got, r.bodies = nil, nil
	return got, bodies
}

func (r *webhookReceiver) answer(statuses ...int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses = statuses
}

// TestWebhooks sets up a webhook through the admin API on a receiver on loopback, which
// webhook_allowed_destinations lets the webhooks alone reach, and checks through the composition
// root on SQLite with the inline queue that a public local post, and not an unlisted one,
// reaches it signed with Standard Webhooks v1 (ADR-0060), that the test message and a
// redelivery keep their webhook-id, that a failure is listed with the delivery, that a 410 Gone
// disables the webhook as the system with its audit log entry and the admin_webhook_disabled
// notification, and that the operations need manage_webhooks.
func TestWebhooks(t *testing.T) {
	t.Parallel()
	recv := &webhookReceiver{}
	srv := httptest.NewServer(recv)
	t.Cleanup(srv.Close)
	cfg := loadConfig(t, map[string]string{
		"MY_DATABASE_URL": newDatabase(t, true), "MY_WEB_ORIGIN": webOrigin, "MY_QUEUE": "inline",
		"MY_WEBHOOK_ALLOWED_DESTINATIONS": "127.0.0.1",
	})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	h, spec, ctx := app.Handler(), apitest.Load(t), t.Context()
	admin, err := app.Identity.CreateAdmin(ctx, identity.CreateAdminInput{
		Username: "alice", Email: "alice@example.com", Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	adminToken := issueAccessToken(t, app, admin.User.ID, "read write admin")
	bob, err := app.Identity.RegisterLocalUser(ctx, identity.RegisterInput{Username: "bob", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatal(err)
	}
	bobToken := issueAccessToken(t, app, bob.User.ID, "read write admin")

	api := func(token, method, path string, body any, out any) int {
		t.Helper()
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, clientapi.BasePath+path, r)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res := spec.Serve(t, h, req)
		if out != nil {
			if err := json.NewDecoder(res.Body).Decode(out); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
		}
		return res.StatusCode
	}

	// Without manage_webhooks, nothing.
	if code := api(bobToken, http.MethodGet, "/admin/webhooks", nil, nil); code != http.StatusForbidden {
		t.Errorf("GET /admin/webhooks without manage_webhooks = %d, want 403", code)
	}
	var problem clientapi.WebhookProblem
	if code := api(adminToken, http.MethodPost, "/admin/webhooks", map[string]any{
		"url": "ftp://hooks.example/", "events": []string{"status.created"},
	}, &problem); code != http.StatusUnprocessableEntity || problem.Reason == nil || *problem.Reason != clientapi.WebhookProblemReasonUrl {
		t.Errorf("POST an ftp URL = %d %+v", code, problem)
	}
	var created clientapi.AdminWebhookCreated
	if code := api(adminToken, http.MethodPost, "/admin/webhooks", map[string]any{
		"url": srv.URL + "/hook?token=s3cret", "description": "Moderation chat", "events": []string{"status.created", "report.created"},
	}, &created); code != http.StatusOK {
		t.Fatalf("POST /admin/webhooks = %d", code)
	}
	hook := created.Webhook
	secret, err := webhook.ParseSecret(created.Secret)
	if err != nil || hook.State != clientapi.AdminWebhookStateEnabled || len(hook.Events) != 2 {
		t.Fatalf("created = %+v, %v", created, err)
	}
	verify := func(req *http.Request, body []byte, wantType string) string {
		t.Helper()
		id, ts := req.Header.Get(webhook.HeaderID), req.Header.Get(webhook.HeaderTimestamp)
		sec, err := strconv.ParseInt(ts, 10, 64)
		if err != nil || !webhook.Verify(secret, req.Header.Get(webhook.HeaderSignature), id, time.Unix(sec, 0), body) {
			t.Errorf("the signature of %s does not verify: %v", body, req.Header)
		}
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(body, &msg); err != nil || msg.Type != wantType {
			t.Errorf("body %s, want type %s", body, wantType)
		}
		return id
	}

	// A public local post is sent; an unlisted one is not.
	if _, err := app.Content.Create(ctx, appcontent.CreateInput{
		AuthorAccountID: admin.AccountID, TextInput: appcontent.TextInput{Text: "hidden"}, Visibility: domcontent.VisibilityUnlisted,
	}); err != nil {
		t.Fatal(err)
	}
	post, err := app.Content.Create(ctx, appcontent.CreateInput{
		AuthorAccountID: admin.AccountID, TextInput: appcontent.TextInput{Text: "hello webhooks"}, Visibility: domcontent.VisibilityPublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	reqs, bodies := recv.take()
	if len(reqs) != 1 || reqs[0].URL.Path != "/hook" || reqs[0].URL.Query().Get("token") != "s3cret" {
		t.Fatalf("received %d requests: %v", len(reqs), reqs)
	}
	verify(reqs[0], bodies[0], "status.created")
	if !bytes.Contains(bodies[0], []byte(post.Status.ID)) || !bytes.Contains(bodies[0], []byte("hello webhooks")) {
		t.Errorf("status.created body = %s", bodies[0])
	}

	// The test message fails, is listed as failing, and a redelivery sends it again.
	recv.answer(http.StatusInternalServerError)
	var delivery clientapi.AdminWebhookDelivery
	if code := api(adminToken, http.MethodPost, "/admin/webhooks/"+hook.Id+"/test", nil, &delivery); code != http.StatusOK ||
		delivery.EventType != clientapi.WebhookMessageTypeWebhookTest {
		t.Fatalf("POST test = %d %+v", code, delivery)
	}
	reqs, bodies = recv.take()
	if len(reqs) != 1 {
		t.Fatalf("test requests = %d", len(reqs))
	}
	testID := verify(reqs[0], bodies[0], "webhook.test")
	var page clientapi.AdminWebhookDeliveryPage
	if code := api(adminToken, http.MethodGet, "/admin/webhooks/"+hook.Id+"/deliveries", nil, &page); code != http.StatusOK || len(page.Items) != 2 ||
		page.Items[0].Status != clientapi.AdminWebhookDeliveryStatusFailing || page.Items[0].LastAttempt == nil ||
		page.Items[0].LastAttempt.StatusCode == nil || *page.Items[0].LastAttempt.StatusCode != http.StatusInternalServerError ||
		page.Items[1].Status != clientapi.AdminWebhookDeliveryStatusSucceeded {
		t.Fatalf("GET deliveries = %d %+v", code, page)
	}
	var got clientapi.AdminWebhook
	if code := api(adminToken, http.MethodGet, "/admin/webhooks/"+hook.Id, nil, &got); code != http.StatusOK || got.Failure == nil || got.Failure.ConsecutiveFailures != 1 {
		t.Errorf("GET the failing webhook = %d %+v", code, got)
	}
	if code := api(adminToken, http.MethodPost, "/admin/webhooks/"+hook.Id+"/deliveries/"+delivery.Id+"/redeliver", nil, nil); code != http.StatusOK {
		t.Fatalf("POST redeliver = %d", code)
	}
	reqs, bodies = recv.take()
	if len(reqs) != 1 || verify(reqs[0], bodies[0], "webhook.test") != testID {
		t.Errorf("redelivery = %d requests, want the same webhook-id %s", len(reqs), testID)
	}

	// 410 Gone disables the webhook as the system, and tells the administrators.
	recv.answer(http.StatusGone)
	if code := api(adminToken, http.MethodPost, "/admin/webhooks/"+hook.Id+"/test", nil, nil); code != http.StatusOK {
		t.Fatalf("POST test = %d", code)
	}
	recv.take()
	if code := api(adminToken, http.MethodGet, "/admin/webhooks/"+hook.Id, nil, &got); code != http.StatusOK ||
		got.State != clientapi.AdminWebhookStateDisabled || got.StateReason != clientapi.AdminWebhookStateReasonGone {
		t.Fatalf("GET the gone webhook = %d %+v", code, got)
	}
	var notes clientapi.NotificationPage
	if code := api(adminToken, http.MethodGet, "/notifications", nil, &notes); code != http.StatusOK || len(notes.Items) == 0 ||
		notes.Items[0].Type != clientapi.NotificationTypeAdminWebhookDisabled {
		t.Errorf("GET /notifications = %d %+v, want admin_webhook_disabled", code, notes)
	}
	var audit clientapi.AdminAuditEntryPage
	if code := api(adminToken, http.MethodGet, "/admin/audit_log?target_kind=webhook&target_id="+hook.Id, nil, &audit); code != http.StatusOK || len(audit.Items) != 2 ||
		audit.Items[0].Action != clientapi.AdminAuditActionWebhookDisabled || audit.Items[0].ActorKind != clientapi.AdminAuditActorKindSystem ||
		audit.Items[1].Action != clientapi.AdminAuditActionWebhookCreated || audit.Items[1].TargetLabel != "Moderation chat (127.0.0.1)" {
		t.Errorf("GET the audit log = %d %+v", code, audit)
	}
	// A disabled webhook receives nothing.
	if _, err := app.Content.Create(ctx, appcontent.CreateInput{
		AuthorAccountID: admin.AccountID, TextInput: appcontent.TextInput{Text: "again"}, Visibility: domcontent.VisibilityPublic,
	}); err != nil {
		t.Fatal(err)
	}
	if reqs, _ := recv.take(); len(reqs) != 0 {
		t.Errorf("a disabled webhook received %d requests", len(reqs))
	}
	var secretResp clientapi.AdminWebhookSecret
	if code := api(adminToken, http.MethodPost, "/admin/webhooks/"+hook.Id+"/rotate_secret", nil, &secretResp); code != http.StatusOK || secretResp.Secret == created.Secret {
		t.Errorf("POST rotate_secret = %d %+v", code, secretResp)
	}
	if code := api(adminToken, http.MethodDelete, "/admin/webhooks/"+hook.Id, nil, nil); code != http.StatusNoContent {
		t.Errorf("DELETE = %d", code)
	}
	var list clientapi.AdminWebhookList
	if code := api(adminToken, http.MethodGet, "/admin/webhooks", nil, &list); code != http.StatusOK || len(list.Items) != 0 {
		t.Errorf("GET /admin/webhooks after the deletion = %d %+v", code, list)
	}
}
