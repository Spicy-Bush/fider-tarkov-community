package handlers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"net/http"
	"os"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/fs"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
)

func TestUpdateSettingsHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateTenantSettings) error {
		Expect(c.Title).Equals("GoT")
		Expect(c.Invitation).Equals("Join us!")
		Expect(c.WelcomeMessage).Equals("Welcome to GoT Feedback Forum")
		Expect(c.Locale).Equals("pt-BR")
		Expect(c.Logo.BlobKey).Equals("logos/hello-world.png")
		return nil
	})

	bus.AddHandler(func(ctx context.Context, c *cmd.UploadImage) error {
		return nil
	})

	server := mock.NewServer()
	mock.DemoTenant.LogoBlobKey = "logos/hello-world.png"

	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(
			handlers.UpdateSettings(),
			`{ "title": "GoT", "invitation": "Join us!", "welcomeMessage": "Welcome to GoT Feedback Forum", "locale": "pt-BR" }`,
		)

	Expect(code).Equals(http.StatusOK)
	ExpectHandler(&cmd.UpdateTenantSettings{}).CalledOnce()
	ExpectHandler(&cmd.UploadImage{}).CalledOnce()
}

func TestUpdateSettingsHandler_NewLogo(t *testing.T) {
	RegisterT(t)
	bus.Init(fs.Service{})

	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateTenantSettings) error {
		Expect(c.Title).Equals("GoT")
		Expect(c.Invitation).Equals("Join us!")
		Expect(c.WelcomeMessage).Equals("Welcome to GoT Feedback Forum")
		Expect(c.Locale).Equals("pt-BR")
		Expect(c.Logo.BlobKey).Equals("logos/picture.png")
		return nil
	})

	bus.AddHandler(func(ctx context.Context, c *cmd.UploadImage) error {
		c.Image.BlobKey = c.Folder + "/" + c.Image.Upload.FileName
		return nil
	})

	logoBytes, _ := os.ReadFile(env.Etc("logo.png"))
	logoB64 := base64.StdEncoding.EncodeToString(logoBytes)

	server := mock.NewServer()
	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(
			handlers.UpdateSettings(), `{ 
				"title": "GoT", 
				"invitation": "Join us!", 
				"welcomeMessage": "Welcome to GoT Feedback Forum",
				"locale": "pt-BR",
				"logo": {
					"upload": {
						"fileName": "picture.png",
						"contentType": "image/png",
						"content": "`+logoB64+`"
					}
				}
			}`)

	Expect(code).Equals(http.StatusOK)
	ExpectHandler(&cmd.UpdateTenantSettings{}).CalledOnce()
	ExpectHandler(&cmd.UploadImage{}).CalledOnce()
}

func TestUpdateSettingsHandler_RemoveLogo(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateTenantSettings) error {
		Expect(c.Title).Equals("GoT")
		Expect(c.Invitation).Equals("Join us!")
		Expect(c.WelcomeMessage).Equals("Welcome to GoT Feedback Forum")
		Expect(c.Logo.Remove).IsTrue()
		Expect(c.Locale).Equals("en")
		return nil
	})

	bus.AddHandler(func(ctx context.Context, c *cmd.UploadImage) error {
		return nil
	})

	server := mock.NewServer()
	mock.DemoTenant.LogoBlobKey = "logos/hello-world.png"

	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(
			handlers.UpdateSettings(), `{ 
				"title": "GoT", 
				"invitation": "Join us!", 
				"locale": "en", 
				"welcomeMessage": "Welcome to GoT Feedback Forum",
				"logo": {
					"remove": true
				}
			}`)

	Expect(code).Equals(http.StatusOK)
	ExpectHandler(&cmd.UpdateTenantSettings{}).CalledOnce()
	ExpectHandler(&cmd.UploadImage{}).CalledOnce()
}

func TestUpdatePrivacyHandler(t *testing.T) {
	RegisterT(t)

	var updateCmd *cmd.UpdateTenantPrivacySettings
	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateTenantPrivacySettings) error {
		updateCmd = c
		return nil
	})

	server := mock.NewServer()
	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(
			handlers.UpdatePrivacy(),
			`{ "isPrivate": true }`,
		)

	Expect(code).Equals(http.StatusOK)
	Expect(updateCmd.IsPrivate).IsTrue()
}

func TestManageMembersHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetAllUsers) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetAllUserProviders) error {
		q.Result = []*entity.UserProvider{}
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()
	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		Execute(
			handlers.ManageMembers(),
		)

	Expect(code).Equals(http.StatusOK)
}

