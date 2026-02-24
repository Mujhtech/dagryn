package models

// Role represents a user's role in a team or project.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer" // Only valid for projects
)

// IsValidTeamRole checks if the role is valid for team membership.
func IsValidTeamRole(r Role) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	}
	return false
}

// IsValidProjectRole checks if the role is valid for project membership.
func IsValidProjectRole(r Role) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	}
	return false
}

// CanManageMembers returns true if the role can manage other members.
func (r Role) CanManageMembers() bool {
	return r == RoleOwner || r == RoleAdmin
}

// CanTriggerRuns returns true if the role can trigger workflow runs.
func (r Role) CanTriggerRuns() bool {
	return r != RoleViewer
}

// CanViewProject returns true if the role can view project details.
func (r Role) CanViewProject() bool {
	return true // All roles can view
}

// CanDeleteProject returns true if the role can delete the project.
func (r Role) CanDeleteProject() bool {
	return r == RoleOwner
}

// CanManageAPIKeys returns true if the role can manage API keys.
func (r Role) CanManageAPIKeys() bool {
	return r == RoleOwner || r == RoleAdmin
}

// Permission represents a specific permission.
type Permission string

const (
	PermissionProjectView     Permission = "project:view"
	PermissionProjectEdit     Permission = "project:edit"
	PermissionProjectDelete   Permission = "project:delete"
	PermissionRunTrigger      Permission = "run:trigger"
	PermissionRunCancel       Permission = "run:cancel"
	PermissionRunView         Permission = "run:view"
	PermissionMembersView     Permission = "members:view"
	PermissionMembersManage   Permission = "members:manage"
	PermissionAPIKeysView     Permission = "apikeys:view"
	PermissionAPIKeysManage   Permission = "apikeys:manage"
	PermissionCacheView       Permission = "cache:view"
	PermissionCacheClear      Permission = "cache:clear"
	PermissionAuditLogsView   Permission = "audit_logs:view"
	PermissionAuditLogsExport Permission = "audit_logs:export"
	PermissionAuditLogsManage Permission = "audit_logs:manage"
)

// RolePermissions maps roles to their permissions.
var RolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermissionProjectView, PermissionProjectEdit, PermissionProjectDelete,
		PermissionRunTrigger, PermissionRunCancel, PermissionRunView,
		PermissionMembersView, PermissionMembersManage,
		PermissionAPIKeysView, PermissionAPIKeysManage,
		PermissionCacheView, PermissionCacheClear,
		PermissionAuditLogsView, PermissionAuditLogsExport, PermissionAuditLogsManage,
	},
	RoleAdmin: {
		PermissionProjectView, PermissionProjectEdit,
		PermissionRunTrigger, PermissionRunCancel, PermissionRunView,
		PermissionMembersView, PermissionMembersManage,
		PermissionAPIKeysView, PermissionAPIKeysManage,
		PermissionCacheView, PermissionCacheClear,
		PermissionAuditLogsView, PermissionAuditLogsExport, PermissionAuditLogsManage,
	},
	RoleMember: {
		PermissionProjectView,
		PermissionRunTrigger, PermissionRunCancel, PermissionRunView,
		PermissionMembersView,
		PermissionAPIKeysView,
		PermissionCacheView,
	},
	RoleViewer: {
		PermissionProjectView,
		PermissionRunView,
		PermissionMembersView,
		PermissionCacheView,
	},
}

// HasPermission checks if a role has a specific permission.
func (r Role) HasPermission(p Permission) bool {
	perms, ok := RolePermissions[r]
	if !ok {
		return false
	}
	for _, perm := range perms {
		if perm == p {
			return true
		}
	}
	return false
}
