package entity

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

// Overrides of defaultRolePermissions; a missing entry uses the default.
type RolePermissions map[enum.Role]map[Permission]bool

type RolePermissionChange struct {
	Role       enum.Role  `json:"role"`
	Permission Permission `json:"permission"`
	Granted    bool       `json:"granted"`
}

type RolePermissionState struct {
	Responses        map[enum.Role][]enum.PostStatus          `json:"responses"`
	DefaultResponses map[enum.Role][]enum.PostStatus          `json:"defaultResponses"`
	ResponseOptions  []enum.PostStatus                        `json:"responseOptions"`
	ResponseLocks    map[enum.Role]map[enum.PostStatus]string `json:"responseLocks"`
	Permissions      map[enum.Role][]Permission               `json:"permissions"`
	BaseLocks        map[enum.Role]map[Permission]string      `json:"baseLocks"`
	Requires         map[Permission][]Permission              `json:"requires"`
}

type RolePermissionUpdate struct {
	RolePermissionState
	Blocked string `json:"blocked,omitempty"`
}

func (tenant *Tenant) PermissionState(viewer *User) RolePermissionState {
	return RolePermissionState{
		Permissions:      tenant.RolePermissions.GrantedByRole(),
		Responses:        tenant.RolePostResponses.ByRole(),
		DefaultResponses: RolePostResponses(nil).ByRole(),
		ResponseOptions:  ResponseOptions,
		ResponseLocks:    RoleResponseLocks(viewer, tenant),
		BaseLocks:        RolePermissionBaseLocks(viewer, tenant),
		Requires:         PermissionRequirements,
	}
}

func (permissions RolePermissions) ChangesFrom(previous RolePermissions) []RolePermissionChange {
	changes := []RolePermissionChange{}
	for _, role := range PermissionRoles {
		for _, permission := range sessionPermissions {
			granted := permissions.Grants(role, permission)
			if granted != previous.Grants(role, permission) {
				changes = append(changes, RolePermissionChange{Role: role, Permission: permission, Granted: granted})
			}
		}
	}

	return changes
}

var PermissionRoles = []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}

var MaxRolePermissionChanges = len(PermissionRoles) * len(sessionPermissions)

var PermissionRequirements = map[Permission][]Permission{
	TagPostsOutsideWindow: {TagPosts},
	ManageReportReasons:   {ManageReports},
	ManageTags:            {ViewPrivateTags},
	ManageMembers:         {ReadProfiles},
	EditUserProfiles:      {ReadProfiles},
	ReadUserEmails:        {ReadProfiles},
	ModerateUsers:         {ReadProfiles},
	ExpireUserModeration:  {ModerateUsers},
	DeleteUserModeration:  {ModerateUsers},
	BlockUsers:            {ReadProfiles},
	ChangeUserVisualRoles: {ReadProfiles},
	ChangeUserRoles:       {ManageMembers},
	ManageSettings:        {ReadSettings},
	ManageContentSettings: {ReadSettings},
	ManagePageTopics:      {ManagePages},
	ManageResponses:       {ReadResponses},
	ManageProfanity:       {ReadSettings},
}

var permissionDependents = map[Permission][]Permission{}

func init() {
	for _, permission := range sessionPermissions {
		for _, requirement := range PermissionRequirements[permission] {
			permissionDependents[requirement] = append(permissionDependents[requirement], permission)
		}
	}
}

func IsPermission(permission Permission) bool {
	_, ok := defaultRolePermissions[permission]
	return ok
}

func IsPermissionRole(role enum.Role) bool {
	return roleSetOf(PermissionRoles...).has(role)
}

func (overrides RolePermissions) Grants(role enum.Role, permission Permission) bool {
	if role == enum.RoleAdministrator {
		return IsPermission(permission)
	}
	if isAdministratorOnlyPermission(permission) {
		return false
	}
	if permission == ManageRolePermissions && role == enum.RoleVisitor {
		return false
	}

	if granted, ok := overrides[role][permission]; ok {
		return granted
	}
	return defaultRolePermissions[permission].has(role)
}

