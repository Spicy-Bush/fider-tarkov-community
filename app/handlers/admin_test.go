package handlers_test

import (
	"context"
	"encoding/base64"
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

	server := mock.NewServer()
	code, _ := server.
		OnTenant(mock.DemoTenant).
		Execute(
			handlers.ManageMembers(),
		)

	Expect(code).Equals(http.StatusOK)
}
