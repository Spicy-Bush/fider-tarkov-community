package cmd

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/webhooks"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
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

	helper := r.Group()
	{
		helper.Use(middlewares.IsAuthenticated())
		helper.Use(middlewares.IsAuthorized(enum.RoleHelper, enum.RoleCollaborator, enum.RoleAdministrator, enum.RoleModerator))
		helper.Use(middlewares.BlockLockedTenants())

		// post queue
		helper.Get("/admin/queue", handlers.PostQueuePage())
		helper.Post("/api/queue/:id/heartbeat", handlers.QueuePostHeartbeat())
		helper.Delete("/api/mod/queue-viewing", handlers.StopViewingQueuePost())
		helper.Get("/api/mod/queue-events", handlers.QueueSSE())

		// tags
		helper.Post("/api/posts/:number/tags/:slug", api.AssignTag())
		helper.Delete("/api/posts/:number/tags/:slug", api.UnassignTag())
	}

	// Available to both collaborators, administrators and moderators
	staff := r.Group()
	{
		staff.Use(middlewares.IsAuthenticated())
		staff.Use(middlewares.IsAuthorized(enum.RoleCollaborator, enum.RoleAdministrator, enum.RoleModerator))
		staff.Use(middlewares.BlockLockedTenants())

		// user profiles
		staff.Get("/profile/:id", handlers.ViewUserProfile())
		staff.Post("/api/users/:userID/name", handlers.UpdateUserName())
		staff.Post("/api/users/:userID/avatar", handlers.UpdateUserAvatar())

		// user moderation
		staff.Post("/api/admin/users/:userID/mute", handlers.MuteUser())
		staff.Post("/api/admin/users/:userID/warn", handlers.WarnUser())
		staff.Post("/api/admin/users/:userID/warnings/:warningID/expire", handlers.ExpireWarning())
		staff.Post("/api/admin/users/:userID/mutes/:muteID/expire", handlers.ExpireMute())
		staff.Get("/api/responses/:type", api.ListCannedResponses())
		staff.Get("/admin/members", handlers.ManageMembers())
		staff.Get("/api/users", api.ListUsers())

		// posts
		staff.Get("/api/posts/:number/votes", api.ListVotes())
		staff.Delete("/api/posts/:number", api.DeletePost())
		staff.Put("/api/posts/:number/status", api.SetResponse())

		// reports
		staff.Get("/admin/reports", handlers.ManageReportsPage())
		staff.Get("/api/admin/moderation/checks", handlers.ListModerationChecks())
		staff.Post("/api/admin/moderation/retry", handlers.RetryModerationChecks())
		staff.Get("/api/reports", handlers.ListReports())
		staff.Get("/api/reports/:id", handlers.GetReport())
		staff.Get("/api/reports/:id/details", handlers.GetReportDetails())
		staff.Post("/api/reports/:id/assign", handlers.AssignReport())
		staff.Delete("/api/reports/:id/assign", handlers.UnassignReport())
		staff.Put("/api/reports/:id/resolve", handlers.ResolveReport())
		staff.Post("/api/reports/:id/heartbeat", handlers.ReportHeartbeat())
		staff.Delete("/api/mod/viewing", handlers.StopViewingReport())
		staff.Get("/api/mod/report-events", handlers.ReportsSSE())

		// content moderation
		staff.Post("/api/admin/moderation/posts/:id/approve", handlers.ApprovePostModeration())
		staff.Post("/api/admin/moderation/comments/:id/approve", handlers.ApproveCommentModeration())
		staff.Post("/api/admin/moderation/posts/:id/hide", handlers.HidePostModeration())
		staff.Post("/api/admin/moderation/comments/:id/hide", handlers.HideCommentModeration())
	}

	// Operations available only to collaborators and administrators
	collabAdmin := r.Group()
	{
		collabAdmin.Use(middlewares.SetLocale("en"))
		collabAdmin.Use(middlewares.IsAuthenticated())
		collabAdmin.Use(middlewares.IsAuthorized(enum.RoleCollaborator, enum.RoleAdministrator))
		collabAdmin.Use(middlewares.BlockLockedTenants())

		// admin pages
		collabAdmin.Get("/admin", handlers.GeneralSettingsPage())

		collabAdmin.Get("/admin/content-settings", handlers.ContentSettingsPage())
		collabAdmin.Post("/api/admin/settings/content-settings", handlers.UpdateContentSettings())

		collabAdmin.Post("/api/admin/settings/message-banner", handlers.UpdateMessageBanner())

		collabAdmin.Get("/admin/responses", handlers.ManageCannedResponses())
		collabAdmin.Post("/api/responses", api.CreateCannedResponse())
		collabAdmin.Put("/api/responses/:id", api.UpdateCannedResponse())
		collabAdmin.Delete("/api/responses/:id", api.DeleteCannedResponse())

		collabAdmin.Get("/api/report-reasons/all", handlers.ListAllReportReasons())
		collabAdmin.Post("/api/report-reasons", handlers.CreateReportReason())

		collabAdmin.Get("/admin/archive", handlers.ArchivePostsPage())
		collabAdmin.Get("/api/archive/posts", handlers.ListArchivablePosts())
		collabAdmin.Post("/api/posts/:number/archive", handlers.ArchivePost())
		collabAdmin.Post("/api/posts/:number/unarchive", handlers.UnarchivePost())

		collabAdmin.Get("/admin/pages", handlers.ManagePages())
		collabAdmin.Get("/admin/pages/new", handlers.EditPagePage())
		collabAdmin.Get("/admin/pages/edit/:id", handlers.EditPagePage())
		collabAdmin.Post("/api/pages", api.CreatePage())
		collabAdmin.Put("/api/pages/:id", api.UpdatePage())
		collabAdmin.Delete("/api/pages/:id", api.DeletePage())
		collabAdmin.Post("/api/pages/:id/draft", api.SavePageDraft())
		collabAdmin.Get("/api/pages/:id/draft", api.GetPageDraft())
		collabAdmin.Post("/api/archive/bulk", handlers.BulkArchive())
		collabAdmin.Put("/api/report-reasons/:id", handlers.UpdateReportReason())
		collabAdmin.Delete("/api/report-reasons/:id", handlers.DeleteReportReason())
		collabAdmin.Put("/api/admin/report-reasons-order", handlers.ReorderReportReasons())

		collabAdmin.Get("/admin/tags", handlers.ManageTags())
		collabAdmin.Get("/admin/sponsorship", handlers.ManageSponsorshipPage())
		collabAdmin.Get("/api/sponsorship/packages", api.ListSponsorshipPackages())
		collabAdmin.Post("/api/sponsorship/packages", api.CreateSponsorshipPackage())
		collabAdmin.Put("/api/sponsorship/packages/:id", api.UpdateSponsorshipPackage())
		collabAdmin.Delete("/api/sponsorship/packages/:id", api.DeleteSponsorshipPackage())
		collabAdmin.Get("/api/sponsorship/campaigns", api.ListSponsorshipCampaigns())
		collabAdmin.Post("/api/sponsorship/campaigns", api.CreateSponsorshipCampaign())
		collabAdmin.Put("/api/sponsorship/campaigns/:id", api.UpdateSponsorshipCampaign())
		collabAdmin.Put("/api/sponsorship/campaigns/:id/graph", api.SaveCampaignGraph())
		collabAdmin.Delete("/api/sponsorship/campaigns/:id", api.DeleteSponsorshipCampaign())
		collabAdmin.Get("/api/ads/placements", api.ListAdPlacements())
		collabAdmin.Put("/api/ads/placements/:id", api.UpdateAdPlacement())
		collabAdmin.Get("/api/sponsorship/campaigns/:id/versions", api.ListCreativeVersions())
		collabAdmin.Post("/api/sponsorship/campaigns/:id/versions", api.CreateCreativeVersion())
		collabAdmin.Get("/api/sponsorship/campaigns/:id/assignments", api.ListCampaignAssignments())
		collabAdmin.Post("/api/tags", api.CreateEditTag())
		collabAdmin.Put("/api/tags/:slug", api.CreateEditTag())
		collabAdmin.Delete("/api/tags/:slug", api.DeleteTag())

		collabAdmin.Get("/admin/webhooks", handlers.ManageWebhooks())
		collabAdmin.Post("/api/admin/webhook", handlers.CreateWebhook())
		collabAdmin.Put("/api/admin/webhook/:id", handlers.UpdateWebhook())
		collabAdmin.Delete("/api/admin/webhook/:id", handlers.DeleteWebhook())
		collabAdmin.Get("/api/admin/webhook/test/:id", handlers.TestWebhook())
		collabAdmin.Post("/api/admin/webhook/preview", handlers.PreviewWebhook())
		collabAdmin.Get("/api/admin/webhook/props/:type", handlers.GetWebhookProps())

		// user moderation
		collabAdmin.Post("/api/admin/visualroles/:visualRole/users", handlers.ChangeUserVisualRole())

		collabAdmin.Put("/api/admin/users/:userID/block", handlers.BlockUser())
		collabAdmin.Delete("/api/admin/users/:userID/block", handlers.UnblockUser())

		collabAdmin.Delete("/api/admin/users/:userID/warnings/:warningID", handlers.DeleteWarning())
		collabAdmin.Delete("/api/admin/users/:userID/mutes/:muteID", handlers.DeleteMute())

		collabAdmin.Put("/api/posts/:number/lock", api.LockOrUnlockPost())
		collabAdmin.Delete("/api/posts/:number/lock", api.LockOrUnlockPost())
	}

	// Only available to administrators
	adminOnly := r.Group()
	{
		adminOnly.Use(middlewares.SetLocale("en"))
		adminOnly.Use(middlewares.IsAuthenticated())
		adminOnly.Use(middlewares.IsAuthorized(enum.RoleAdministrator))
		adminOnly.Use(middlewares.BlockLockedTenants())

		// admin pages
		adminOnly.Post("/api/admin/settings/general", handlers.UpdateSettings()) // General Page

		adminOnly.Get("/admin/privacy", handlers.Page("Privacy · Site Settings", "", "Administration/pages/PrivacySettings.page"))
		adminOnly.Post("/api/admin/settings/privacy", handlers.UpdatePrivacy())

		adminOnly.Get("/admin/advanced", handlers.AdvancedSettingsPage())
		adminOnly.Post("/api/admin/settings/advanced", handlers.UpdateAdvancedSettings())

		adminOnly.Post("/api/page-topics", api.CreatePageTopic())
		adminOnly.Put("/api/page-topics/:id", api.UpdatePageTopic())
		adminOnly.Delete("/api/page-topics/:id", api.DeletePageTopic())
		adminOnly.Post("/api/page-tags", api.CreatePageTag())
		adminOnly.Put("/api/page-tags/:id", api.UpdatePageTag())
		adminOnly.Delete("/api/page-tags/:id", api.DeletePageTag())
		adminOnly.Post("/api/admin/navigation", api.SaveNavigationLinks())
		adminOnly.Post("/api/admin/settings/profanity", handlers.UpdateProfanityWords())

		adminOnly.Get("/admin/invitations", handlers.Page("Invitations · Site Settings", "", "Administration/pages/Invitations.page"))
		adminOnly.Post("/api/invitations/send", api.SendInvites())
		adminOnly.Post("/api/invitations/sample", api.SendSampleInvite())

		adminOnly.Get("/admin/authentication", handlers.ManageAuthentication())
		adminOnly.Post("/api/admin/oauth", handlers.SaveOAuthConfig())
		adminOnly.Get("/api/admin/oauth/:provider", handlers.GetOAuthConfig())
		adminOnly.Post("/api/admin/settings/emailauth", handlers.UpdateEmailAuthAllowed())

		if env.IsBillingEnabled() {
			adminOnly.Get("/admin/billing", handlers.ManageBilling())
			adminOnly.Post("/api/billing/checkout-link", handlers.GenerateCheckoutLink())
		}

		adminOnly.Get("/admin/files", handlers.FileManagementPage())
		adminOnly.Get("/api/admin/files", handlers.ListFiles())
		adminOnly.Post("/api/admin/files", handlers.UploadFile())
		adminOnly.Post("/api/admin/files-bulk/delete", handlers.BulkDeleteFiles())
		adminOnly.Get("/api/admin/files-bulk/prunable-count", handlers.GetPrunableFilesCount())
		adminOnly.Post("/api/admin/files-bulk/prune", handlers.PruneUnusedFiles())
		adminOnly.Put("/api/admin/files/:blobKey/*path", handlers.RenameFile())
		adminOnly.Delete("/api/admin/files/:blobKey/*path", handlers.DeleteFile())
		adminOnly.Get("/api/admin/files/:blobKey/usage/*path", handlers.GetFileUsage())

		// user management
		adminOnly.Post("/api/users", api.CreateUser())
		adminOnly.Post("/api/admin/roles/:role/users", handlers.ChangeUserRole())

		// export
		adminOnly.Get("/admin/export", handlers.Page("Export · Site Settings", "", "Administration/pages/Export.page"))
		adminOnly.Get("/admin/export/posts.csv", handlers.ExportPostsToCSV())
		adminOnly.Get("/admin/export/backup.zip", handlers.ExportBackupZip())

		// dev
		adminOnly.Get("/_design", handlers.Page("Design System", "A preview of Fider UI elements", "DesignSystem/DesignSystem.page"))
	}

	return r
}
