package authz

import (
	pb "github.com/Ajay01103/go-notion/workspace/gen/pb"
)

// Permission defines what operations a user can perform
type Permission string

const (
	// Workspace-level permissions
	PermWorkspaceView   Permission = "workspace:view"
	PermWorkspaceEdit   Permission = "workspace:edit"
	PermWorkspaceDelete Permission = "workspace:delete"

	// Members permissions
	PermMembersView    Permission = "members:view"
	PermMembersInvite  Permission = "members:invite"
	PermMembersRemove  Permission = "members:remove"
	PermMembersPromote Permission = "members:promote"

	// Content permissions (for future use)
	PermContentCreate  Permission = "content:create"
	PermContentEdit    Permission = "content:edit"
	PermContentDelete  Permission = "content:delete"
	PermContentView    Permission = "content:view"
	PermContentComment Permission = "content:comment"
)

// RoleLevel represents the privilege level of a role
type RoleLevel int

const (
	RoleLevelGuest RoleLevel = iota
	RoleLevelMember
	RoleLevelEditor
	RoleLevelAdmin
	RoleLevelOwner
)

// roleHierarchy maps role names to privilege levels
var roleHierarchy = map[string]RoleLevel{
	"guest":  RoleLevelGuest,
	"member": RoleLevelMember,
	"editor": RoleLevelEditor,
	"admin":  RoleLevelAdmin,
	"owner":  RoleLevelOwner,
}

// protoRoleToString converts proto role enum to string
func protoRoleToString(role pb.WorkspaceRole) string {
	switch role {
	case pb.WorkspaceRole_WORKSPACE_ROLE_OWNER:
		return "owner"
	case pb.WorkspaceRole_WORKSPACE_ROLE_ADMIN:
		return "admin"
	case pb.WorkspaceRole_WORKSPACE_ROLE_EDITOR:
		return "editor"
	case pb.WorkspaceRole_WORKSPACE_ROLE_MEMBER:
		return "member"
	case pb.WorkspaceRole_WORKSPACE_ROLE_GUEST:
		return "guest"
	default:
		return ""
	}
}

// ProtoRoleToString converts proto role enum to string.
func ProtoRoleToString(role pb.WorkspaceRole) string {
	return protoRoleToString(role)
}

// stringToProtoRole converts string role to proto role enum
func stringToProtoRole(role string) pb.WorkspaceRole {
	switch role {
	case "owner":
		return pb.WorkspaceRole_WORKSPACE_ROLE_OWNER
	case "admin":
		return pb.WorkspaceRole_WORKSPACE_ROLE_ADMIN
	case "editor":
		return pb.WorkspaceRole_WORKSPACE_ROLE_EDITOR
	case "member":
		return pb.WorkspaceRole_WORKSPACE_ROLE_MEMBER
	case "guest":
		return pb.WorkspaceRole_WORKSPACE_ROLE_GUEST
	default:
		return pb.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED
	}
}

// StringToProtoRole converts string role to proto role enum.
func StringToProtoRole(role string) pb.WorkspaceRole {
	return stringToProtoRole(role)
}

// rolePermissions maps roles to their allowed permissions
var rolePermissions = map[string][]Permission{
	"guest": {
		PermWorkspaceView,
		PermMembersView,
		PermContentView,
	},
	"member": {
		PermWorkspaceView,
		PermMembersView,
		PermContentView,
		PermContentCreate,
		PermContentEdit,    // own content only — enforced at content service
		PermContentComment,
	},
	"editor": {
		PermWorkspaceView,
		PermMembersView,
		PermContentView,
		PermContentCreate,
		PermContentEdit,    // any content
		PermContentDelete,
		PermContentComment,
	},
	"admin": {
		PermWorkspaceView,
		PermWorkspaceEdit,
		PermMembersView,
		PermMembersInvite,
		PermMembersRemove,
		PermMembersPromote,
		PermContentView,
		PermContentCreate,
		PermContentEdit,
		PermContentDelete,
		PermContentComment,
	},
	"owner": {
		PermWorkspaceView,
		PermWorkspaceEdit,
		PermWorkspaceDelete,
		PermMembersView,
		PermMembersInvite,
		PermMembersRemove,
		PermMembersPromote,
		PermContentView,
		PermContentCreate,
		PermContentEdit,
		PermContentDelete,
		PermContentComment,
	},
}

// Checker provides authorization checks
type Checker struct{}

// Can checks if a role has a specific permission
func (c *Checker) Can(role string, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// RoleAtLeast checks if a role meets a minimum level
// Useful for: "must be at least admin"
func (c *Checker) RoleAtLeast(role, minimum string) bool {
	roleLevel, ok1 := roleHierarchy[role]
	minLevel, ok2 := roleHierarchy[minimum]
	if !ok1 || !ok2 {
		return false
	}
	return roleLevel >= minLevel
}

// CanPromoteTo checks if an actor can assign a target role
// Rule: you can only assign roles strictly below your own
func (c *Checker) CanPromoteTo(actorRole, targetRole string) bool {
	actorLevel, ok1 := roleHierarchy[actorRole]
	targetLevel, ok2 := roleHierarchy[targetRole]
	if !ok1 || !ok2 {
		return false
	}
	return actorLevel > targetLevel
}

// GetRoleLevel returns the privilege level of a role
func GetRoleLevel(role string) RoleLevel {
	level, ok := roleHierarchy[role]
	if !ok {
		return RoleLevelGuest
	}
	return level
}

// New creates a new authorization checker
func New() *Checker {
	return &Checker{}
}
