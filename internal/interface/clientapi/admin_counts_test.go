// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/interface/clientapi"
)

type fakeCounts struct{ reports, registrations int64 }

func (f fakeCounts) CountOpenReports(context.Context) (int64, error) { return f.reports, nil }

func (f fakeCounts) CountPendingRegistrations(context.Context) (int64, error) {
	return f.registrations, nil
}

// TestGetAdminCounts checks that each count needs its own permission and that a count the
// viewer may not see is omitted, not refused.
func TestGetAdminCounts(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	for _, tc := range []struct {
		name  string
		perms []identity.Permission
		want  clientapi.AdminCounts
	}{
		{"both", []identity.Permission{identity.PermManageReports, identity.PermManageAccounts}, clientapi.AdminCounts{OpenReports: n(3), PendingRegistrations: n(0)}},
		{"reports", []identity.Permission{identity.PermManageReports, identity.PermViewAudit}, clientapi.AdminCounts{OpenReports: n(3)}},
		{"registrations", []identity.Permission{identity.PermManageAccounts}, clientapi.AdminCounts{PendingRegistrations: n(0)}},
		{"none", []identity.Permission{identity.PermManageDomains}, clientapi.AdminCounts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newAuthEnv(t, func(o *clientapi.Options) {
				o.Permissions = fakePermissions{"user-" + aliceAccountID: tc.perms}
				o.ReportCounts, o.RegistrationCounts = fakeCounts{reports: 3}, fakeCounts{reports: 3}
			})
			e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
			res := e.admin(t, http.MethodGet, "/admin/counts", "admin-read", nil)
			if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("GET /admin/counts = %d (Cache-Control %q), want 200 no-store", res.StatusCode, res.Header.Get("Cache-Control"))
			}
			if got := decodeJSON[clientapi.AdminCounts](t, res); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("counts = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestGetAdminCountsNeedsTheAdminScope checks that a token without admin:read is refused.
func TestGetAdminCountsNeedsTheAdminScope(t *testing.T) {
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{"user-" + aliceAccountID: identity.AllPermissions()}
		o.ReportCounts, o.RegistrationCounts = fakeCounts{}, fakeCounts{}
	})
	if res := e.admin(t, http.MethodGet, "/admin/counts", "read", nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("GET /admin/counts with read = %d, want 403", res.StatusCode)
	}
}
