// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"io"
	"net/http"
	"slices"
	"testing"

	appmoderation "github.com/mycroblog/my/internal/app/moderation"
	"github.com/mycroblog/my/internal/domain/moderation"
	"github.com/mycroblog/my/internal/interface/clientapi"
	"github.com/mycroblog/my/internal/platform"
)

// TestDomainBlockCSVThroughTheAPI drives the domain block list in Mastodon's CSV on the real
// database (FR-706): the preview says what each row of a Mastodon export would issue, against
// the actions in effect on the domain and its parents; the import issues the chosen rows'
// actions, each audited; and the export gives back what Mastodon imports.
func TestDomainBlockCSVThroughTheAPI(t *testing.T) {
	t.Parallel()
	c := newStatusesClientWith(t, nil, platform.AppOptions{}, "carol")
	ctx := t.Context()
	if _, err := c.app.Moderation.IssueDomainAction(ctx, appmoderation.DomainActionInput{Domain: "noisy.example", Kind: moderation.DomainLimit}); err != nil {
		t.Fatal(err)
	}
	csv := "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\n" +
		"spam.example,suspend,false,true,Spam,true\n" +
		"noisy.example,silence,true,false,,false\n" +
		"sub.noisy.example,silence,false,false,,false\n" +
		"例え.jp,noop,true,false,,false\n" +
		"calm.example,noop,false,false,,false\n" +
		"SPAM.example.,silence,false,false,,false\n" +
		"bad domain,suspend,false,false,,false\n" +
		"odd.example,ban,false,false,,false\n"
	var preview clientapi.DomainBlockImportPreview
	if res := c.do("carol", http.MethodPost, "/admin/domain_actions/import/preview", map[string]any{"csv": csv}, &preview); res.StatusCode != http.StatusOK {
		t.Fatalf("preview = %d", res.StatusCode)
	}
	type row struct {
		status clientapi.DomainBlockImportStatus
		issue  []clientapi.AdminDomainActionKind
	}
	want := []row{
		{clientapi.DomainBlockImportStatusImportable, []clientapi.AdminDomainActionKind{"suspend", "reject_reports"}},
		{clientapi.DomainBlockImportStatusImportable, []clientapi.AdminDomainActionKind{"reject_media"}},
		{clientapi.DomainBlockImportStatusAlreadyApplied, []clientapi.AdminDomainActionKind{}},
		{clientapi.DomainBlockImportStatusImportable, []clientapi.AdminDomainActionKind{"reject_media"}},
		{clientapi.DomainBlockImportStatusNoAction, []clientapi.AdminDomainActionKind{}},
		{clientapi.DomainBlockImportStatusDuplicate, []clientapi.AdminDomainActionKind{}},
		{clientapi.DomainBlockImportStatusInvalid, []clientapi.AdminDomainActionKind{}},
		{clientapi.DomainBlockImportStatusInvalid, []clientapi.AdminDomainActionKind{}},
	}
	if len(preview.Rows) != len(want) {
		t.Fatalf("preview = %+v", preview.Rows)
	}
	var chosen []clientapi.DomainBlock
	for i, r := range preview.Rows {
		if r.Line != i+2 || r.Status != want[i].status || !slices.Equal(r.Issue, want[i].issue) {
			t.Errorf("row %d = %+v, want %+v", i+1, r, want[i])
		}
		if r.Status == clientapi.DomainBlockImportStatusImportable {
			chosen = append(chosen, *r.Block)
		}
	}
	if b := preview.Rows[3].Block; b == nil || b.Domain != "xn--r8jz45g.jp" || preview.Rows[3].WrittenDomain != "例え.jp" {
		t.Errorf("IDN row = %+v", preview.Rows[3])
	}

	var res clientapi.DomainBlockImportResult
	body := map[string]any{"blocks": chosen, "private_note": "from mastodon.example"}
	if r := c.do("carol", http.MethodPost, "/admin/domain_actions/import", body, &res); r.StatusCode != http.StatusOK {
		t.Fatalf("import = %d", r.StatusCode)
	}
	if len(res.Issued) != 4 || len(res.Skipped) != 0 || res.Issued[0].Domain != "spam.example" || res.Issued[0].PrivateNote == nil ||
		!res.Issued[0].Obfuscate || res.Issued[0].PublicComment == nil || *res.Issued[0].PublicComment != "Spam" {
		t.Errorf("import = %+v", res)
	}
	audit, err := c.app.Moderation.AuditLog(ctx, appmoderation.AuditQuery{TargetKind: moderation.AuditTargetDomain}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) != 5 || audit[0].Action != moderation.AuditDomainActionIssued || audit[0].ActorUserID == "" {
		t.Errorf("audit log = %+v, want the first action and four by carol", audit)
	}
	// The same blocks again issue nothing.
	if r := c.do("carol", http.MethodPost, "/admin/domain_actions/import", body, &res); r.StatusCode != http.StatusOK || len(res.Issued) != 0 ||
		len(res.Skipped) != 3 || res.Skipped[0].Reason != clientapi.DomainBlockImportStatusAlreadyApplied {
		t.Errorf("import again = %d %+v, want everything already applied", r.StatusCode, res)
	}

	r := c.do("carol", http.MethodGet, "/admin/domain_actions/export", nil, nil)
	got, _ := io.ReadAll(r.Body)
	wantCSV := "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\n" +
		"noisy.example,silence,true,false,,false\n" +
		"spam.example,suspend,false,true,Spam,true\n" +
		"xn--r8jz45g.jp,noop,true,false,,false\n"
	if r.StatusCode != http.StatusOK || string(got) != wantCSV {
		t.Errorf("export = %d\n%s\nwant\n%s", r.StatusCode, got, wantCSV)
	}
}
