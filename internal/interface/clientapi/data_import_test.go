// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/pkg/testkit"
)

// fakeImports reads files with the domain's parser; each account has one block, and an import
// stays in progress until done is set.
type fakeImports struct {
	now     time.Time
	started []appidentity.ImportInput
	done    bool
}

func (f *fakeImports) WriteExport(_ context.Context, accountID string, kind identity.ImportKind, w io.Writer) error {
	rows := []identity.ExportRow{{Target: "blocked-by-" + accountID + "@remote.example"}}
	return identity.WriteExportCSV(w, kind, rows)
}

func (f *fakeImports) PreviewImport(_ context.Context, in appidentity.ImportInput) (appidentity.ImportPreview, error) {
	entries, skipped, err := identity.ParseImportCSV(in.Kind, in.Text)
	if err != nil {
		return appidentity.ImportPreview{}, err
	}
	p := appidentity.ImportPreview{Entries: entries, Skipped: skipped}
	if in.Mode == identity.ImportOverwrite {
		p.Removals = []appidentity.ImportTarget{{ID: testkit.NamedID("removal")}}
	}
	return p, nil
}

func (f *fakeImports) StartImport(ctx context.Context, in appidentity.ImportInput) (identity.DataImportProgress, error) {
	if len(f.started) > 0 && !f.done {
		return identity.DataImportProgress{}, appidentity.ErrImportInProgress
	}
	p, err := f.PreviewImport(ctx, in)
	if err != nil {
		return identity.DataImportProgress{}, err
	}
	f.started = append(f.started, in)
	f.done = false
	return f.progress(len(p.Entries) + len(p.Removals)), nil
}

func (f *fakeImports) progress(total int) identity.DataImportProgress {
	in := f.started[len(f.started)-1]
	p := identity.DataImportProgress{
		DataImport: identity.DataImport{ID: "01890a5d-ac96-774b-bcce-b302099d0001", UserID: in.UserID, Kind: in.Kind, Mode: in.Mode, CreatedAt: f.now},
		Total:      total,
	}
	if f.done {
		p.Processed, p.Failed = total, 1
	}
	return p
}

func (f *fakeImports) ListImports(_ context.Context, userID string) ([]identity.DataImportProgress, error) {
	if len(f.started) == 0 || f.started[0].UserID != userID {
		return nil, nil
	}
	return []identity.DataImportProgress{f.progress(2)}, nil
}

func TestCSVExports(t *testing.T) {
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Imports = &fakeImports{} })
	res := e.send(t, http.MethodGet, "/csv_exports/blocks", "read-accounts", "")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/csv" || res.Header.Get("Cache-Control") != "no-store" ||
		res.Header.Get("Content-Disposition") != `attachment; filename="blocked_accounts.csv"` ||
		string(body) != "blocked-by-"+aliceAccountID+"@remote.example\n" {
		t.Errorf("GET /csv_exports/blocks = %d %q %v", res.StatusCode, body, res.Header)
	}
	res = e.send(t, http.MethodGet, "/csv_exports/following", "read-accounts", "")
	if body, _ := io.ReadAll(res.Body); !strings.HasPrefix(string(body), "Account address,Show boosts,Notify on new posts,Languages\n") {
		t.Errorf("following = %q, want Mastodon's header", body)
	}
	if res := e.send(t, http.MethodGet, "/csv_exports/lists", "read-accounts", ""); res.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown kind = %d, want 400", res.StatusCode)
	}
	if res := e.send(t, http.MethodGet, "/csv_exports/blocks", "follow-push", ""); res.StatusCode != http.StatusForbidden {
		t.Errorf("without read:accounts = %d, want 403", res.StatusCode)
	}
}

func TestDataImports(t *testing.T) {
	f := &fakeImports{now: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)}
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Imports = f })
	file := `{"kind":"blocks","mode":"overwrite","text":"a@remote.example\nnot an address\n@A@REMOTE.EXAMPLE\n"}`

	res := e.sendJSON(t, http.MethodPost, "/imports/preview", "write", file)
	preview := decodeJSON[clientapi.DataImportPreview](t, res)
	if res.StatusCode != http.StatusOK || preview.Entries != 1 || preview.Removals != 1 || len(preview.Skipped) != 2 ||
		preview.Skipped[0] != (clientapi.DataImportSkipped{Line: 2, Value: "not an address", Reason: clientapi.DataImportSkippedReasonInvalid}) ||
		preview.Skipped[1].Reason != clientapi.DataImportSkippedReasonDuplicate {
		t.Errorf("preview = %d %+v", res.StatusCode, preview)
	}
	if len(f.started) != 0 {
		t.Fatalf("a preview started %+v", f.started)
	}
	for body, want := range map[string]int{
		`{"kind":"blocks","mode":"merge","text":"\"a"}`: http.StatusUnprocessableEntity,
	} {
		if res := e.sendJSON(t, http.MethodPost, "/imports/preview", "write", body); res.StatusCode != want {
			t.Errorf("preview %s = %d, want %d", body, res.StatusCode, want)
		}
	}
	if res := e.sendJSON(t, http.MethodPost, "/imports", "read", file); res.StatusCode != http.StatusForbidden {
		t.Errorf("start without write:accounts = %d, want 403", res.StatusCode)
	}

	res = e.sendJSON(t, http.MethodPost, "/imports", "write", file)
	started := decodeJSON[clientapi.DataImport](t, res)
	if res.StatusCode != http.StatusAccepted || started.State != clientapi.DataImportStateInProgress || started.Total != 2 ||
		started.Kind != clientapi.DataImportKindBlocks || started.Mode != clientapi.DataImportModeOverwrite || !started.CreatedAt.Equal(f.now) {
		t.Errorf("start = %d %+v, want 202 in progress", res.StatusCode, started)
	}
	if len(f.started) != 1 || f.started[0].UserID != userOf(aliceAccountID) || f.started[0].AccountID != aliceAccountID {
		t.Errorf("started = %+v, want alice's import", f.started)
	}
	res = e.sendJSON(t, http.MethodPost, "/imports", "write", file)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusConflict || p.Type != clientapi.ProblemTypeImportInProgress {
		t.Errorf("second start = %d %+v, want 409 import-in-progress", res.StatusCode, p)
	}

	f.done = true
	res = e.send(t, http.MethodGet, "/imports", "read-accounts", "")
	list := decodeJSON[clientapi.DataImportList](t, res)
	if len(list.Items) != 1 || list.Items[0].State != clientapi.DataImportStateFinished || list.Items[0].Processed != 2 || list.Items[0].Failed != 1 {
		t.Errorf("GET /imports = %+v, want the finished import", list)
	}
	if list := decodeJSON[clientapi.DataImportList](t, e.send(t, http.MethodGet, "/imports", "bob", "")); len(list.Items) != 0 {
		t.Errorf("bob's imports = %+v, want none", list)
	}
}
