package cmd

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/webhooks"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func routes(r *web.Engine) *web.Engine {
	r.Worker().Use(middlewares.WorkerSetup())

	r.Get("/_health", handlers.Health())

	r.Use(middlewares.CatchPanic())
	r.Use(middlewares.Instrumentation())

	r.NotFound(func(c *web.Context) error {
		mw := middlewares.Chain(
			middlewares.WebSetup(),
			middlewares.Tenant(),
			middlewares.User(),
		)
		next := mw(func(c *web.Context) error {
			return c.NotFound()
		})
		return next(c)
	})

	r.Use(middlewares.Secure())
	r.Use(middlewares.Compress())

	assets := r.Group()
	{
		assets.Use(middlewares.CORS())
		assets.Use(middlewares.ClientCache(365 * 24 * time.Hour))
		assets.Get("/static/favicon", handlers.Favicon())
		assets.Static("/assets/*filepath", "dist")
		assets.Static("/misc/*filepath", "misc")
	}

	r.Use(middlewares.Session())

	r.Get("/robots.txt", handlers.RobotsTXT())
	r.Post("/api/log-error", handlers.LogError())

	r.Use(middlewares.Maintenance())
	r.Use(middlewares.WebSetup())
	r.Use(middlewares.Tenant())

	tenantAssets := r.Group()
	{
		tenantAssets.Use(middlewares.RequireTenant())
		tenantAssets.Use(middlewares.ClientCache(5 * 24 * time.Hour))
		tenantAssets.Get("/static/avatars/letter/:id/:name", handlers.LetterAvatar())
		tenantAssets.Get("/static/avatars/gravatar/:id/*name", handlers.Gravatar())

		tenantAssets.Use(middlewares.ClientCache(30 * 24 * time.Hour))
		tenantAssets.Get("/static/custom/:md5.css", func(c *web.Context) error {
			return c.Blob(http.StatusOK, "text/css", []byte(c.Tenant().CustomCSS))
		})
	}

	r.Use(middlewares.User())
	r.Use(middlewares.FilterContext())

	imageAssets := r.Group()
	imageAssets.Use(middlewares.RequireTenant())
	imageAssets.Use(middlewares.ClientCache(30 * 24 * time.Hour))
	imageAssets.Get("/static/favicon/*bkey", handlers.Favicon())
	imageAssets.Get("/static/images/*bkey", handlers.ViewUploadedImage())

	if env.IsBillingEnabled() {
		wh := r.Group()
		{
			wh.Post("/webhooks/paddle", webhooks.IncomingPaddleWebhook())
		}
	}

	r.Use(middlewares.CSRF())

	r.Get("/terms", handlers.LegalPage("Terms of Service", "terms.md"))
	r.Get("/privacy", handlers.LegalPage("Privacy Policy", "privacy.md"))
	r.Get("/advertise", handlers.AdvertisePage())
	r.Get("/ads/click/:id", handlers.SponsorshipClick())

	r.Post("/api/tenants", handlers.CreateTenant())
	r.Get("/api/tenants/:subdomain/availability", handlers.CheckAvailability())
	r.Get("/signup", handlers.SignUp())
	r.Get("/oauth/:provider", handlers.SignInByOAuth())
	r.Get("/oauth/:provider/callback", handlers.OAuthCallback())

	// Starting from this step, a Tenant is required
	r.Use(middlewares.RequireTenant())

	r.Get("/sitemap.xml", handlers.Sitemap())

	pwa := r.Group()
	{
		pwa.Get("/manifest.json", handlers.Manifest())
		pwa.Get("/sw.js", handlers.ServiceWorker())
		pwa.Get("/api/push/vapid-key", handlers.GetVAPIDPublicKey())
	}

	r.Get("/signup/verify", handlers.VerifySignUpKey())
	r.Get("/signout", handlers.SignOut())
	r.Get("/oauth/:provider/token", handlers.OAuthToken())
	r.Get("/oauth/:provider/echo", handlers.OAuthEcho())

	// If tenant is pending, block it from using any other route
	r.Use(middlewares.BlockPendingTenants())

	r.Get("/signin", handlers.SignInPage())
	r.Get("/not-invited", handlers.NotInvitedPage())
	r.Get("/signin/verify", handlers.VerifySignInKey(enum.EmailVerificationKindSignIn))
	r.Get("/invite/verify", handlers.VerifySignInKey(enum.EmailVerificationKindUserInvitation))
	r.Post("/api/signin/complete", handlers.CompleteSignInProfile())
	r.Post("/api/signin", handlers.SignInByEmail())

	// Block if it's private tenant with unauthenticated user
	r.Use(middlewares.CheckTenantPrivacy())

	r.Get("/", handlers.Index())
	r.Get("/posts/:number", handlers.PostDetails())
	r.Get("/posts/:number/:slug", handlers.PostDetails())
	r.Get("/pages", handlers.ListPagesPage())
	r.Get("/pages/:slug", handlers.ViewPage())

	// Does not require authentication
	publicApi := r.Group()
	{
		publicApi.Get("/api/posts", api.SearchPosts())
		publicApi.Get("/api/tags", api.ListTags())
		publicApi.Get("/api/posts/:number", api.GetPost())
		publicApi.Get("/api/posts/:number/comments", api.ListDiscussion())
		publicApi.Get("/api/comments/:id", api.ReadDiscussionComment())
		publicApi.Get("/api/posts/:number/attachments", api.GetPostAttachments())
		publicApi.Get("/api/pages", api.SearchPages())
		publicApi.Get("/api/pages/:id/comments", api.ListDiscussion())
		publicApi.Post("/api/ads/select", api.SelectAds())
		publicApi.Get("/api/ads/placement-config", api.PublicAdPlacementConfig())
	}

	// Available to any authenticated user
	membersApi := r.Group()
	{
		membersApi.Use(middlewares.IsAuthenticated())
		membersApi.Use(middlewares.BlockLockedTenants())

		// user settings
		membersApi.Get("/profile", handlers.UserProfile())
		membersApi.Get("/api/user/moderation", handlers.ProfileModerationStatus())
		membersApi.Get("/api/user/moderation/avatar", handlers.PreviewProfileAvatar())
		membersApi.Post("/api/user/name", handlers.UpdateUserName())
		membersApi.Post("/api/user/avatar", handlers.UpdateUserAvatar())
		membersApi.Post("/api/user/settings", handlers.UpdateUserSettings())
		membersApi.Get("/change-email/verify", handlers.VerifyChangeEmailKey())
		membersApi.Post("/api/user/regenerate-apikey", handlers.RegenerateAPIKey())
		membersApi.Post("/api/user/change-email", handlers.ChangeUserEmail())
		membersApi.Delete("/api/user", handlers.DeleteUser())
		membersApi.Get("/api/user/profile/:userID/content/search", api.SearchUserContent()) // 'Visitors' users can only search their own content
		membersApi.Get("/api/user/profile/:userID/stats", api.GetUserProfileStats())        // 'Visitors' users can only view their own stats
		membersApi.Get("/api/user/profile/:userID/standing", api.GetUserProfileStanding())  // 'Visitors' users can only view their own standing

		// notifications
		membersApi.Get("/notifications", handlers.Notifications())
		membersApi.Get("/notifications/:id", handlers.ReadNotification())
		membersApi.Get("/api/notifications", handlers.GetAllNotifications())
		membersApi.Get("/api/notifications/unread/total", handlers.TotalUnreadNotifications())
		membersApi.Post("/api/notifications/purge-read", handlers.PurgeReadNotifications())
		membersApi.Post("/api/notifications/read-all", handlers.ReadAllNotifications())
		membersApi.Post("/api/notifications/read/:id", handlers.MarkNotificationAsRead())

		// push notifications
		membersApi.Post("/api/push/subscribe", handlers.SavePushSubscription())
		membersApi.Delete("/api/push/subscribe", handlers.DeletePushSubscription())
		membersApi.Get("/api/push/status", handlers.HasPushSubscription())

		// posting
		membersApi.Post("/api/posts", api.CreatePost())
		membersApi.Put("/api/posts/:number", api.UpdatePost())

		// comments
		membersApi.Post("/api/posts/:number/comments", api.CreateDiscussionComment())
		membersApi.Put("/api/comments/:id", api.EditDiscussionComment())
		membersApi.Delete("/api/comments/:id", api.DeleteDiscussionComment())
		membersApi.Put("/api/comments/:id/reactions/:reaction", api.ReactToDiscussionComment())
		membersApi.Get("/api/taggable-users", api.ListTaggableUsers())

		// voting
		membersApi.Post("/api/posts/:number/up", api.AddVote())
		membersApi.Post("/api/posts/:number/down", api.AddDownVote())
		membersApi.Delete("/api/posts/:number/votes", api.RemoveVote())
		membersApi.Post("/api/posts/:number/subscription", api.Subscribe())
		membersApi.Delete("/api/posts/:number/subscription", api.Unsubscribe())

		membersApi.Post("/api/posts/:number/report", handlers.ReportPost())
		membersApi.Post("/api/comments/:id/report", handlers.ReportComment())
		membersApi.Get("/api/report-reasons", handlers.GetReportReasons())

		membersApi.Post("/api/pages/:id/reactions", api.TogglePageReaction())
		membersApi.Post("/api/pages/:id/subscribe", api.TogglePageSubscription())
		membersApi.Post("/api/pages/:id/comments", api.CreateDiscussionComment())
	}

	queue := membersApi.Group()
	{
		queue.Use(middlewares.RequirePermission(entity.ManageQueue))

		queue.Get("/admin/queue", handlers.PostQueuePage())
		queue.Post("/api/queue/:id/heartbeat", handlers.QueuePostHeartbeat())
		queue.Delete("/api/mod/queue-viewing", handlers.StopViewingQueuePost())
		queue.Get("/api/mod/queue-events", handlers.QueueSSE())
	}

	postTags := membersApi.Group()
	{
		postTags.Use(middlewares.RequirePermission(entity.TagPosts))

		postTags.Post("/api/posts/:number/tags/:slug", api.AssignTag())
		postTags.Delete("/api/posts/:number/tags/:slug", api.UnassignTag())
	}

	profiles := membersApi.Group()
	{
		profiles.Use(middlewares.RequirePermission(entity.ReadProfiles))

		profiles.Get("/profile/:id", handlers.ViewUserProfile())
	}

	profileEdits := membersApi.Group()
	{
		profileEdits.Use(middlewares.RequirePermission(entity.EditUserProfiles))

		profileEdits.Post("/api/users/:userID/name", handlers.UpdateUserName())
		profileEdits.Post("/api/users/:userID/avatar", handlers.UpdateUserAvatar())
	}

	userModeration := membersApi.Group()
	{
		userModeration.Use(middlewares.RequirePermission(entity.ModerateUsers))

		userModeration.Post("/api/admin/users/:userID/mute", handlers.MuteUser())
		userModeration.Post("/api/admin/users/:userID/warn", handlers.WarnUser())
	}

	expireModeration := membersApi.Group()
	{
		expireModeration.Use(middlewares.RequirePermission(entity.ExpireUserModeration))

		expireModeration.Post("/api/admin/users/:userID/warnings/:warningID/expire", handlers.ExpireWarning())
		expireModeration.Post("/api/admin/users/:userID/mutes/:muteID/expire", handlers.ExpireMute())
	}

	readResponses := membersApi.Group()
	{
		readResponses.Use(middlewares.RequirePermission(entity.ReadResponses))

		readResponses.Get("/api/responses/:type", api.ListCannedResponses())
	}

	members := membersApi.Group()
	{
		members.Use(middlewares.RequirePermission(entity.ManageMembers))

		members.Get("/admin/members", handlers.ManageMembers())
		members.Get("/api/users", api.ListUsers())
	}

	postVotes := membersApi.Group()
	{
		postVotes.Use(middlewares.RequirePermission(entity.ViewPostVotes))

		postVotes.Get("/api/posts/:number/votes", api.ListVotes())
	}

	deletePosts := membersApi.Group()
	{
		deletePosts.Use(middlewares.RequirePermission(entity.DeletePosts))

		deletePosts.Delete("/api/posts/:number", api.DeletePost())
	}

	postResponses := membersApi.Group()
	{
		postResponses.Use(middlewares.RequirePermission(entity.RespondToPosts))

		postResponses.Put("/api/posts/:number/status", api.SetResponse())
	}

	reports := membersApi.Group()
	{
		reports.Use(middlewares.RequirePermission(entity.ManageReports))

		reports.Get("/admin/reports", handlers.ManageReportsPage())
		reports.Get("/api/reports", handlers.ListReports())
		reports.Get("/api/reports/:id", handlers.GetReport())
		reports.Get("/api/reports/:id/details", handlers.GetReportDetails())
		reports.Post("/api/reports/:id/assign", handlers.AssignReport())
		reports.Delete("/api/reports/:id/assign", handlers.UnassignReport())
		reports.Put("/api/reports/:id/resolve", handlers.ResolveReport())
		reports.Post("/api/reports/:id/heartbeat", handlers.ReportHeartbeat())
		reports.Delete("/api/mod/viewing", handlers.StopViewingReport())
		reports.Get("/api/mod/report-events", handlers.ReportsSSE())
	}

	contentModeration := membersApi.Group()
	{
		contentModeration.Use(middlewares.RequirePermission(entity.ModeratePosts))

		contentModeration.Get("/api/admin/moderation/checks", handlers.ListModerationChecks())
		contentModeration.Post("/api/admin/moderation/retry", handlers.RetryModerationChecks())
		contentModeration.Post("/api/admin/moderation/posts/:id/approve", handlers.ApprovePostModeration())
		contentModeration.Post("/api/admin/moderation/comments/:id/approve", handlers.ApproveCommentModeration())
		contentModeration.Post("/api/admin/moderation/posts/:id/hide", handlers.HidePostModeration())
		contentModeration.Post("/api/admin/moderation/comments/:id/hide", handlers.HideCommentModeration())
	}

	readSettings := membersApi.Group()
	{
		readSettings.Use(middlewares.SetLocale("en"))
		readSettings.Use(middlewares.RequirePermission(entity.ReadSettings))

		readSettings.Get("/admin", handlers.GeneralSettingsPage())
	}

	contentSettings := membersApi.Group()
	{
		contentSettings.Use(middlewares.SetLocale("en"))
		contentSettings.Use(middlewares.RequirePermission(entity.ManageContentSettings))

		contentSettings.Get("/admin/content-settings", handlers.ContentSettingsPage())
		contentSettings.Post("/api/admin/settings/content-settings", handlers.UpdateContentSettings())
		contentSettings.Post("/api/admin/settings/message-banner", handlers.UpdateMessageBanner())
	}

	responses := membersApi.Group()
	{
		responses.Use(middlewares.SetLocale("en"))
		responses.Use(middlewares.RequirePermission(entity.ManageResponses))

		responses.Get("/admin/responses", handlers.ManageCannedResponses())
		responses.Post("/api/responses", api.CreateCannedResponse())
		responses.Put("/api/responses/:id", api.UpdateCannedResponse())
		responses.Delete("/api/responses/:id", api.DeleteCannedResponse())
	}

	reportReasons := membersApi.Group()
	{
		reportReasons.Use(middlewares.SetLocale("en"))
		reportReasons.Use(middlewares.RequirePermission(entity.ManageReportReasons))

		reportReasons.Get("/api/report-reasons/all", handlers.ListAllReportReasons())
		reportReasons.Post("/api/report-reasons", handlers.CreateReportReason())
		reportReasons.Put("/api/report-reasons/:id", handlers.UpdateReportReason())
		reportReasons.Delete("/api/report-reasons/:id", handlers.DeleteReportReason())
		reportReasons.Put("/api/admin/report-reasons-order", handlers.ReorderReportReasons())
	}

	archive := membersApi.Group()
	{
		archive.Use(middlewares.SetLocale("en"))
		archive.Use(middlewares.RequirePermission(entity.ManageArchive))

		archive.Get("/admin/archive", handlers.ArchivePostsPage())
		archive.Get("/api/archive/posts", handlers.ListArchivablePosts())
		archive.Post("/api/posts/:number/archive", handlers.ArchivePost())
		archive.Post("/api/posts/:number/unarchive", handlers.UnarchivePost())
		archive.Post("/api/archive/bulk", handlers.BulkArchive())
	}

	pages := membersApi.Group()
	{
		pages.Use(middlewares.SetLocale("en"))
		pages.Use(middlewares.RequirePermission(entity.ManagePages))

		pages.Get("/admin/pages", handlers.ManagePages())
		pages.Get("/admin/pages/new", handlers.EditPagePage())
		pages.Get("/admin/pages/edit/:id", handlers.EditPagePage())
		pages.Post("/api/pages", api.CreatePage())
		pages.Put("/api/pages/:id", api.UpdatePage())
		pages.Delete("/api/pages/:id", api.DeletePage())
		pages.Post("/api/pages/:id/draft", api.SavePageDraft())
		pages.Get("/api/pages/:id/draft", api.GetPageDraft())
	}

	tags := membersApi.Group()
	{
		tags.Use(middlewares.SetLocale("en"))
		tags.Use(middlewares.RequirePermission(entity.ManageTags))

		tags.Get("/admin/tags", handlers.ManageTags())
		tags.Post("/api/tags", api.CreateEditTag())
		tags.Put("/api/tags/:slug", api.CreateEditTag())
		tags.Delete("/api/tags/:slug", api.DeleteTag())
	}

	sponsorship := membersApi.Group()
	{
		sponsorship.Use(middlewares.SetLocale("en"))
		sponsorship.Use(middlewares.RequirePermission(entity.ManageSponsorship))

		sponsorship.Get("/admin/sponsorship", handlers.ManageSponsorshipPage())
		sponsorship.Get("/api/sponsorship/packages", api.ListSponsorshipPackages())
		sponsorship.Post("/api/sponsorship/packages", api.CreateSponsorshipPackage())
		sponsorship.Put("/api/sponsorship/packages/:id", api.UpdateSponsorshipPackage())
		sponsorship.Delete("/api/sponsorship/packages/:id", api.DeleteSponsorshipPackage())
		sponsorship.Get("/api/sponsorship/campaigns", api.ListSponsorshipCampaigns())
		sponsorship.Post("/api/sponsorship/campaigns", api.CreateSponsorshipCampaign())
		sponsorship.Put("/api/sponsorship/campaigns/:id", api.UpdateSponsorshipCampaign())
		sponsorship.Put("/api/sponsorship/campaigns/:id/graph", api.SaveCampaignGraph())
		sponsorship.Delete("/api/sponsorship/campaigns/:id", api.DeleteSponsorshipCampaign())
		sponsorship.Get("/api/ads/placements", api.ListAdPlacements())
		sponsorship.Put("/api/ads/placements/:id", api.UpdateAdPlacement())
		sponsorship.Get("/api/sponsorship/campaigns/:id/versions", api.ListCreativeVersions())
		sponsorship.Post("/api/sponsorship/campaigns/:id/versions", api.CreateCreativeVersion())
		sponsorship.Get("/api/sponsorship/campaigns/:id/assignments", api.ListCampaignAssignments())
	}

	webhooks := membersApi.Group()
	{
		webhooks.Use(middlewares.SetLocale("en"))
		webhooks.Use(middlewares.RequirePermission(entity.ManageWebhooks))

		webhooks.Get("/admin/webhooks", handlers.ManageWebhooks())
		webhooks.Post("/api/admin/webhook", handlers.CreateWebhook())
		webhooks.Put("/api/admin/webhook/:id", handlers.UpdateWebhook())
		webhooks.Delete("/api/admin/webhook/:id", handlers.DeleteWebhook())
		webhooks.Get("/api/admin/webhook/test/:id", handlers.TestWebhook())
		webhooks.Post("/api/admin/webhook/preview", handlers.PreviewWebhook())
		webhooks.Get("/api/admin/webhook/props/:type", handlers.GetWebhookProps())
	}

	visualRoles := membersApi.Group()
	{
		visualRoles.Use(middlewares.SetLocale("en"))
		visualRoles.Use(middlewares.RequirePermission(entity.ChangeUserVisualRoles))

		visualRoles.Post("/api/admin/visualroles/:visualRole/users", handlers.ChangeUserVisualRole())
	}

	blocks := membersApi.Group()
	{
		blocks.Use(middlewares.SetLocale("en"))
		blocks.Use(middlewares.RequirePermission(entity.BlockUsers))

		blocks.Put("/api/admin/users/:userID/block", handlers.BlockUser())
		blocks.Delete("/api/admin/users/:userID/block", handlers.UnblockUser())
	}

	deleteModeration := membersApi.Group()
	{
		deleteModeration.Use(middlewares.SetLocale("en"))
		deleteModeration.Use(middlewares.RequirePermission(entity.DeleteUserModeration))

		deleteModeration.Delete("/api/admin/users/:userID/warnings/:warningID", handlers.DeleteWarning())
		deleteModeration.Delete("/api/admin/users/:userID/mutes/:muteID", handlers.DeleteMute())
	}

	postLocks := membersApi.Group()
	{
		postLocks.Use(middlewares.SetLocale("en"))
		postLocks.Use(middlewares.RequirePermission(entity.LockPosts))

		postLocks.Put("/api/posts/:number/lock", api.LockOrUnlockPost())
		postLocks.Delete("/api/posts/:number/lock", api.LockOrUnlockPost())
	}

	settings := membersApi.Group()
	{
		settings.Use(middlewares.SetLocale("en"))
		settings.Use(middlewares.RequirePermission(entity.ManageSettings))

		settings.Post("/api/admin/settings/general", handlers.UpdateSettings()) // General Page
		settings.Get("/admin/privacy", handlers.Page("Privacy · Site Settings", "", "Administration/pages/PrivacySettings.page"))
		settings.Post("/api/admin/settings/privacy", handlers.UpdatePrivacy())
		settings.Get("/admin/advanced", handlers.AdvancedSettingsPage())
		settings.Post("/api/admin/settings/advanced", handlers.UpdateAdvancedSettings())
	}

	pageTopics := membersApi.Group()
	{
		pageTopics.Use(middlewares.SetLocale("en"))
		pageTopics.Use(middlewares.RequirePermission(entity.ManagePageTopics))

		pageTopics.Post("/api/page-topics", api.CreatePageTopic())
		pageTopics.Put("/api/page-topics/:id", api.UpdatePageTopic())
		pageTopics.Delete("/api/page-topics/:id", api.DeletePageTopic())
		pageTopics.Post("/api/page-tags", api.CreatePageTag())
		pageTopics.Put("/api/page-tags/:id", api.UpdatePageTag())
		pageTopics.Delete("/api/page-tags/:id", api.DeletePageTag())
	}

	navigation := membersApi.Group()
	{
		navigation.Use(middlewares.SetLocale("en"))
		navigation.Use(middlewares.RequirePermission(entity.ManageNavigation))

		navigation.Post("/api/admin/navigation", api.SaveNavigationLinks())
	}

	profanity := membersApi.Group()
	{
		profanity.Use(middlewares.SetLocale("en"))
		profanity.Use(middlewares.RequirePermission(entity.ManageProfanity))

		profanity.Post("/api/admin/settings/profanity", handlers.UpdateProfanityWords())
	}

	invitations := membersApi.Group()
	{
		invitations.Use(middlewares.SetLocale("en"))
		invitations.Use(middlewares.RequirePermission(entity.ManageInvitations))

		invitations.Get("/admin/invitations", handlers.Page("Invitations · Site Settings", "", "Administration/pages/Invitations.page"))
		invitations.Post("/api/invitations/send", api.SendInvites())
		invitations.Post("/api/invitations/sample", api.SendSampleInvite())
	}

	authentication := membersApi.Group()
	{
		authentication.Use(middlewares.SetLocale("en"))
		authentication.Use(middlewares.RequirePermission(entity.ManageAuthentication))

		authentication.Get("/admin/authentication", handlers.ManageAuthentication())
		authentication.Post("/api/admin/oauth", handlers.SaveOAuthConfig())
		authentication.Get("/api/admin/oauth/:provider", handlers.GetOAuthConfig())
		authentication.Post("/api/admin/settings/emailauth", handlers.UpdateEmailAuthAllowed())
	}

	if env.IsBillingEnabled() {
		billing := membersApi.Group()
		{
			billing.Use(middlewares.SetLocale("en"))
			billing.Use(middlewares.RequirePermission(entity.ManageBilling))

			billing.Get("/admin/billing", handlers.ManageBilling())
			billing.Post("/api/billing/checkout-link", handlers.GenerateCheckoutLink())
		}
	}

	files := membersApi.Group()
	{
		files.Use(middlewares.SetLocale("en"))
		files.Use(middlewares.RequirePermission(entity.ManageFiles))

		files.Get("/admin/files", handlers.FileManagementPage())
		files.Get("/api/admin/files", handlers.ListFiles())
		files.Post("/api/admin/files", handlers.UploadFile())
		files.Post("/api/admin/files-bulk/delete", handlers.BulkDeleteFiles())
		files.Get("/api/admin/files-bulk/prunable-count", handlers.GetPrunableFilesCount())
		files.Post("/api/admin/files-bulk/prune", handlers.PruneUnusedFiles())
		files.Put("/api/admin/files/:blobKey/*path", handlers.RenameFile())
		files.Delete("/api/admin/files/:blobKey/*path", handlers.DeleteFile())
		files.Get("/api/admin/files/:blobKey/usage/*path", handlers.GetFileUsage())
	}

	createUsers := membersApi.Group()
	{
		createUsers.Use(middlewares.SetLocale("en"))
		createUsers.Use(middlewares.RequirePermission(entity.CreateUsers))

		createUsers.Post("/api/users", api.CreateUser())
	}

	roles := membersApi.Group()
	{
		roles.Use(middlewares.SetLocale("en"))
		roles.Use(middlewares.RequirePermission(entity.ChangeUserRoles))

		roles.Post("/api/admin/roles/:role/users", handlers.ChangeUserRole())
	}

	exports := membersApi.Group()
	{
		exports.Use(middlewares.SetLocale("en"))
		exports.Use(middlewares.RequirePermission(entity.ExportBackup))

		exports.Get("/admin/export", handlers.Page("Export · Site Settings", "", "Administration/pages/Export.page"))
		exports.Get("/admin/export/posts.csv", handlers.ExportPostsToCSV())
		exports.Get("/admin/export/backup.zip", handlers.ExportBackupZip())
	}

	designSystem := membersApi.Group()
	{
		designSystem.Use(middlewares.SetLocale("en"))
		designSystem.Use(middlewares.RequirePermission(entity.ViewDesignSystem))

		designSystem.Get("/_design", handlers.Page("Design System", "A preview of Fider UI elements", "DesignSystem/DesignSystem.page"))
	}

	return r
}
