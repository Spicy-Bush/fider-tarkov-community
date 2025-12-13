package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func init() {
	bus.Register(Service{})
}

type Service struct{}

func (s Service) Name() string {
	return "PostgreSQL"
}

func (s Service) Category() string {
	return "sqlstore"
}

func (s Service) Enabled() bool {
	return true
}

func (s Service) Init() {
	bus.AddHandler(storeEvent)

	bus.AddHandler(purgeExpiredNotifications)

	bus.AddHandler(createCannedResponse)
	bus.AddHandler(updateCannedResponse)
	bus.AddHandler(deleteCannedResponse)
	bus.AddHandler(getCannedResponseByID)
	bus.AddHandler(listCannedResponses)

	bus.AddHandler(markAllNotificationsAsRead)
	bus.AddHandler(markNotificationAsRead)
	bus.AddHandler(countUnreadNotifications)
	bus.AddHandler(getNotificationByID)
	bus.AddHandler(getActiveNotifications)
	bus.AddHandler(purgeReadNotifications)
	bus.AddHandler(addNewNotification)
	bus.AddHandler(addSubscriber)
	bus.AddHandler(removeSubscriber)
	bus.AddHandler(supressEmail)
	bus.AddHandler(getActiveSubscribers)

	bus.AddHandler(getTagBySlug)
	bus.AddHandler(getAssignedTags)
	bus.AddHandler(getAllTags)
	bus.AddHandler(addNewTag)
	bus.AddHandler(updateTag)
	bus.AddHandler(deleteTag)
	bus.AddHandler(assignTag)
	bus.AddHandler(unassignTag)

	bus.AddHandler(addVote)
	bus.AddHandler(removeVote)
	bus.AddHandler(listPostVotes)

	bus.AddHandler(addNewPost)
	bus.AddHandler(updatePost)
	bus.AddHandler(getPostByID)
	bus.AddHandler(getPostBySlug)
	bus.AddHandler(getPostByNumber)
	bus.AddHandler(getUserPostCount)
	bus.AddHandler(countUntaggedPosts)
	bus.AddHandler(searchPosts)
	bus.AddHandler(getAllPosts)
	bus.AddHandler(getPostsByIDs)
	bus.AddHandler(countPostPerStatus)
	bus.AddHandler(markPostAsDuplicate)
	bus.AddHandler(setPostResponse)
	bus.AddHandler(postIsReferenced)
	bus.AddHandler(lockPost)
	bus.AddHandler(unlockPost)
	bus.AddHandler(refreshPostStats)
	bus.AddHandler(archivePost)
	bus.AddHandler(unarchivePost)
	bus.AddHandler(bulkArchivePosts)
	bus.AddHandler(getArchivablePosts)
	bus.AddHandler(countVotesSinceArchive)

	bus.AddHandler(setAttachments)
	bus.AddHandler(getAttachments)
	bus.AddHandler(uploadImage)
	bus.AddHandler(uploadImages)

	bus.AddHandler(getNameFromBlobKey)
	bus.AddHandler(isImageFileInUse)
	bus.AddHandler(getImageFile)
	bus.AddHandler(uploadImageFile)
	bus.AddHandler(renameImageFile)
	bus.AddHandler(deleteImageFile)
	bus.AddHandler(deleteImageFileReferences)
	bus.AddHandler(updateImageFileReferences)
	bus.AddHandler(listImageFiles)

	bus.AddHandler(addNewComment)
	bus.AddHandler(updateComment)
	bus.AddHandler(toggleCommentReaction)
	bus.AddHandler(getUserCommentCount)
	bus.AddHandler(deleteComment)
	bus.AddHandler(getCommentByID)
	bus.AddHandler(getCommentsByPost)

	bus.AddHandler(countUsers)
	bus.AddHandler(blockUser)
	bus.AddHandler(unblockUser)
	bus.AddHandler(regenerateAPIKey)
	bus.AddHandler(userSubscribedTo)
	bus.AddHandler(deleteCurrentUser)
	bus.AddHandler(changeUserEmail)
	bus.AddHandler(changeUserRole)
	bus.AddHandler(changeUserVisualRole)
	bus.AddHandler(updateCurrentUserSettings)
	bus.AddHandler(getCurrentUserSettings)
	bus.AddHandler(registerUser)
	bus.AddHandler(registerUserProvider)
	bus.AddHandler(updateCurrentUser)
	bus.AddHandler(updateUserAvatar)
	bus.AddHandler(getUserByAPIKey)
	bus.AddHandler(getUserByEmail)
	bus.AddHandler(getUserByID)
	bus.AddHandler(getUserByProvider)
	bus.AddHandler(getAllUserProviders)
	bus.AddHandler(getAllUsers)
	bus.AddHandler(getAllUsersNames)
	bus.AddHandler(getUserProfileStats)
	bus.AddHandler(getUserProfileStanding)
	bus.AddHandler(searchUserContent)

	bus.AddHandler(createTenant)
	bus.AddHandler(getFirstTenant)
	bus.AddHandler(getTenantByDomain)
	bus.AddHandler(getTenantProfanityWords)
	bus.AddHandler(activateTenant)
	bus.AddHandler(isSubdomainAvailable)
	bus.AddHandler(isCNAMEAvailable)
	bus.AddHandler(updateTenantSettings)
	bus.AddHandler(updateTenantPrivacySettings)
	bus.AddHandler(updateTenantEmailAuthAllowedSettings)
	bus.AddHandler(updateTenantAdvancedSettings)
	bus.AddHandler(updateGeneralSettings)
	bus.AddHandler(UpdateMessageBanner)

	bus.AddHandler(getVerificationByKey)
	bus.AddHandler(saveVerificationKey)
	bus.AddHandler(setKeyAsVerified)

	bus.AddHandler(listCustomOAuthConfig)
	bus.AddHandler(getCustomOAuthConfigByProvider)
	bus.AddHandler(saveCustomOAuthConfig)

	bus.AddHandler(getWebhook)
	bus.AddHandler(listAllWebhooks)
	bus.AddHandler(listAllWebhooksByType)
	bus.AddHandler(listActiveWebhooksByType)
	bus.AddHandler(createEditWebhook)
	bus.AddHandler(deleteWebhook)
	bus.AddHandler(markWebhookAsFailed)

	bus.AddHandler(getBillingState)
	bus.AddHandler(activateBillingSubscription)
	bus.AddHandler(cancelBillingSubscription)
	bus.AddHandler(lockExpiredTenants)
	bus.AddHandler(getTrialingTenantContacts)

	bus.AddHandler(setSystemSettings)
	bus.AddHandler(getSystemSettings)

	bus.AddHandler(muteUser)
	bus.AddHandler(warnUser)
	bus.AddHandler(deleteWarning)
	bus.AddHandler(deleteMute)
	bus.AddHandler(expireWarning)
	bus.AddHandler(expireMute)
	bus.AddHandler(getUsersToNotify)

	bus.AddHandler(updateUser)

	bus.AddHandler(createReport)
	bus.AddHandler(assignReport)
	bus.AddHandler(unassignReport)
	bus.AddHandler(resolveReport)
	bus.AddHandler(deleteReport)
	bus.AddHandler(getReportByID)
	bus.AddHandler(listReports)
	bus.AddHandler(countPendingReports)
	bus.AddHandler(getReportReasons)
	bus.AddHandler(listAllReportReasons)
	bus.AddHandler(countUserReportsToday)
	bus.AddHandler(hasUserReportedTarget)
	bus.AddHandler(getUserReportedItemsOnPost)
	bus.AddHandler(createReportReason)
	bus.AddHandler(updateReportReason)
	bus.AddHandler(deleteReportReason)
	bus.AddHandler(reorderReportReasons)

	bus.AddHandler(savePushSubscription)
	bus.AddHandler(deletePushSubscription)
	bus.AddHandler(deleteAllPushSubscriptions)
	bus.AddHandler(deletePushSubscriptionByEndpoint)
	bus.AddHandler(getPushSubscriptionsByUser)
	bus.AddHandler(getPushSubscriptionsByUsers)
	bus.AddHandler(getAllPushSubscriptions)
	bus.AddHandler(hasPushSubscription)
}

type SqlHandler func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error

func using(ctx context.Context, handler SqlHandler) error {
	trx, owned, err := dbx.GetOrBeginTx(ctx)
	if err != nil {
		return err
	}

	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	err = handler(trx, tenant, user)

	if owned {
		if err != nil {
			trx.MustRollback()
		} else {
			trx.MustCommit()
		}
	}

	return err
}
