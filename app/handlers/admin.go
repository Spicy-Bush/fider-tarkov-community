package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

// GeneralSettingsPage is the general settings page
func GeneralSettingsPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/GeneralSettings.page",
			Title: "General · Site Settings",
		})
	}
}

func ContentSettingsPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ContentSettings.page",
			Title: "Content Settings · Site Settings",
			Data: web.Map{
				"roles": []string{
					enum.RoleVisitor.String(),
					enum.RoleCollaborator.String(),
					enum.RoleModerator.String(),
					enum.RoleAdministrator.String(),
					enum.RoleHelper.String(),
				},
			},
		})
	}
}

func UpdateContentSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdateContentSettings)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			err := bus.Dispatch(c, &cmd.UpdateContentSettings{
				Settings: action.Settings,
			})
			if err != nil {
				return c.Failure(err)
			}

			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// AdvancedSettingsPage is the advanced settings page
func AdvancedSettingsPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/AdvancedSettings.page",
			Title: "Advanced · Site Settings",
			Data: web.Map{
				"customCSS": c.Tenant().CustomCSS,
				// replace commas with newlines makes it easier to edit for the user, we convert it back to commas when saving
				"profanityWords": strings.ReplaceAll(c.Tenant().ProfanityWords, ",", "\n"),
			},
		})
	}
}

// UpdateSettings update current tenant' settings
func UpdateSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateTenantSettings()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c,
				&cmd.UploadImage{
					Image:  action.Logo,
					Folder: "logos",
				},
				&cmd.UpdateTenantSettings{
					Logo:           action.Logo,
					Title:          action.Title,
					Invitation:     action.Invitation,
					WelcomeMessage: action.WelcomeMessage,
					CNAME:          action.CNAME,
					Locale:         action.Locale,
				},
			); err != nil {
				return c.Failure(err)
			}

			// Handle userlist.
			if env.Config.UserList.Enabled {
				c.Enqueue(tasks.UserListUpdateCompany(&dto.UserListUpdateCompany{
					TenantID: c.Tenant().ID,
					Name:     action.Title,
				}))
			}

			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// UpdateAdvancedSettings update current tenant' advanced settings
func UpdateAdvancedSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdateTenantAdvancedSettings)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}
		tenant := c.Tenant()

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.UpdateTenantAdvancedSettings{
				CustomCSS:      action.CustomCSS,
				ProfanityWords: tenant.ProfanityWords,
			}); err != nil {
				return c.Failure(err)
			}

			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// UpdatePrivacy update current tenant's privacy settings
func UpdatePrivacy() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdateTenantPrivacy)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateSettings := &cmd.UpdateTenantPrivacySettings{
				IsPrivate: action.IsPrivate,
			}
			if err := bus.Dispatch(c, updateSettings); err != nil {
				return c.Failure(err)
			}

			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// UpdateEmailAuthAllowed update current tenant's allow email auth settings
func UpdateEmailAuthAllowed() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdateTenantEmailAuthAllowed)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateSettings := &cmd.UpdateTenantEmailAuthAllowedSettings{
				IsEmailAuthAllowed: action.IsEmailAuthAllowed,
			}
			if err := bus.Dispatch(c, updateSettings); err != nil {
				return c.Failure(err)
			}

			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// ManageMembers is the page used by administrators to change member's role
func ManageMembers() web.HandlerFunc {
	return func(c *web.Context) error {
		allUsers := &query.GetAllUsers{}
		if err := bus.Dispatch(c, allUsers); err != nil {
			return c.Failure(err)
		}

		// Create an array of UserWithEmail structs from the allUsers.Result & add the providers to each user
		allProviders := &query.GetAllUserProviders{}
		if err := bus.Dispatch(c, allProviders); err != nil {
			return c.Failure(err)
		}

		userMap := make(map[int]*entity.User)
		for _, user := range allUsers.Result {
			userMap[user.ID] = user
			user.Providers = []*entity.UserProvider{}
		}

		for _, p := range allProviders.Result {
			if user, exists := userMap[p.UserID]; exists {
				user.Providers = append(user.Providers, &entity.UserProvider{
					Name: p.Name,
					UID:  p.UID,
				})
			}
		}

		// Only administrators and collaborators can see emails
		canSeeEmail := c.User().IsCollaborator() || c.User().IsAdministrator()

		allUsersWithEmail := make([]entity.UserWithEmail, len(allUsers.Result))
		for i, user := range allUsers.Result {
			if canSeeEmail {
				allUsersWithEmail[i] = entity.UserWithEmail{
					User: user,
				}
			} else {
				userCopy := *user
				userCopy.Email = ""
				allUsersWithEmail[i] = entity.UserWithEmail{
					User: &userCopy,
				}
			}
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageMembers.page",
			Title: "Manage Members · Site Settings",
			Data: web.Map{
				"users": allUsersWithEmail,
			},
		})
	}
}

