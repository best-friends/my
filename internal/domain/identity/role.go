// SPDX-License-Identifier: AGPL-3.0-only

package identity

import "time"

// Permission is a capability that a role grants.
type Permission string

// Permissions (data-model §4.1, role_permissions.permission).
const (
	PermManageReports  Permission = "manage_reports"
	PermManageAccounts Permission = "manage_accounts"
	PermManageDomains  Permission = "manage_domains"
	PermManageRules    Permission = "manage_rules"
	PermManageSettings Permission = "manage_settings"
	PermManageRoles    Permission = "manage_roles"
	PermManageInvites  Permission = "manage_invites"
	PermViewAudit      Permission = "view_audit"
	// PermManageJobs: read and requeue the dead jobs of the job queue.
	PermManageJobs Permission = "manage_jobs"
	// PermManageTwoFactor: remove the two-factor authentication of a local user (FR-005). It is
	// server administration (ADR-0058 §4): only the built-in admin role has it by default.
	PermManageTwoFactor Permission = "manage_two_factor"
)

// AllPermissions lists every permission.
func AllPermissions() []Permission {
	return []Permission{
		PermManageReports, PermManageAccounts, PermManageDomains, PermManageRules,
		PermManageSettings, PermManageRoles, PermManageInvites, PermViewAudit, PermManageJobs,
		PermManageTwoFactor,
	}
}

// Role is a named set of permissions. Position orders roles; a higher position outranks a lower one.
type Role struct {
	ID        string
	Key       string
	Position  int
	CreatedAt time.Time
}

// RoleGrant is a role granted to a user (data-model §4.1, user_role_grants).
type RoleGrant struct {
	ID     string
	UserID string
	RoleID string
	// GrantedByUserID is the user who granted the role, or "" for a grant from the CLI.
	GrantedByUserID string
	GrantedAt       time.Time
}

// Keys of the built-in roles.
const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
)

// BuiltinRole describes a role that every instance has.
type BuiltinRole struct {
	Key         string
	Position    int
	Permissions []Permission
}

// BuiltinRoles returns the built-in roles: admin holds every permission; moderator handles
// reports and accounts and reads the audit log.
func BuiltinRoles() []BuiltinRole {
	return []BuiltinRole{
		{Key: RoleAdmin, Position: 1000, Permissions: AllPermissions()},
		{Key: RoleModerator, Position: 100, Permissions: []Permission{PermManageReports, PermManageAccounts, PermViewAudit}},
	}
}
