package cmd

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/apiv1"
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
	r.Post("/_api/log-error", handlers.LogError())

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
		tenantAssets.Get("/static/favicon/*bkey", handlers.Favicon())
		tenantAssets.Get("/static/images/*bkey", handlers.ViewUploadedImage())
		tenantAssets.Get("/static/custom/:md5.css", func(c *web.Context) error {
			return c.Blob(http.StatusOK, "text/css", []byte(c.Tenant().CustomCSS))
		})
	}

	r.Use(middlewares.User())
	r.Use(middlewares.FilterContext())

	if env.IsBillingEnabled() {
		wh := r.Group()
		{
			wh.Post("/webhooks/paddle", webhooks.IncomingPaddleWebhook())
		}
	}

	r.Use(middlewares.CSRF())

	r.Get("/terms", handlers.LegalPage("Terms of Service", "terms.md"))
	r.Get("/privacy", handlers.LegalPage("Privacy Policy", "privacy.md"))

	r.Post("/_api/tenants", handlers.CreateTenant())
	r.Get("/_api/tenants/:subdomain/availability", handlers.CheckAvailability())
	r.Get("/signup", handlers.SignUp())
	r.Get("/oauth/:provider", handlers.SignInByOAuth())
	r.Get("/oauth/:provider/callback", handlers.OAuthCallback())

	// Starting from this step, a Tenant is required
	r.Use(middlewares.RequireTenant())

	r.Get("/sitemap.xml", handlers.Sitemap())

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
	r.Post("/_api/signin/complete", handlers.CompleteSignInProfile())
	r.Post("/_api/signin", handlers.SignInByEmail())

	// Block if it's private tenant with unauthenticated user
	r.Use(middlewares.CheckTenantPrivacy())

	r.Get("/", handlers.Index())
	r.Get("/posts/:number", handlers.PostDetails())
	r.Get("/posts/:number/:slug", handlers.PostDetails())

	// Does not require authentication
	publicApi := r.Group()
	{
		publicApi.Get("/api/v1/posts", apiv1.SearchPosts())
		publicApi.Get("/api/v1/tags", apiv1.ListTags())
		publicApi.Get("/api/v1/posts/:number", apiv1.GetPost())
		publicApi.Get("/api/v1/posts/:number/comments", apiv1.ListComments())
		publicApi.Get("/api/v1/posts/:number/comments/:id", apiv1.GetComment())
		publicApi.Get("/api/v1/posts/:number/attachments", apiv1.GetPostAttachments())
	}

	// Available to any authenticated user
	membersApi := r.Group()
	{
		membersApi.Use(middlewares.IsAuthenticated())
		membersApi.Use(middlewares.BlockLockedTenants())

		// user settings
		membersApi.Get("/profile", handlers.UserProfile())
		membersApi.Post("/_api/user/name", handlers.UpdateUserName())
		membersApi.Post("/_api/user/avatar", handlers.UpdateUserAvatar())
		membersApi.Post("/_api/user/settings", handlers.UpdateUserSettings())
		membersApi.Get("/change-email/verify", handlers.VerifyChangeEmailKey())
		membersApi.Post("/_api/user/regenerate-apikey", handlers.RegenerateAPIKey())
		membersApi.Post("/_api/user/change-email", handlers.ChangeUserEmail())
		membersApi.Delete("/_api/user", handlers.DeleteUser())
		membersApi.Get("/api/v1/user/profile/:userID/content/search", apiv1.SearchUserContent()) // 'Visitors' users can only search their own content
		membersApi.Get("/api/v1/user/profile/:userID/stats", apiv1.GetUserProfileStats())        // 'Visitors' users can only view their own stats
		membersApi.Get("/api/v1/user/profile/:userID/standing", apiv1.GetUserProfileStanding())  // 'Visitors' users can only view their own standing

		// notifications
		membersApi.Get("/notifications", handlers.Notifications())
		membersApi.Get("/notifications/:id", handlers.ReadNotification())
		membersApi.Get("/_api/notifications", handlers.GetAllNotifications())
		membersApi.Get("/_api/notifications/unread/total", handlers.TotalUnreadNotifications())
		membersApi.Post("/_api/notifications/purge-read", handlers.PurgeReadNotifications())
		membersApi.Post("/_api/notifications/read-all", handlers.ReadAllNotifications())

		// posting
		membersApi.Post("/api/v1/posts", apiv1.CreatePost())
		membersApi.Put("/api/v1/posts/:number", apiv1.UpdatePost())

		// comments
		membersApi.Post("/api/v1/posts/:number/comments", apiv1.PostComment())
		membersApi.Put("/api/v1/posts/:number/comments/:id", apiv1.UpdateComment())
		membersApi.Delete("/api/v1/posts/:number/comments/:id", apiv1.DeleteComment())
		membersApi.Post("/api/v1/posts/:number/comments/:id/reactions/:reaction", apiv1.ToggleReaction())
		membersApi.Get("/api/v1/taggable-users", apiv1.ListTaggableUsers())

		// voting
		membersApi.Post("/api/v1/posts/:number/up", apiv1.AddVote())
		membersApi.Post("/api/v1/posts/:number/down", apiv1.AddDownVote())
		membersApi.Delete("/api/v1/posts/:number/votes", apiv1.RemoveVote())
		membersApi.Post("/api/v1/posts/:number/votes/toggle", apiv1.ToggleVote())
		membersApi.Post("/api/v1/posts/:number/subscription", apiv1.Subscribe())
		membersApi.Delete("/api/v1/posts/:number/subscription", apiv1.Unsubscribe())

		membersApi.Post("/api/v1/posts/:number/report", handlers.ReportPost())
		membersApi.Post("/api/v1/posts/:number/comments/:id/report", handlers.ReportComment())
		membersApi.Get("/api/v1/report-reasons", handlers.GetReportReasons())
	}

	helper := r.Group()
	{
		helper.Use(middlewares.IsAuthenticated())
		helper.Use(middlewares.IsAuthorized(enum.RoleHelper, enum.RoleCollaborator, enum.RoleAdministrator, enum.RoleModerator))
		helper.Use(middlewares.BlockLockedTenants())

		// post queue
		helper.Get("/admin/queue", handlers.PostQueuePage())
		helper.Post("/api/v1/queue/:id/heartbeat", handlers.QueuePostHeartbeat())
		helper.Delete("/api/mod/queue-viewing", handlers.StopViewingQueuePost())
		helper.Get("/api/mod/queue-events", handlers.QueueSSE())

		// tags
		helper.Post("/api/v1/posts/:number/tags/:slug", apiv1.AssignTag())
		helper.Delete("/api/v1/posts/:number/tags/:slug", apiv1.UnassignTag())
	}

	// Available to both collaborators, administrators and moderators
	staff := r.Group()
	{
		staff.Use(middlewares.IsAuthenticated())
		staff.Use(middlewares.IsAuthorized(enum.RoleCollaborator, enum.RoleAdministrator, enum.RoleModerator))
		staff.Use(middlewares.BlockLockedTenants())

		// user profiles
		staff.Get("/profile/:id", handlers.ViewUserProfile())
		staff.Post("/_api/users/:userID/name", handlers.UpdateUserName())
		staff.Post("/_api/users/:userID/avatar", handlers.UpdateUserAvatar())

		// user moderation
		staff.Post("/_api/admin/users/:userID/mute", handlers.MuteUser())
		staff.Post("/_api/admin/users/:userID/warn", handlers.WarnUser())
		staff.Post("/_api/admin/users/:userID/warnings/:warningID/expire", handlers.ExpireWarning())
		staff.Post("/_api/admin/users/:userID/mutes/:muteID/expire", handlers.ExpireMute())
		staff.Get("/api/v1/responses/:type", apiv1.ListCannedResponses())
		staff.Get("/admin/members", handlers.ManageMembers())
		staff.Get("/api/v1/users", apiv1.ListUsers())

		// posts
		staff.Get("/api/v1/posts/:number/votes", apiv1.ListVotes())
		staff.Delete("/api/v1/posts/:number", apiv1.DeletePost())

		// reports
		staff.Get("/admin/reports", handlers.ManageReportsPage())
		staff.Get("/api/v1/reports", handlers.ListReports())
		staff.Get("/api/v1/reports/:id", handlers.GetReport())
		staff.Get("/api/v1/reports/:id/details", handlers.GetReportDetails())
		staff.Post("/api/v1/reports/:id/assign", handlers.AssignReport())
		staff.Delete("/api/v1/reports/:id/assign", handlers.UnassignReport())
		staff.Put("/api/v1/reports/:id/resolve", handlers.ResolveReport())
		staff.Post("/api/v1/reports/:id/heartbeat", handlers.ReportHeartbeat())
		staff.Delete("/api/mod/viewing", handlers.StopViewingReport())
		staff.Get("/api/mod/report-events", handlers.ReportsSSE())
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
		collabAdmin.Post("/_api/admin/settings/content-settings", handlers.UpdateContentSettings())

		collabAdmin.Post("/_api/admin/settings/message-banner", handlers.UpdateMessageBanner())

		collabAdmin.Get("/admin/responses", handlers.ManageCannedResponses())
		collabAdmin.Post("/api/v1/responses", apiv1.CreateCannedResponse())
		collabAdmin.Put("/api/v1/responses/:id", apiv1.UpdateCannedResponse())
		collabAdmin.Delete("/api/v1/responses/:id", apiv1.DeleteCannedResponse())

		collabAdmin.Get("/api/v1/report-reasons/all", handlers.ListAllReportReasons())
		collabAdmin.Post("/api/v1/report-reasons", handlers.CreateReportReason())

		collabAdmin.Get("/admin/archive", handlers.ArchivePostsPage())
		collabAdmin.Get("/api/v1/archive/posts", handlers.ListArchivablePosts())
		collabAdmin.Post("/api/v1/posts/:number/archive", handlers.ArchivePost())
		collabAdmin.Post("/api/v1/posts/:number/unarchive", handlers.UnarchivePost())
		collabAdmin.Post("/api/v1/archive/bulk", handlers.BulkArchive())
		collabAdmin.Put("/api/v1/report-reasons/:id", handlers.UpdateReportReason())
		collabAdmin.Delete("/api/v1/report-reasons/:id", handlers.DeleteReportReason())
		collabAdmin.Put("/api/v1/admin/report-reasons-order", handlers.ReorderReportReasons())

		collabAdmin.Get("/admin/tags", handlers.ManageTags())
		collabAdmin.Post("/api/v1/tags", apiv1.CreateEditTag())
		collabAdmin.Put("/api/v1/tags/:slug", apiv1.CreateEditTag())
		collabAdmin.Delete("/api/v1/tags/:slug", apiv1.DeleteTag())

		collabAdmin.Get("/admin/webhooks", handlers.ManageWebhooks())
		collabAdmin.Post("/_api/admin/webhook", handlers.CreateWebhook())
		collabAdmin.Put("/_api/admin/webhook/:id", handlers.UpdateWebhook())
		collabAdmin.Delete("/_api/admin/webhook/:id", handlers.DeleteWebhook())
		collabAdmin.Get("/_api/admin/webhook/test/:id", handlers.TestWebhook())
		collabAdmin.Post("/_api/admin/webhook/preview", handlers.PreviewWebhook())
		collabAdmin.Get("/_api/admin/webhook/props/:type", handlers.GetWebhookProps())

		// user moderation
		collabAdmin.Post("/_api/admin/visualroles/:visualRole/users", handlers.ChangeUserVisualRole())

		collabAdmin.Put("/_api/admin/users/:userID/block", handlers.BlockUser())
		collabAdmin.Delete("/_api/admin/users/:userID/block", handlers.UnblockUser())

		collabAdmin.Delete("/_api/admin/users/:userID/warnings/:warningID", handlers.DeleteWarning())
		collabAdmin.Delete("/_api/admin/users/:userID/mutes/:muteID", handlers.DeleteMute())

		collabAdmin.Put("/api/v1/posts/:number/lock", apiv1.LockOrUnlockPost())
		collabAdmin.Delete("/api/v1/posts/:number/lock", apiv1.LockOrUnlockPost())

		// posts
		collabAdmin.Put("/api/v1/posts/:number/status", apiv1.SetResponse())
	}

	// Only available to administrators
	adminOnly := r.Group()
	{
		adminOnly.Use(middlewares.SetLocale("en"))
		adminOnly.Use(middlewares.IsAuthenticated())
		adminOnly.Use(middlewares.IsAuthorized(enum.RoleAdministrator))
		adminOnly.Use(middlewares.BlockLockedTenants())

		// admin pages
		adminOnly.Post("/_api/admin/settings/general", handlers.UpdateSettings()) // General Page

		adminOnly.Get("/admin/privacy", handlers.Page("Privacy · Site Settings", "", "Administration/pages/PrivacySettings.page"))
		adminOnly.Post("/_api/admin/settings/privacy", handlers.UpdatePrivacy())

		adminOnly.Get("/admin/advanced", handlers.AdvancedSettingsPage())
		adminOnly.Post("/_api/admin/settings/advanced", handlers.UpdateAdvancedSettings())
		adminOnly.Post("/_api/admin/settings/profanity", handlers.UpdateProfanityWords())

		adminOnly.Get("/admin/invitations", handlers.Page("Invitations · Site Settings", "", "Administration/pages/Invitations.page"))
		adminOnly.Post("/api/v1/invitations/send", apiv1.SendInvites())
		adminOnly.Post("/api/v1/invitations/sample", apiv1.SendSampleInvite())

		adminOnly.Get("/admin/authentication", handlers.ManageAuthentication())
		adminOnly.Post("/_api/admin/oauth", handlers.SaveOAuthConfig())
		adminOnly.Get("/_api/admin/oauth/:provider", handlers.GetOAuthConfig())
		adminOnly.Post("/_api/admin/settings/emailauth", handlers.UpdateEmailAuthAllowed())

		if env.IsBillingEnabled() {
			adminOnly.Get("/admin/billing", handlers.ManageBilling())
			adminOnly.Post("/_api/billing/checkout-link", handlers.GenerateCheckoutLink())
		}

		adminOnly.Get("/admin/files", handlers.FileManagementPage())
		adminOnly.Get("/api/v1/admin/files", handlers.ListFiles())
		adminOnly.Post("/api/v1/admin/files", handlers.UploadFile())
		adminOnly.Put("/api/v1/admin/files/:blobKey/*path", handlers.RenameFile())
		adminOnly.Delete("/api/v1/admin/files/:blobKey/*path", handlers.DeleteFile())
		adminOnly.Get("/api/v1/admin/files/:blobKey/usage/*path", handlers.GetFileUsage())

		// user management
		adminOnly.Post("/api/v1/users", apiv1.CreateUser())
		adminOnly.Post("/_api/admin/roles/:role/users", handlers.ChangeUserRole())

		// export
		adminOnly.Get("/admin/export", handlers.Page("Export · Site Settings", "", "Administration/pages/Export.page"))
		adminOnly.Get("/admin/export/posts.csv", handlers.ExportPostsToCSV())
		adminOnly.Get("/admin/export/backup.zip", handlers.ExportBackupZip())

		// dev
		adminOnly.Get("/_design", handlers.Page("Design System", "A preview of Fider UI elements", "DesignSystem/DesignSystem.page"))
	}

	return r
}