func TestManagePermissionsPage(t *testing.T) {
	RegisterT(t)
	bus.AddHandler(func(ctx context.Context, q *query.GetRolePermissionState) error {
		tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		q.Result = tenant.PermissionState(ctx.Value(app.UserCtxKey).(*entity.User))
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	tenant := *mock.DemoTenant
	tenant.RolePermissions = entity.RolePermissions{enum.RoleHelper: {entity.ManageReports: true}}
	status, response := mock.NewServer().
		OnTenant(&tenant).
		AsUser(mock.JonSnow).
		AddHeader("Accept", web.PageDataContentType).
		Execute(handlers.ManagePermissionsPage())
	Expect(status).Equals(http.StatusOK)

	var page struct {
		Page  string `json:"page"`
		Props struct {
			Permissions map[string][]string          `json:"permissions"`
			Defaults    map[string][]string          `json:"defaults"`
			Requires    map[string][]string          `json:"requires"`
			BaseLocks   map[string]map[string]string `json:"baseLocks"`
		} `json:"props"`
	}
	Expect(json.Unmarshal(response.Body.Bytes(), &page)).IsNil()
	Expect(page.Page).Equals("Administration/pages/ManagePermissions.page")
	Expect(page.Props.Permissions["helper"]).Equals([]string{"createPosts", "editPosts", "manageQueue", "manageReports", "tagPosts"})
	Expect(page.Props.Defaults["helper"]).Equals([]string{"createPosts", "editPosts", "manageQueue", "tagPosts"})
	Expect(page.Props.Permissions["visitor"]).Equals([]string{"createPosts", "editPosts"})
	Expect(page.Props.Requires["manageReportReasons"]).Equals([]string{"manageReports"})
	Expect(len(page.Props.BaseLocks["administrator"])).Equals(len(page.Props.Permissions["administrator"]))
	Expect(page.Props.BaseLocks["administrator"]["manageAuthentication"]).Equals("Administrators have every permission")
	Expect(page.Props.BaseLocks["visitor"]).Equals(map[string]string{
		"manageAuthentication":  "Only administrators can manage authentication",
		"exportBackup":          "Only administrators can export full backups",
		"manageRolePermissions": "Visitors cannot manage role permissions",
	})
}

func TestUpdateRolePermissionsHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateRolePermissions) error {
		tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		next, blocked := tenant.RolePermissions.ApplyChanges(ctx.Value(app.UserCtxKey).(*entity.User), tenant, c.Changes)
		Expect(blocked).Equals("")
		tenant.RolePermissions = next
		c.Result.RolePermissionState = tenant.PermissionState(ctx.Value(app.UserCtxKey).(*entity.User))
		return nil
	})

	tenant := *mock.DemoTenant
	status, response := mock.NewServer().
		OnTenant(&tenant).
		AsUser(mock.JonSnow).
		ExecutePost(handlers.UpdateRolePermissions(), `{ "changes": [{ "role": "visitor", "permission": "manageTags", "granted": true }], "submissionId": "permission-save" }`)

	Expect(status).Equals(http.StatusOK)
	var saved struct {
		Permissions map[string][]string `json:"permissions"`
	}
	Expect(json.Unmarshal(response.Body.Bytes(), &saved)).IsNil()
	Expect(saved.Permissions["visitor"]).Equals([]string{"createPosts", "editPosts", "manageTags", "viewPrivateTags"})
	ExpectHandler(&cmd.UpdateRolePermissions{}).CalledOnce()
}

func TestUpdateRolePermissionsHandler_Rejects(t *testing.T) {
	RegisterT(t)

	for _, test := range []struct {
		name   string
		user   *entity.User
		body   string
		status int
	}{
		{"visitor", mock.AryaStark, `{ "changes": [{ "role": "visitor", "permission": "manageTags", "granted": true }] }`, http.StatusForbidden},
		{"no changes", mock.JonSnow, `{ "changes": [], "submissionId": "permission-save" }`, http.StatusBadRequest},
		{"unknown role", mock.JonSnow, `{ "changes": [{ "role": "owner", "permission": "manageTags", "granted": true }], "submissionId": "permission-save" }`, http.StatusBadRequest},
		{"unknown permission", mock.JonSnow, `{ "changes": [{ "role": "visitor", "permission": "root", "granted": true }], "submissionId": "permission-save" }`, http.StatusBadRequest},
		{"missing identity", mock.JonSnow, `{ "changes": [{ "role": "visitor", "permission": "manageTags", "granted": true }] }`, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := mock.NewServer().
				OnTenant(mock.DemoTenant).
				AsUser(test.user).
				ExecutePost(handlers.UpdateRolePermissions(), test.body)
			Expect(status).Equals(test.status)
			ExpectHandler(&cmd.UpdateRolePermissions{}).CalledTimes(0)
		})
	}
}
