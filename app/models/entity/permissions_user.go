package entity

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

type UserPermissions struct {
	ReadProfile      bool `json:"readProfile"`
	EditName         bool `json:"editName"`
	EditAvatar       bool `json:"editAvatar"`
	Block            bool `json:"block"`
	Moderate         bool `json:"moderate"`
	DeleteModeration bool `json:"deleteModeration"`
	ExpireModeration bool `json:"expireModeration"`
	ChangeRole       bool `json:"changeRole"`
	ChangeVisualRole bool `json:"changeVisualRole"`
}

func (target *User) AllowedActions(viewer *User, tenant *Tenant) UserPermissions {
	permissions := UserPermissions{}

	if target == nil || target.Status == enum.UserDeleted || !CanAct(viewer, tenant) {
		return permissions
	}

	if target.Tenant != nil && tenant != nil && target.Tenant.ID != tenant.ID {
		return permissions
	}

	self := viewer.ID == target.ID
	outranks := viewer.outranks(target.Role)

	canEditProfile := viewer.IsAdministrator() || (target.Role != enum.RoleAdministrator &&
		(viewer.Role == enum.RoleCollaborator || roleRank(target.Role) <= roleRank(viewer.Role)))

	permissions.ReadProfile = self || Can(viewer, tenant, ReadProfiles)
	permissions.EditName = self || (canEditProfile && Can(viewer, tenant, EditUserProfiles))
	permissions.EditAvatar = permissions.EditName
	permissions.ChangeRole = !self && outranks && Can(viewer, tenant, ChangeUserRoles)
	permissions.ChangeVisualRole = canEditProfile && Can(viewer, tenant, ChangeUserVisualRoles)

	if !self && target.Role != enum.RoleAdministrator {
		permissions.Block = Can(viewer, tenant, BlockUsers) &&
			(viewer.IsAdministrator() || (outranks && (target.Role == enum.RoleVisitor || target.Role == enum.RoleHelper)))

		permissions.Moderate = outranks && target.Status != enum.UserBlocked && Can(viewer, tenant, ModerateUsers)
		permissions.DeleteModeration = outranks && Can(viewer, tenant, DeleteUserModeration)
		permissions.ExpireModeration = outranks && Can(viewer, tenant, ExpireUserModeration)
	}

	return permissions
}

func roleRank(role enum.Role) int {
	for index, candidate := range PermissionRoles {
		if candidate == role {
			return index + 1
		}
	}
	return 0
}

func (viewer *User) outranks(role enum.Role) bool {
	return viewer.IsAdministrator() || roleRank(role) < roleRank(viewer.Role)
}

func CanAssignRole(viewer *User, role enum.Role) bool {
	return roleRank(role) > 0 && viewer.outranks(role)
}
