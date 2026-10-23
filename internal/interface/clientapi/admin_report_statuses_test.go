// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	appcontent "github.com/mycroblog/my/internal/app/content"
	"github.com/mycroblog/my/internal/domain/content"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

func (f *fakeModeration) ReportStatuses(_ context.Context, id string) ([]appcontent.ReviewedStatus, error) {
	if _, err := f.report(id); err != nil {
		return nil, err
	}
	return f.reviewed[id], nil
}

func TestReportDetailShowsTheReportedStatuses(t *testing.T) {
	e, f := newModerationEnv(t)
	deletedAt := adminNow.Add(-time.Hour)
	direct := sampleView(appcontent.Viewer{Kind: appcontent.ViewerAnonymous})
	direct.Status.Visibility = content.VisibilityDirect
	private := sampleView(appcontent.Viewer{Kind: appcontent.ViewerAnonymous})
	private.Status.ID, private.Status.Visibility = "01890a5d-ac96-774b-bcce-b302099a3002", content.VisibilityPrivate
	f.reviewed = map[string][]appcontent.ReviewedStatus{
		reportTwo: {{StatusView: direct}, {StatusView: private, DeletedAt: deletedAt}},
	}

	res := e.sendJSON(t, http.MethodGet, "/admin/reports/"+reportTwo, "admin-read", "")
	got := decodeJSON[clientapi.AdminReportDetail](t, res)
	if res.StatusCode != http.StatusOK || len(got.Statuses) != 2 {
		t.Fatalf("detail = %d %+v, want the two statuses", res.StatusCode, got.Statuses)
	}
	if s := got.Statuses[0]; s.Status.Id != sampleStatusID || s.Status.Visibility != clientapi.VisibilityDirect ||
		s.Status.Content != "<p>hi</p>" || s.DeletedAt != nil || s.Status.Favourited != nil {
		t.Errorf("direct status = %+v, want it shown, not deleted, without the moderator's relation", s)
	}
	if s := got.Statuses[1]; s.Status.Visibility != clientapi.VisibilityPrivate || s.DeletedAt == nil || !s.DeletedAt.Equal(deletedAt) {
		t.Errorf("private status = %+v, want it shown deleted at %v", s, deletedAt)
	}

	// The operations that answer with the detail carry the statuses, and a report without
	// statuses has an empty list.
	res = e.sendJSON(t, http.MethodPost, "/admin/reports/"+reportTwo+"/assign_to_self", "admin", "")
	if got := decodeJSON[clientapi.AdminReportDetail](t, res); res.StatusCode != http.StatusOK || len(got.Statuses) != 2 {
		t.Errorf("assign_to_self = %d %d statuses, want 2", res.StatusCode, len(got.Statuses))
	}
	res = e.sendJSON(t, http.MethodGet, "/admin/reports/"+reportOne, "admin-read", "")
	if got := decodeJSON[clientapi.AdminReportDetail](t, res); res.StatusCode != http.StatusOK || got.Statuses == nil || len(got.Statuses) != 0 {
		t.Errorf("detail without statuses = %d %+v, want an empty list", res.StatusCode, got.Statuses)
	}
	// The list of reports does not carry them.
	res = e.sendJSON(t, http.MethodGet, "/admin/reports", "admin-read", "")
	if body, _ := io.ReadAll(res.Body); strings.Contains(string(body), "<p>hi</p>") {
		t.Errorf("the list of reports shows a status: %s", body)
	}
}
