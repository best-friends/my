// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/domain/notification"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeNotifications keeps notifications in memory with the semantics of
// appnotification.Service: newest first, dismissals, and a forward-only read marker.
type fakeNotifications struct {
	ns        []notification.Notification // ascending by ID
	dismissed map[string]bool
	markers   map[string]string
	queries   []appnotification.Query
	err       error
}

func (f *fakeNotifications) visible(accountID string) []notification.Notification {
	var out []notification.Notification
	for _, n := range slices.Backward(f.ns) {
		if n.RecipientAccountID == accountID && !f.dismissed[n.ID] {
			out = append(out, n)
		}
	}
	return out
}

func (f *fakeNotifications) List(_ context.Context, accountID string, q appnotification.Query) ([]notification.Notification, error) {
	f.queries = append(f.queries, q)
	if f.err != nil {
		return nil, f.err
	}
	for _, t := range q.Types {
		if !t.Valid() {
			return nil, notification.ErrInvalidType
		}
	}
	var out []notification.Notification
	for _, n := range f.visible(accountID) {
		if (q.Before == "" || n.ID < q.Before) && (len(q.Types) == 0 || slices.Contains(q.Types, n.Type)) && len(out) < q.Limit {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeNotifications) ReadMarker(_ context.Context, accountID string) (notification.ReadMarker, error) {
	return notification.ReadMarker{AccountID: accountID, LastReadID: f.markers[accountID]}, nil
}

func (f *fakeNotifications) UnreadCount(_ context.Context, accountID string) (int64, error) {
	var n int64
	for _, x := range f.visible(accountID) {
		if x.ID > f.markers[accountID] {
			n++
		}
	}
	return n, nil
}

func (f *fakeNotifications) own(accountID, id string) error {
	for _, n := range f.ns {
		if n.ID == id && n.RecipientAccountID == accountID {
			return nil
		}
	}
	return appnotification.ErrNotFound
}

func (f *fakeNotifications) MarkRead(ctx context.Context, accountID, lastReadID string) (notification.ReadMarker, error) {
	if err := f.own(accountID, lastReadID); err != nil {
		return notification.ReadMarker{}, err
	}
	if lastReadID > f.markers[accountID] {
		f.markers[accountID] = lastReadID
	}
	return f.ReadMarker(ctx, accountID)
}

func (f *fakeNotifications) Dismiss(_ context.Context, accountID, id string) error {
	if err := f.own(accountID, id); err != nil {
		return err
	}
	f.dismissed[id] = true
	return nil
}

const (
	bobAccountID = "01890a5d-ac96-774b-bcce-b302099a8058"
	statusID     = "01890a5d-ac96-774b-bcce-b302099a9000"
)

// notificationID returns the ID of the i-th notification; IDs grow with i.
func notificationID(i int) string { return fmt.Sprintf("01890a5d-ac96-774b-bcce-b302099a%04d", i) }

func newNotificationEnv(t *testing.T) (*authEnv, *fakeNotifications) {
	t.Helper()
	at := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	f := &fakeNotifications{dismissed: map[string]bool{}, markers: map[string]string{}}
	types := []notification.Type{notification.TypeFollow, notification.TypeMention, notification.TypeFavourite, notification.TypeModerationNotice}
	for i, typ := range types {
		n := notification.Notification{
			ID: notificationID(i + 1), RecipientAccountID: aliceAccountID, Type: typ, SourceAccountID: bobAccountID,
			CreatedAt: at.Add(time.Duration(i) * time.Minute),
		}
		switch typ {
		case notification.TypeMention, notification.TypeFavourite:
			n.StatusID = statusID
		case notification.TypeModerationNotice:
			n.SourceAccountID = ""
		}
		f.ns = append(f.ns, n)
	}
	// A notification of bob, which alice may neither see nor act on.
	f.ns = append(f.ns, notification.Notification{
		ID: notificationID(9), RecipientAccountID: bobAccountID, Type: notification.TypeFollow, SourceAccountID: aliceAccountID, CreatedAt: at,
	})
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Notifications = f
		o.Accounts.(fakeAccounts)[bobAccountID] = o.Accounts.(fakeAccounts)[aliceAccountID]
	})
	return e, f
}

func (e *authEnv) send(t *testing.T, method, path, token, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, clientapi.BasePath+path, strings.NewReader(body))
	req.Header.Set("Accept-Language", "en")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.spec.Serve(t, e.h, req)
}

func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func ids(page clientapi.NotificationPage) []string {
	out := []string{}
	for _, n := range page.Items {
		out = append(out, n.Id)
	}
	return out
}