// ManageAuthentication is the page used by administrators to change site authentication settings
func ManageAuthentication() web.HandlerFunc {
	return func(c *web.Context) error {
		listProviders := &query.ListAllOAuthProviders{}
		if err := bus.Dispatch(c, listProviders); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageAuthentication.page",
			Title: "Authentication · Site Settings",
			Data: web.Map{
				"providers": listProviders.Result,
			},
		})
	}
}

// ManageCannedResponses is the page used by administrators to manage canned responses
func ManageCannedResponses() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageCannedResponses.page",
			Title: "Canned Responses · Site Settings",
			Data: web.Map{
				"types": []string{"warning", "mute"},
			},
		})
	}
}

// GetOAuthConfig returns OAuth config based on given provider
func GetOAuthConfig() web.HandlerFunc {
	return func(c *web.Context) error {
		getConfig := &query.GetCustomOAuthConfigByProvider{
			Provider: c.Param("provider"),
		}
		if err := bus.Dispatch(c, getConfig); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getConfig.Result)
	}
}

// SaveOAuthConfig is used to create/edit OAuth configurations
func SaveOAuthConfig() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewCreateEditOAuthConfig()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c,
				&cmd.UploadImage{
					Image:  action.Logo,
					Folder: "logos",
				},
				&cmd.SaveCustomOAuthConfig{
					ID:                action.ID,
					Logo:              action.Logo,
					Provider:          action.Provider,
					Status:            action.Status,
					DisplayName:       action.DisplayName,
					ClientID:          action.ClientID,
					ClientSecret:      action.ClientSecret,
					AuthorizeURL:      action.AuthorizeURL,
					TokenURL:          action.TokenURL,
					Scope:             action.Scope,
					ProfileURL:        action.ProfileURL,
					IsTrusted:         action.IsTrusted,
					JSONUserIDPath:    action.JSONUserIDPath,
					JSONUserNamePath:  action.JSONUserNamePath,
					JSONUserEmailPath: action.JSONUserEmailPath,
				},
			); err != nil {
				return c.Failure(err)
			}

			web.InvalidateOAuthCache()
			return c.Ok(web.Map{})
		})
	}
}

// UpdateProfanityWords is used to update profanity words for the tenant
func UpdateProfanityWords() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateProfanityWords()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := action.Run(c); err != nil {
				return c.Failure(err)
			}
			middlewares.InvalidateTenantCache()
			return c.Ok(web.Map{})
		})
	}
}

// MuteUser mutes a user for a specified duration
func MuteUser() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.BadRequest(web.Map{
				"message": "Invalid user ID",
			})
		}

		action := new(actions.MuteUser)
		action.UserID = userID

		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			expiresAt := time.Now().Add(time.Duration(action.Duration) * time.Minute)
			muteUser := &cmd.MuteUser{
				UserID:    userID,
				Reason:    action.Reason,
				ExpiresAt: expiresAt,
			}

			if err := bus.Dispatch(c, muteUser); err != nil {
				return c.Failure(err)
			}

			getUser := &query.GetUserByID{UserID: userID}
			if err := bus.Dispatch(c, getUser); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutMute(getUser.Result, action.Reason, &expiresAt))

			return c.Ok(web.Map{})
		})
	}
}

// WarnUser adds a warning to a user
func WarnUser() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.BadRequest(web.Map{
				"message": "Invalid user ID",
			})
		}

		action := new(actions.WarnUser)
		action.UserID = userID

		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			var expiresAt time.Time
			if action.Duration > 0 {
				expiresAt = time.Now().Add(time.Duration(action.Duration) * time.Minute)
			}
			warnUser := &cmd.WarnUser{
				UserID:    userID,
				Reason:    action.Reason,
				ExpiresAt: expiresAt,
			}

			if err := bus.Dispatch(c, warnUser); err != nil {
				return c.Failure(err)
			}

			getUser := &query.GetUserByID{UserID: userID}
			if err := bus.Dispatch(c, getUser); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutWarning(getUser.Result, action.Reason, &expiresAt))

			return c.Ok(web.Map{})
		})
	}
}
