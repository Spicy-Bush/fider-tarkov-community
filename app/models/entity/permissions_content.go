package entity

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

func CanBypassContentRestrictions(user *User, tenant *Tenant) bool {
	return Can(user, tenant, BypassContentRestrictions)
}

func CanBypassPostingRateLimits(user *User, tenant *Tenant) bool {
	return Can(user, tenant, BypassPostingRateLimits)
}

func canManageContentRole(viewer *User, target enum.Role) bool {
	return viewer != nil && (collaboratorRoles.has(viewer.Role) || (roleRank(target) > 0 && viewer.outranks(target)))
}

// SQL readers bind these roles
func ModeratedContentRoles(viewer *User, tenant *Tenant) []enum.Role {
	roles := []enum.Role{}
	if !Can(viewer, tenant, ModeratePosts) {
		return roles
	}

	if collaboratorRoles.has(viewer.Role) {
		roles = append(roles, 0)
	}
	for _, role := range PermissionRoles {
		if canManageContentRole(viewer, role) {
			roles = append(roles, role)
		}
	}
	return roles
}