func TestListNotifications(t *testing.T) {
	e, f := newNotificationEnv(t)

	res := e.send(t, http.MethodGet, "/notifications", "read", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	page := decode[clientapi.NotificationPage](t, res)
	if got, want := ids(page), []string{notificationID(4), notificationID(3), notificationID(2), notificationID(1)}; !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
	if page.NextCursor != nil {
		t.Errorf("next_cursor = %q on the last page", *page.NextCursor)
	}
	if got := f.queries[0]; got.Limit != 21 || got.Before != "" || got.Types != nil {
		t.Errorf("query = %+v, want the default limit and one more", got)
	}
	notice, fav := page.Items[0], page.Items[1]
	if notice.Type != "moderation_notice" || notice.Account != nil || notice.StatusId != nil {
		t.Errorf("moderation notice = %+v, want neither account nor status", notice)
	}
	if fav.Type != "favourite" || fav.Account == nil || fav.Account.Username != "alice" || fav.StatusId == nil || *fav.StatusId != statusID ||
		!fav.CreatedAt.Equal(time.Date(2026, 10, 15, 0, 2, 0, 0, time.UTC)) {
		t.Errorf("favourite = %+v", fav)
	}

	// Pages follow next_cursor until the last one, which has none.
	var got []string
	cursor := ""
	for range 3 {
		path := "/notifications?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		res := e.send(t, http.MethodGet, path, "read", "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s = %d", path, res.StatusCode)
		}
		page := decode[clientapi.NotificationPage](t, res)
		got = append(got, ids(page)...)
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if want := []string{notificationID(4), notificationID(3), notificationID(2), notificationID(1)}; !slices.Equal(got, want) {
		t.Errorf("paged ids = %v, want %v", got, want)
	}

	// types keeps only the given types.
	res = e.send(t, http.MethodGet, "/notifications?types=follow&types=mention", "read", "")
	if page := decode[clientapi.NotificationPage](t, res); !slices.Equal(ids(page), []string{notificationID(2), notificationID(1)}) {
		t.Errorf("filtered ids = %v", ids(page))
	}
}

func TestListNotificationsProblems(t *testing.T) {
	e, f := newNotificationEnv(t)
	for _, path := range []string{"/notifications?types=poke", "/notifications?limit=0", "/notifications?limit=41", "/notifications?limit=x"} {
		res := e.send(t, http.MethodGet, path, "read", "")
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", path, res.StatusCode)
		}
		// alice's locale is ja.
		if p := decodeProblem(t, res); p.Detail == nil || !strings.Contains(*p.Detail, "パラメータ") || res.Header.Get("Content-Language") != "ja" {
			t.Errorf("%s: problem = %+v (detail %v)", path, p, p.Detail)
		}
	}
	if res := e.send(t, http.MethodGet, "/notifications", "read-accounts", ""); res.StatusCode != http.StatusForbidden {
		t.Errorf("without read:notifications = %d, want 403", res.StatusCode)
	}
	f.err = errors.New("database down")
	if res := e.send(t, http.MethodGet, "/notifications", "read", ""); res.StatusCode != http.StatusInternalServerError {
		t.Errorf("failing list = %d, want 500", res.StatusCode)
	}
}

func TestNotificationMarker(t *testing.T) {
	e, _ := newNotificationEnv(t)
	marker := func(res *http.Response) clientapi.NotificationMarker {
		t.Helper()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.StatusCode)
		}
		if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}
		return decode[clientapi.NotificationMarker](t, res)
	}

	if m := marker(e.send(t, http.MethodGet, "/notifications/marker", "read", "")); m.LastReadId != nil || m.UnreadCount != 4 {
		t.Errorf("initial marker = %+v, want none read and 4 unread", m)
	}
	body := func(id string) string { return `{"last_read_id":"` + id + `"}` }
	if m := marker(e.send(t, http.MethodPut, "/notifications/marker", "write", body(notificationID(3)))); m.LastReadId == nil || *m.LastReadId != notificationID(3) || m.UnreadCount != 1 {
		t.Errorf("marker after reading 3 = %+v", m)
	}
	// The marker never moves back.
	if m := marker(e.send(t, http.MethodPut, "/notifications/marker", "write", body(notificationID(1)))); *m.LastReadId != notificationID(3) || m.UnreadCount != 1 {
		t.Errorf("marker after reading 1 = %+v, want it still at 3", m)
	}
	if m := marker(e.send(t, http.MethodGet, "/notifications/marker", "read", "")); *m.LastReadId != notificationID(3) {
		t.Errorf("marker = %+v", m)
	}

	// Another account's notification, or none at all, is not found.
	for _, id := range []string{notificationID(9), notificationID(7)} {
		res := e.send(t, http.MethodPut, "/notifications/marker", "write", body(id))
		if res.StatusCode != http.StatusNotFound || res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("marking %s = %d, want 404", id, res.StatusCode)
		}
	}
	if res := e.send(t, http.MethodPut, "/notifications/marker", "write", "{"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed body = %d, want 400", res.StatusCode)
	}
	if res := e.send(t, http.MethodPut, "/notifications/marker", "read", body(notificationID(3))); res.StatusCode != http.StatusForbidden {
		t.Errorf("without write:notifications = %d, want 403", res.StatusCode)
	}
}

func TestDismissNotification(t *testing.T) {
	e, _ := newNotificationEnv(t)
	for range 2 { // dismissing again changes nothing
		if res := e.send(t, http.MethodPost, "/notifications/"+notificationID(2)+"/dismiss", "write", ""); res.StatusCode != http.StatusNoContent {
			t.Fatalf("dismiss = %d, want 204", res.StatusCode)
		}
	}
	page := decode[clientapi.NotificationPage](t, e.send(t, http.MethodGet, "/notifications", "read", ""))
	if want := []string{notificationID(4), notificationID(3), notificationID(1)}; !slices.Equal(ids(page), want) {
		t.Errorf("ids after dismissing 2 = %v, want %v", ids(page), want)
	}
	if m := decode[clientapi.NotificationMarker](t, e.send(t, http.MethodGet, "/notifications/marker", "read", "")); m.UnreadCount != 3 {
		t.Errorf("unread count = %d, want 3", m.UnreadCount)
	}

	res := e.send(t, http.MethodPost, "/notifications/"+notificationID(9)+"/dismiss", "write", "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("dismissing bob's notification = %d, want 404", res.StatusCode)
	}
	if p := decodeProblem(t, res); p.Type != clientapi.ProblemTypeBlank || p.Status != http.StatusNotFound {
		t.Errorf("problem = %+v", p)
	}
}
