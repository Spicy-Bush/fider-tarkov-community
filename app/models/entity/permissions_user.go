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

	if target == nil || target.Status == enum.UserDeleted || !canAct(viewer, tenant) {
		return permissions
	}

	if target.Tenant != nil && tenant != nil && target.Tenant.ID != tenant.ID {
		return permissions
	}

	self := viewer.ID == target.ID
	permissions.ReadProfile = self || Can(viewer, tenant, ReadProfiles)
	permissions.EditName = self || Can(viewer, tenant, EditUserProfiles)

	if !self && viewer.Role == enum.RoleModerator && (target.Role == enum.RoleAdministrator || target.Role == enum.RoleCollaborator) {
		permissions.EditName = false
	}

	permissions.EditAvatar = permissions.EditName
	permissions.ChangeRole = !self && Can(viewer, tenant, ChangeUserRoles)
	permissions.ChangeVisualRole = Can(viewer, tenant, ChangeUserVisualRoles) &&
		(target.Role != enum.RoleAdministrator || viewer.Role == enum.RoleAdministrator)

	if !self && target.Role != enum.RoleAdministrator {
		permissions.Block = Can(viewer, tenant, BlockUsers) &&
			(viewer.Role != enum.RoleCollaborator || target.Role == enum.RoleVisitor || target.Role == enum.RoleHelper)

		canModerateTarget := true
		if viewer.Role == enum.RoleModerator {
			canModerateTarget = target.Role == enum.RoleVisitor || target.Role == enum.RoleHelper
		} else if viewer.Role == enum.RoleCollaborator {
			canModerateTarget = target.Role != enum.RoleCollaborator
		}

		permissions.Moderate = canModerateTarget && target.Status != enum.UserBlocked && Can(viewer, tenant, ModerateUsers)
		permissions.DeleteModeration = canModerateTarget && Can(viewer, tenant, DeleteUserModeration)
		permissions.ExpireModeration = canModerateTarget && Can(viewer, tenant, ExpireUserModeration)
	}

	return permissions
}
