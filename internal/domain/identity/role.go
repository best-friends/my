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
	// PermManageCustomEmojis: register, change, disable, and delete the custom emoji, local and
	// remote (ADR-0051 §5). It is server administration (ADR-0058 §4).
	PermManageCustomEmojis Permission = "manage_custom_emojis"
	// PermManageWebhooks: set up the instance's webhooks, see their deliveries, and send them
	// again (ADR-0060). It is server administration (ADR-0058 §4).
	PermManageWebhooks Permission = "manage_webhooks"
)

// AllPermissions lists every permission.
func AllPermissions() []Permission {
	return []Permission{
		PermManageReports, PermManageAccounts, PermManageDomains, PermManageRules,
		PermManageSettings, PermManageRoles, PermManageInvites, PermViewAudit, PermManageJobs,
		PermManageTwoFactor, PermManageCustomEmojis, PermManageWebhooks,
	}
}

// PermissionCategory is the area of the staff's work that a permission belongs to (ADR-0058):
// moderation decides on people, posts and remote servers case by case; server administration
// changes the server itself as a whole.
type PermissionCategory string

// Permission categories.
const (
	CategoryModeration PermissionCategory = "moderation"
	CategoryServer     PermissionCategory = "server"
)

// Category returns the area that p belongs to. The web client's navigation and `my admin role`
// group the permissions by it.
func (p Permission) Category() PermissionCategory {
	switch p {
	case PermManageReports, PermManageAccounts, PermManageDomains, PermManageInvites, PermViewAudit:
		return CategoryModeration
	default:
		return CategoryServer
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
		{Key: RoleAdmin, Position: AdminPosition, Permissions: AllPermissions()},
		{Key: RoleModerator, Position: 100, Permissions: []Permission{PermManageReports, PermManageAccounts, PermViewAudit}},
	}
}
