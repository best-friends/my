// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"database/sql"
	"net/http"
	"reflect"
	"testing"

	"github.com/mycroblog/my/internal/domain/identity"
	"github.com/mycroblog/my/internal/platform"
)

// TestServeEnsuresBuiltinRoles starts the server on a database whose built-in roles an older
// version created: the admin role gains every permission without a manual step, and the
// moderator role keeps the permissions an operator gave it.
func TestServeEnsuresBuiltinRoles(t *testing.T) {
	t.Parallel()
	path := newDatabase(t, true)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO roles (id, key, position, created_at) VALUES ('old-admin', 'admin', 1000, 0), ('old-moderator', 'moderator', 100, 0)`,
		`INSERT INTO role_permissions (role_id, permission) VALUES ('old-admin', 'manage_reports'), ('old-moderator', 'view_audit')`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := start(t, loadConfig(t, map[string]string{"MY_DATABASE_URL": path}), platform.ServeOptions{}, nil)
	// Serve accepts connections only after it has ensured the roles.
	if resp, _ := s.do(t, http.MethodGet, platform.PathLiveness, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("liveness = %d", resp.StatusCode)
	}
	roles, err := s.app.Identity.Roles(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]identity.Permission{}
	for _, r := range roles {
		got[r.Key+"/"+r.ID] = r.Permissions
	}
	want := map[string][]identity.Permission{
		"admin/old-admin":         identity.AllPermissions(),
		"moderator/old-moderator": {identity.PermViewAudit},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roles = %v, want %v", got, want)
	}
}
