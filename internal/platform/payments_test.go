// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/payment/fake"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/supporters"
	"github.com/best-friends/my/internal/platform"
)

// syncedSubscriptions is a supporters.Syncer that keeps the snapshots it is given.
type syncedSubscriptions struct {
	mu    sync.Mutex
	snaps []supporters.SubscriptionSnapshot
}

func (s *syncedSubscriptions) SyncSubscription(_ context.Context, _ string, snap supporters.SubscriptionSnapshot) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snaps = append(s.snaps, snap)
	return true, nil
}

func (s *syncedSubscriptions) SyncCheckout(context.Context, string, supporters.CheckoutResult) (bool, error) {
	return false, nil
}

func (s *syncedSubscriptions) RecordMoney(context.Context, string, []supporters.MoneyMovement) error {
	return nil
}

func (s *syncedSubscriptions) KnownMoney(context.Context, string, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (s *syncedSubscriptions) OpenCheckouts(context.Context, string, time.Time) ([]supporters.CheckoutRef, error) {
	return nil, nil
}

func (s *syncedSubscriptions) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.snaps)
}

// TestPaymentWebhooks serves the webhook endpoint of a payment provider on SQLite with the
// inline queue and the fake adapter (ADR-0074 §3): a signed delivery is recorded and its job
// re-reads the subscription, a repeated one is accepted without another job, a forged one is a
// 400, another provider's path a 404, and the reconciliation and the purge are scheduled.
func TestPaymentWebhooks(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{
		"MY_DATABASE_URL": newDatabase(t, true), "MY_WEB_ORIGIN": webOrigin, "MY_QUEUE": "inline",
		"MY_PAYMENTS_RECONCILE_INTERVAL": "1h",
	})
	prov := fake.New(fake.Options{Name: "stripe"})
	prov.PutSubscription(supporters.SubscriptionSnapshot{Ref: "sub_1", LocalID: "local-1", Status: supporters.StatusActive})
	synced := &syncedSubscriptions{}
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{
		Logger: slog.New(slog.DiscardHandler), PaymentProvider: prov, SupporterSync: synced,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	for name, every := range map[string]time.Duration{
		supporters.JobPaymentReconcile: time.Hour, supporters.JobPaymentReceiptPurge: supporters.ReceiptPurgeInterval,
	} {
		if !slices.ContainsFunc(app.Scheduler.Schedules(), func(s queue.Schedule) bool { return s.Name == name && s.Every == every }) {
			t.Errorf("no schedule %s every %s", name, every)
		}
	}
	h := app.Handler()
	post := func(provider string, header http.Header, body []byte) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/payments/webhooks/"+provider, bytes.NewReader(body))
		for k, v := range header {
			req.Header[k] = v
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	header, body := prov.Deliver(supporters.Notice{EventID: "evt_1", Kind: supporters.NoticeSubscription, ObjectID: "sub_1"}, false, time.Now())
	if code := post("stripe", header, body); code != http.StatusOK {
		t.Fatalf("a signed delivery = %d, want 200", code)
	}
	if code := post("stripe", header, body); code != http.StatusOK {
		t.Errorf("the same delivery again = %d, want 200", code)
	}
	if n := synced.count(); n != 1 {
		t.Errorf("the deliveries synced the subscription %d times, want once", n)
	}
	if code := post("stripe", header, append(bytes.Clone(body), ' ')); code != http.StatusBadRequest {
		t.Errorf("a tampered delivery = %d, want 400", code)
	}
	if code := post("other", header, body); code != http.StatusNotFound {
		t.Errorf("a delivery to another provider = %d, want 404", code)
	}
	st, err := app.Payments.Status(t.Context())
	if err != nil || st.LastWebhook == nil || st.LastWebhook.EventID != "evt_1" {
		t.Errorf("Status = %+v, %v, want evt_1 as the last webhook", st, err)
	}
}

// Without payments_provider, the webhook path is not served.
func TestPaymentWebhooksOff(t *testing.T) {
	t.Parallel()
	cfg := loadConfig(t, map[string]string{"MY_DATABASE_URL": newDatabase(t, true), "MY_WEB_ORIGIN": webOrigin, "MY_QUEUE": "inline"})
	app, err := platform.NewApp(t.Context(), cfg, platform.AppOptions{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	if app.Payments != nil {
		t.Error("payments_provider = none built the payments")
	}
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/payments/webhooks/stripe", bytes.NewReader([]byte("{}"))))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /payments/webhooks/stripe without a provider = %d, want 404", rec.Code)
	}
}