func isAdministratorOnlyPermission(permission Permission) bool {
	// Both can expose or assert another administrator's identity.
	return permission == ManageAuthentication || permission == ExportBackup
}

func (overrides RolePermissions) GrantedByRole() map[enum.Role][]Permission {
	granted := make(map[enum.Role][]Permission, len(PermissionRoles))
	for _, role := range PermissionRoles {
		granted[role] = []Permission{}
		for _, permission := range sessionPermissions {
			if overrides.Grants(role, permission) {
				granted[role] = append(granted[role], permission)
			}
		}
	}
	return granted
}

func RolePermissionLock(viewer *User, tenant *Tenant, role enum.Role, permission Permission) string {
	if role == enum.RoleAdministrator {
		return "Administrators have every permission"
	}
	if permission == ManageRolePermissions && role == enum.RoleVisitor {
		return "Visitors cannot manage role permissions"
	}
	if isAdministratorOnlyPermission(permission) {
		if permission == ExportBackup {
			return "Only administrators can export full backups"
		}
		return "Only administrators can manage authentication"
	}

	if viewer.IsAdministrator() {
		return ""
	}
	if !viewer.outranks(role) {
		return "You can only change roles below your own"
	}
	if !tenant.RolePermissions.Grants(viewer.Role, permission) {
		return "You can only change permissions you have"
	}
	return ""
}

func RolePermissionBaseLocks(viewer *User, authority *Tenant) map[enum.Role]map[Permission]string {
	locks := make(map[enum.Role]map[Permission]string, len(PermissionRoles))
	for _, role := range PermissionRoles {
		locks[role] = map[Permission]string{}
		for _, permission := range sessionPermissions {
			if reason := RolePermissionLock(viewer, authority, role, permission); reason != "" {
				locks[role][permission] = reason
			}
		}
	}

	return locks
}

func lockedPermissionChange(viewer *User, authority *Tenant, role enum.Role, cells map[Permission]bool) string {
	for permission := range cells {
		if reason := RolePermissionLock(viewer, authority, role, permission); reason != "" {
			return reason
		}
	}

	return ""
}

// Revocations run first so a batch revoking a requirement and granting its dependent keeps both.
func (current RolePermissions) ApplyChanges(viewer *User, authority *Tenant, changes []RolePermissionChange) (next RolePermissions, blocked string) {
	next = make(RolePermissions, len(current))
	for role, permissions := range current {
		next[role] = make(map[Permission]bool, len(permissions))
		for permission, granted := range permissions {
			next[role][permission] = granted
		}
	}

	for _, granting := range []bool{false, true} {
		for _, change := range changes {
			if change.Granted != granting {
				continue
			}

			cells := map[Permission]bool{}
			collectRolePermissionClosure(next, change.Role, change.Permission, change.Granted, cells)
			if reason := lockedPermissionChange(viewer, authority, change.Role, cells); reason != "" {
				return nil, reason
			}

			for permission := range cells {
				next.set(change.Role, permission, change.Granted)
			}
		}
	}

	return next, ""
}

func collectRolePermissionClosure(current RolePermissions, role enum.Role, permission Permission, granted bool, cells map[Permission]bool) {
	if cells[permission] || current.Grants(role, permission) == granted {
		return
	}
	cells[permission] = true

	linked := permissionDependents[permission]
	if granted {
		linked = PermissionRequirements[permission]
	}
	for _, next := range linked {
		collectRolePermissionClosure(current, role, next, granted, cells)
	}
}

// Cells equal to the default are not stored, so they follow future default changes.
func (overrides RolePermissions) set(role enum.Role, permission Permission, granted bool) {
	if defaultRolePermissions[permission].has(role) == granted {
		delete(overrides[role], permission)
		if len(overrides[role]) == 0 {
			delete(overrides, role)
		}
		return
	}

	if overrides[role] == nil {
		overrides[role] = map[Permission]bool{}
	}
	overrides[role][permission] = granted
}
