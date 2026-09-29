package entity

import (
	"maps"
	"slices"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type Permission string

const (
	BypassContentRestrictions Permission = "bypassContentRestrictions"
	BypassPostingRateLimits   Permission = "bypassPostingRateLimits"
	TagPostsOutsideWindow     Permission = "tagPostsOutsideWindow"
	ReadSettings              Permission = "readSettings"
	ManageProfanity           Permission = "manageProfanity"
	CreateUsers               Permission = "createUsers"
	ViewDesignSystem          Permission = "viewDesignSystem"
	ManageAPIKeys             Permission = "manageAPIKeys"
	ChangeOwnEmail            Permission = "changeOwnEmail"
	EnableEmailNotifications  Permission = "enableEmailNotifications"
	ManageSettings            Permission = "manageSettings"
	ManageContentSettings     Permission = "manageContentSettings"
	ManagePages               Permission = "managePages"
	ManagePageTopics          Permission = "managePageTopics"
	ManageNavigation          Permission = "manageNavigation"
	ManageTags                Permission = "manageTags"
	ViewPrivateTags           Permission = "viewPrivateTags"
	ManageMembers             Permission = "manageMembers"
	ManageReports             Permission = "manageReports"
	ManageReportReasons       Permission = "manageReportReasons"
	ManageQueue               Permission = "manageQueue"
	ManageArchive             Permission = "manageArchive"
	ManageResponses           Permission = "manageResponses"
	ReadResponses             Permission = "readResponses"
	ManageSponsorship         Permission = "manageSponsorship"
	ManageWebhooks            Permission = "manageWebhooks"
	ManageAuthentication      Permission = "manageAuthentication"
	ManageRolePermissions     Permission = "manageRolePermissions"
	ManageInvitations         Permission = "manageInvitations"
	ManageBilling             Permission = "manageBilling"
	ManageFiles               Permission = "manageFiles"
	ExportBackup              Permission = "exportBackup"
	ReadProfiles              Permission = "readProfiles"
	EditUserProfiles          Permission = "editUserProfiles"
	ModerateUsers             Permission = "moderateUsers"
	DeleteUserModeration      Permission = "deleteUserModeration"
	ExpireUserModeration      Permission = "expireUserModeration"
	BlockUsers                Permission = "blockUsers"
	ChangeUserRoles           Permission = "changeUserRoles"
	ChangeUserVisualRoles     Permission = "changeUserVisualRoles"
	ReadUserEmails            Permission = "readUserEmails"
	CreatePosts               Permission = "createPosts"
	EditPosts                 Permission = "editPosts"
	DeletePosts               Permission = "deletePosts"
	RespondToPosts            Permission = "respondToPosts"
	LockPosts                 Permission = "lockPosts"
	ModeratePosts             Permission = "moderatePosts"
	TagPosts                  Permission = "tagPosts"
	ViewPostVotes             Permission = "viewPostVotes"
	ExportFeedback            Permission = "exportFeedback"
)

type roleSet uint8

func roleSetOf(members ...enum.Role) roleSet {
	var set roleSet
	for _, role := range members {
		set |= 1 << role
	}
	return set
}

func (set roleSet) has(role enum.Role) bool {
	return role > 0 && role < 8 && set&(1<<role) != 0
}

var (
	administratorRoles = roleSetOf(enum.RoleAdministrator)
	collaboratorRoles  = roleSetOf(enum.RoleCollaborator, enum.RoleAdministrator)
	staffRoles         = collaboratorRoles | roleSetOf(enum.RoleModerator)
	helperRoles        = staffRoles | roleSetOf(enum.RoleHelper)
	allRoles           = helperRoles | roleSetOf(enum.RoleVisitor)
)

var defaultRolePermissions = map[Permission]roleSet{
	BypassContentRestrictions: collaboratorRoles,
	BypassPostingRateLimits:   staffRoles,
	TagPostsOutsideWindow:     staffRoles,
	ManageSettings:            administratorRoles,
	ManageProfanity:           administratorRoles,
	CreateUsers:               administratorRoles,
	ViewDesignSystem:          administratorRoles,
	EnableEmailNotifications:  administratorRoles,
	ManagePageTopics:          administratorRoles,
	ManageNavigation:          administratorRoles,
	ManageAuthentication:      administratorRoles,
	ManageRolePermissions:     administratorRoles,
	ManageInvitations:         administratorRoles,
	ManageBilling:             administratorRoles,
	ManageFiles:               administratorRoles,
	ExportBackup:              administratorRoles,
	ChangeUserRoles:           administratorRoles,

	ReadSettings:          collaboratorRoles,
	ManageAPIKeys:         collaboratorRoles,
	ChangeOwnEmail:        collaboratorRoles,
	ManageContentSettings: collaboratorRoles,
	ManagePages:           collaboratorRoles,
	ManageTags:            collaboratorRoles,
	ManageReportReasons:   collaboratorRoles,
	ManageArchive:         collaboratorRoles,
	ManageResponses:       collaboratorRoles,
	ManageSponsorship:     collaboratorRoles,
	ManageWebhooks:        collaboratorRoles,
	DeleteUserModeration:  collaboratorRoles,
	BlockUsers:            collaboratorRoles,
	ChangeUserVisualRoles: collaboratorRoles,
	ReadUserEmails:        collaboratorRoles,
	LockPosts:             collaboratorRoles,
	ExportFeedback:        collaboratorRoles,

	ManageMembers:        staffRoles,
	ViewPrivateTags:      staffRoles,
	ManageReports:        staffRoles,
	ReadResponses:        staffRoles,
	ReadProfiles:         staffRoles,
	EditUserProfiles:     staffRoles,
	ModerateUsers:        staffRoles,
	ExpireUserModeration: staffRoles,
	DeletePosts:          staffRoles,
	ModeratePosts:        staffRoles,
	ViewPostVotes:        staffRoles,

	ManageQueue: helperRoles,
	TagPosts:    helperRoles,

	EditPosts:   allRoles,
	CreatePosts: allRoles,
}

var sessionPermissions = slices.Sorted(maps.Keys(defaultRolePermissions))

func Can(user *User, tenant *Tenant, permission Permission) bool {
	if !CanAct(user, tenant) {
		return false
	}

	if permission == RespondToPosts {
		return len(AllowedPostResponses(user, tenant)) > 0
	}

	var overrides RolePermissions
	if tenant != nil {
		overrides = tenant.RolePermissions
	}
	if !overrides.Grants(user.Role, permission) {
		return false
	}

	if permission == CreatePosts {
		if user.IsMuted() {
			return false
		}

		if tenant == nil || tenant.GeneralSettings == nil {
			return true
		}

		settings := tenant.GeneralSettings
		if settings.PostingGloballyDisabled && !CanBypassContentRestrictions(user, tenant) {
			return false
		}

		for _, role := range settings.PostingDisabledFor {
			if role == user.Role.String() {
				return false
			}
		}
	}

	return true
}

func CanAct(user *User, tenant *Tenant) bool {
	if user == nil || user.Status != enum.UserActive {
		return false
	}

	return tenant == nil || (tenant.Status != enum.TenantLocked && !tenant.IsDisabled())
}

func PermissionsFor(user *User, tenant *Tenant) map[Permission]bool {
	permissions := make(map[Permission]bool, len(sessionPermissions))
	for _, permission := range sessionPermissions {
		permissions[permission] = Can(user, tenant, permission)
	}

	permissions[RespondToPosts] = Can(user, tenant, RespondToPosts)
	return permissions
}

func CanChangeNotificationChannels(user *User, tenant *Tenant, current, next enum.NotificationChannel) bool {
	if !CanAct(user, tenant) {
		return false
	}

	addsEmail := next&enum.NotificationChannelEmail != 0 && current&enum.NotificationChannelEmail == 0
	return !addsEmail || Can(user, tenant, EnableEmailNotifications)
}
