// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/blob"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeOwnAccount deletes accounts whose password is "secret".
type fakeOwnAccount struct{ deleted []string }

func (f *fakeOwnAccount) DeleteOwnAccount(_ context.Context, userID, password string) error {
	if password != "secret" {
		return appidentity.ErrWrongPassword
	}
	f.deleted = append(f.deleted, userID)
	return nil
}

// fakeExports keeps the exports in memory, newest first; a ready export's archive is its ID.
type fakeExports struct {
	now     time.Time
	exports []identity.DataExport
}

func (f *fakeExports) RequestExport(_ context.Context, userID string) (identity.DataExport, error) {
	for _, e := range f.exports {
		if e.UserID == userID && e.Result == nil {
			return identity.DataExport{}, appidentity.ErrExportInProgress
		}
	}
	e := identity.DataExport{ID: fmt.Sprintf("01890a5d-ac96-774b-bcce-b302099d%04d", len(f.exports)+1), UserID: userID, RequestedAt: f.now}
	f.exports = append([]identity.DataExport{e}, f.exports...)
	return e, nil
}

func (f *fakeExports) ListExports(_ context.Context, userID string) ([]identity.DataExport, error) {
	var out []identity.DataExport
	for _, e := range f.exports {
		if e.UserID == userID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeExports) OpenExport(_ context.Context, userID, id string) (io.ReadCloser, blob.Info, error) {
	for _, e := range f.exports {
		if e.ID != id || e.UserID != userID {
			continue
		}
		if e.State(f.now) != identity.ExportStateReady {
			return nil, blob.Info{}, appidentity.ErrExportNotReady
		}
		return io.NopCloser(strings.NewReader(e.ID)), blob.Info{Size: int64(len(e.ID))}, nil
	}
	return nil, blob.Info{}, appidentity.ErrNotFound
}

func (f *fakeExports) Now() time.Time { return f.now }

func TestDeleteOwnAccount(t *testing.T) {
	own := &fakeOwnAccount{}
	e := newAuthEnv(t, func(o *clientapi.Options) { o.OwnAccount = own })
	alice := "user-" + aliceAccountID

	res := e.sendJSON(t, http.MethodPost, "/accounts/delete", "write", `{"password":"wrong"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusUnprocessableEntity || p.Type != clientapi.ProblemTypeWrongPassword || res.Header.Get("Content-Language") != "ja" {
		t.Errorf("wrong password = %d %+v, want 422 wrong-password in ja", res.StatusCode, p)
	}
	if res := e.sendJSON(t, http.MethodPost, "/accounts/delete", "read", `{"password":"secret"}`); res.StatusCode != http.StatusForbidden {
		t.Errorf("without write:accounts = %d, want 403", res.StatusCode)
	}
	if len(own.deleted) != 0 {
		t.Fatalf("deleted = %v, want none yet", own.deleted)
	}
	res = e.sendJSON(t, http.MethodPost, "/accounts/delete", "write", `{"password":"secret"}`)
	if res.StatusCode != http.StatusNoContent || len(own.deleted) != 1 || own.deleted[0] != alice {
		t.Errorf("delete = %d, deleted %v; want 204 and %s", res.StatusCode, own.deleted, alice)
	}
	// Past the limit of attempts, the password is not checked.
	for range clientapi.DeleteAccountLimit.Max {
		e.sendJSON(t, http.MethodPost, "/accounts/delete", "write", `{"password":"wrong"}`)
	}
	if res := e.sendJSON(t, http.MethodPost, "/accounts/delete", "write", `{"password":"secret"}`); res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("past the limit = %d, want 429", res.StatusCode)
	}
}

func TestDataExports(t *testing.T) {
	f := &fakeExports{now: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Exports = f })

	res := e.send(t, http.MethodPost, "/exports", "write", "")
	if res.StatusCode != http.StatusAccepted || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("POST /exports = %d, want 202 no-store", res.StatusCode)
	}
	pending := decodeJSON[clientapi.DataExport](t, res)
	if pending.State != clientapi.DataExportStatePending || !pending.RequestedAt.Equal(f.now) || pending.CompletedAt != nil || pending.FileSize != nil {
		t.Errorf("new export = %+v, want pending", pending)
	}
	res = e.send(t, http.MethodPost, "/exports", "write", "")
	if p := decodeProblem(t, res); res.StatusCode != http.StatusConflict || p.Type != clientapi.ProblemTypeExportInProgress {
		t.Errorf("second POST /exports = %d %+v, want 409 export-in-progress", res.StatusCode, p)
	}
	archive := "/exports/" + pending.Id + "/archive"
	res = e.send(t, http.MethodGet, archive, "read-accounts", "")
	if p := decodeProblem(t, res); res.StatusCode != http.StatusConflict || p.Type != clientapi.ProblemTypeExportNotReady {
		t.Errorf("GET archive of a pending export = %d %+v, want 409 export-not-ready", res.StatusCode, p)
	}

	// The build ends.
	f.exports[0].Result = &identity.DataExportResult{
		ExportID: pending.Id, Outcome: identity.ExportReady, BlobKey: "k", FileSize: int64(len(pending.Id)),
		ExpiresAt: f.now.Add(time.Hour), CompletedAt: f.now,
	}
	res = e.send(t, http.MethodGet, "/exports", "read-accounts", "")
	list := decodeJSON[clientapi.DataExportList](t, res)
	if len(list.Items) != 1 || list.Items[0].State != clientapi.DataExportStateReady || list.Items[0].FileSize == nil ||
		*list.Items[0].FileSize != int64(len(pending.Id)) || list.Items[0].ExpiresAt == nil {
		t.Errorf("GET /exports = %+v, want the ready export", list)
	}
	res = e.send(t, http.MethodGet, archive, "read-accounts", "")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/zip" || string(body) != pending.Id ||
		res.Header.Get("Content-Disposition") != `attachment; filename="export-`+pending.Id+`.zip"` {
		t.Errorf("GET archive = %d %q %v, want the archive", res.StatusCode, body, res.Header)
	}
	if res := e.send(t, http.MethodGet, archive, "bob", ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET another user's archive = %d, want 404", res.StatusCode)
	}

	// An expired export is left out of the list.
	f.now = f.now.Add(time.Hour)
	res = e.send(t, http.MethodGet, "/exports", "read-accounts", "")
	if list := decodeJSON[clientapi.DataExportList](t, res); len(list.Items) != 0 {
		t.Errorf("GET /exports after the expiry = %+v, want none", list)
	}
	if res := e.send(t, http.MethodGet, "/exports", "follow-push", ""); res.StatusCode != http.StatusForbidden {
		t.Errorf("GET /exports without read:accounts = %d, want 403", res.StatusCode)
	}
}
