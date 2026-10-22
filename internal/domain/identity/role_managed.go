// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Roles that another module manages (ADR-0072 §5, managed_roles): the role of a supporter plan
// is granted and revoked only by the supporters module, from the state of a subscription, and
// money never buys a staff permission. The rules hold whatever the order of the changes:
//
//   - a managed role has only member permissions (ADR-0070 §1) and a position below every staff
//     role;
//   - a staff role (a built-in role, or one with a moderation or server permission) stays above
//     every managed role;
//   - a role becomes managed only while nobody holds it, so that its grants all come from the
//     module;
//   - nobody grants or revokes a managed role by hand, nor changes its permissions or position
//     or deletes it directly: the module does, through the operations that take its transaction.

// RoleManager names the module that manages a role (managed_roles.manager).
type RoleManager string

// The managers.
const (
	// ManagerSupporters: the role of a supporter plan (ADR-0072).
	ManagerSupporters RoleManager = "supporters"
)

// ManagedRole is a role that a module manages (managed_roles).
type ManagedRole struct {
	RoleID    string
	Manager   RoleManager
	CreatedAt time.Time
}

// ErrRoleManagedByPlan reports a direct change to a managed role: a grant or a revocation by
// hand, a change of its permissions or position outside its plan, or its deletion.
var ErrRoleManagedByPlan = errors.New("identity: a supporter plan manages the role")

// ErrBreaksManagedRoles reports a change to a role that is not managed that would put a staff
// role at or below a managed one: a staff role moved to or below the highest managed role, or a
// staff permission added to a role there.
var ErrBreaksManagedRoles = errors.New("identity: the change would put a staff role at or below a role that a supporter plan manages")

// ErrRoleNotManageable is what every UnmanageableRoleError matches.
var ErrRoleNotManageable = errors.New("identity: the role cannot be managed by a supporter plan")

// ManageRule is a rule that a role must meet to be managed (ADR-0072 §5).
type ManageRule string

// The rules.
const (
	// RuleBuiltin: admin and moderator are never managed.
	RuleBuiltin ManageRule = "builtin"
	// RuleManaged: another plan manages the role already.
	RuleManaged ManageRule = "managed"
	// RuleHeld: users hold the role.
	RuleHeld ManageRule = "held"
	// RulePermissions: the role has a permission that is not of CategoryMember.
	RulePermissions ManageRule = "permissions"
	// RulePosition: the role is not below every staff role.
	RulePosition ManageRule = "position"
)

// UnmanageableRoleError reports the rule that a role breaks to be managed.
type UnmanageableRoleError struct {
	Rule ManageRule
}

func (e *UnmanageableRoleError) Error() string {
	return fmt.Sprintf("identity: the role cannot be managed by a supporter plan: %s", e.Rule)
}

// Is makes the error match ErrRoleNotManageable.
func (e *UnmanageableRoleError) Is(target error) bool { return target == ErrRoleNotManageable }

// RoleFacts is what the rules of the managed roles read of a role.
type RoleFacts struct {
	Role        Role
	Permissions []Permission
	// Managed reports a row in managed_roles.
	Managed bool
}

// MemberOnly reports whether every permission of perms is of CategoryMember.
func MemberOnly(perms []Permission) bool {
	return !slices.ContainsFunc(perms, func(p Permission) bool { return p.Category() != CategoryMember })
}

// IsStaffRole reports whether a role with key and perms is a staff role: a built-in role, or one
// with a permission of the moderation or server category.
func IsStaffRole(key string, perms []Permission) bool {
	return IsBuiltinRole(key) || !MemberOnly(perms)
}

// lowestStaffPosition returns the lowest position among the staff roles of roles other than
// except, and false without one.
func lowestStaffPosition(roles []RoleFacts, except string) (int, bool) {
	lowest, ok := 0, false
	for _, r := range roles {
		if r.Role.ID == except || r.Managed || !IsStaffRole(r.Role.Key, r.Permissions) {
			continue
		}
		if !ok || r.Role.Position < lowest {
			lowest, ok = r.Role.Position, true
		}
	}
	return lowest, ok
}

// highestManagedPosition returns the highest position among the managed roles of roles other
// than except, and false without one.
func highestManagedPosition(roles []RoleFacts, except string) (int, bool) {
	highest, ok := 0, false
	for _, r := range roles {
		if r.Role.ID == except || !r.Managed {
			continue
		}
		if !ok || r.Role.Position > highest {
			highest, ok = r.Role.Position, true
		}
	}
	return highest, ok
}

// CheckManagedShape checks the permissions and the position that a managed role (roleID, which
// may be new) is to have among roles: only member permissions, and a position below every staff
// role. It returns an *UnmanageableRoleError.
func CheckManagedShape(roleID string, position int, perms []Permission, roles []RoleFacts) error {
	if !MemberOnly(perms) {
		return &UnmanageableRoleError{Rule: RulePermissions}
	}
	if lowest, ok := lowestStaffPosition(roles, roleID); ok && position >= lowest {
		return &UnmanageableRoleError{Rule: RulePosition}
	}
	return nil
}

// CheckManageable checks that candidate may become managed: not a built-in role, not managed,
// held by nobody (holders), and of the shape of CheckManagedShape among roles. It returns an
// *UnmanageableRoleError.
func CheckManageable(candidate RoleFacts, holders int64, roles []RoleFacts) error {
	switch {
	case IsBuiltinRole(candidate.Role.Key):
		return &UnmanageableRoleError{Rule: RuleBuiltin}
	case candidate.Managed:
		return &UnmanageableRoleError{Rule: RuleManaged}
	case holders > 0:
		return &UnmanageableRoleError{Rule: RuleHeld}
	}
	return CheckManagedShape(candidate.Role.ID, candidate.Role.Position, candidate.Permissions, roles)
}

// CheckUnmanagedChange checks the position and the permissions that a role that is not managed
// (roleID, "" for a new role, with key) is to have among roles: a staff role must stay above
// every managed role. It fails with ErrBreaksManagedRoles.
func CheckUnmanagedChange(roleID, key string, position int, perms []Permission, roles []RoleFacts) error {
	if !IsStaffRole(key, perms) {
		return nil
	}
	if highest, ok := highestManagedPosition(roles, roleID); ok && position <= highest {
		return fmt.Errorf("%w: position %d is at or below %d", ErrBreaksManagedRoles, position, highest)
	}
	return nil
}
