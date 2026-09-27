package entity

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

type Permission string

const (
	ReadSettings             Permission = "readSettings"
	ManageProfanity          Permission = "manageProfanity"
	CreateUsers              Permission = "createUsers"
	ViewDesignSystem         Permission = "viewDesignSystem"
	ManageAPIKeys            Permission = "manageAPIKeys"
	ChangeOwnEmail           Permission = "changeOwnEmail"
	EnableEmailNotifications Permission = "enableEmailNotifications"
	ManageSettings           Permission = "manageSettings"
	ManageContentSettings    Permission = "manageContentSettings"
	ManagePages              Permission = "managePages"
	ManagePageTopics         Permission = "managePageTopics"
	ManageNavigation         Permission = "manageNavigation"
	ManageTags               Permission = "manageTags"
	ManageMembers            Permission = "manageMembers"
	ManageReports            Permission = "manageReports"
	ManageReportReasons      Permission = "manageReportReasons"
	ManageQueue              Permission = "manageQueue"
	ManageArchive            Permission = "manageArchive"
	ManageResponses          Permission = "manageResponses"
	ReadResponses            Permission = "readResponses"
	ManageSponsorship        Permission = "manageSponsorship"
	ManageWebhooks           Permission = "manageWebhooks"
	ManageAuthentication     Permission = "manageAuthentication"
	ManageInvitations        Permission = "manageInvitations"
	ManageBilling            Permission = "manageBilling"
	ManageFiles              Permission = "manageFiles"
	ExportBackup             Permission = "exportBackup"
	ReadProfiles             Permission = "readProfiles"
	EditUserProfiles         Permission = "editUserProfiles"
	ModerateUsers            Permission = "moderateUsers"
	DeleteUserModeration     Permission = "deleteUserModeration"
	ExpireUserModeration     Permission = "expireUserModeration"
	BlockUsers               Permission = "blockUsers"
	ChangeUserRoles          Permission = "changeUserRoles"
	ChangeUserVisualRoles    Permission = "changeUserVisualRoles"
	ReadUserEmails           Permission = "readUserEmails"
	CreatePosts              Permission = "createPosts"
	EditPosts                Permission = "editPosts"
	DeletePosts              Permission = "deletePosts"
	RespondToPosts           Permission = "respondToPosts"
	LockPosts                Permission = "lockPosts"
	ModeratePosts            Permission = "moderatePosts"
	TagPosts                 Permission = "tagPosts"
	ViewPostVotes            Permission = "viewPostVotes"
)

var sessionPermissions = [...]Permission{
	ReadSettings,
	ManageProfanity,
	CreateUsers,
	ViewDesignSystem,
	ManageAPIKeys,
	ChangeOwnEmail,
	EnableEmailNotifications,
	ManageSettings,
	ManageContentSettings,
	ManagePages,
	ManagePageTopics,
	ManageNavigation,
	ManageTags,
	ManageMembers,
	ManageReports,
	ManageReportReasons,
	ManageQueue,
	ManageArchive,
	ManageResponses,
	ReadResponses,
	ManageSponsorship,
	ManageWebhooks,
	ManageAuthentication,
	ManageInvitations,
	ManageBilling,
	ManageFiles,
	ExportBackup,
	ReadProfiles,
	EditUserProfiles,
	ModerateUsers,
	DeleteUserModeration,
	ExpireUserModeration,
	BlockUsers,
	ChangeUserRoles,
	ChangeUserVisualRoles,
	ReadUserEmails,
	CreatePosts,
	EditPosts,
	DeletePosts,
	RespondToPosts,
	LockPosts,
	ModeratePosts,
	TagPosts,
	ViewPostVotes,
}

func Can(user *User, tenant *Tenant, permission Permission) bool {
	if !canAct(user, tenant) {
		return false
	}

	switch permission {
	case ManageSettings, ManageProfanity, CreateUsers, ViewDesignSystem, EnableEmailNotifications,
		ManagePageTopics, ManageNavigation, ManageAuthentication,
		ManageInvitations, ManageBilling, ManageFiles, ExportBackup, ChangeUserRoles:
		return user.IsAdministrator()

	case ReadSettings, ManageAPIKeys, ChangeOwnEmail,
		ManageContentSettings, ManagePages, ManageTags, ManageReportReasons,
		ManageArchive, ManageResponses, ManageSponsorship, ManageWebhooks,
		DeleteUserModeration, BlockUsers, ChangeUserVisualRoles, ReadUserEmails,
		LockPosts:
		return user.IsCollaborator()

	case ManageMembers, ManageReports, ReadResponses, ReadProfiles, EditUserProfiles,
		ModerateUsers, ExpireUserModeration, DeletePosts, RespondToPosts,
		ModeratePosts, ViewPostVotes:
		return user.IsCollaborator() || user.IsModerator()

	case ManageQueue, TagPosts:
		return user.IsCollaborator() || user.IsModerator() || user.IsHelper()

	case EditPosts:
		return true

	case CreatePosts:
		if user.IsMuted() {
			return false
		}

		if tenant == nil || tenant.GeneralSettings == nil {
			return true
		}

		settings := tenant.GeneralSettings
		if settings.PostingGloballyDisabled && !user.IsCollaborator() {
			return false
		}

		for _, role := range settings.PostingDisabledFor {
			if role == user.Role.String() {
				return false
			}
		}

		return true
	}

	return false
}

func canAct(user *User, tenant *Tenant) bool {
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

	return permissions
}

func CanChangeNotificationChannels(user *User, tenant *Tenant, current, next enum.NotificationChannel) bool {
	if !canAct(user, tenant) {
		return false
	}

	addsEmail := next&enum.NotificationChannelEmail != 0 && current&enum.NotificationChannelEmail == 0
	return !addsEmail || Can(user, tenant, EnableEmailNotifications)
}
