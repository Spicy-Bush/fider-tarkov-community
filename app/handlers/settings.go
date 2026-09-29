package handlers

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"

	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func generateRandomUsername() string {
	return fmt.Sprintf("User%05d", rand.Intn(100000))
}

// ChangeUserEmail register the intent of changing user email
func ChangeUserEmail() web.HandlerFunc {
	return func(c *web.Context) error {
		if !entity.Can(c.User(), c.Tenant(), entity.ChangeOwnEmail) {
			return c.Redirect(c.BaseURL() + "/profile#settings")
		}

		action := actions.NewChangeUserEmail()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			err := bus.Dispatch(c, &cmd.SaveVerificationKey{
				Key:      action.VerificationKey,
				Duration: 24 * time.Hour,
				Request:  action,
			})
			if err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.SendChangeEmailConfirmation(action))

			return c.Ok(web.Map{})
		})
	}
}

// VerifyChangeEmailKey checks if key is correct and update user's email
func VerifyChangeEmailKey() web.HandlerFunc {
	return func(c *web.Context) error {
		if !entity.Can(c.User(), c.Tenant(), entity.ChangeOwnEmail) {
			return c.Redirect(c.BaseURL() + "/profile#settings")
		}
		key := c.QueryParam("k")
		result, err := validateKey(enum.EmailVerificationKindChangeEmail, key, c)
		if result == nil {
			return err
		}

		if result.UserID != c.User().ID {
			return c.Redirect(c.BaseURL())
		}

		err = c.WithTransaction(func() error {
			changeEmail := &cmd.ChangeUserEmail{
				UserID: result.UserID,
				Email:  result.Email,
			}
			if err := bus.Dispatch(c, changeEmail); err != nil {
				return c.Failure(err)
			}

			if err := bus.Dispatch(c, &cmd.SetKeyAsVerified{Key: key}); err != nil {
				return c.Failure(err)
			}

			if env.Config.UserList.Enabled {
				c.Enqueue(tasks.UserListUpdateUser(c.User().ID, "", result.Email))
			}
			return nil
		})
		if err != nil {
			return err
		}

		return c.Redirect(c.BaseURL() + "/profile#settings")
	}
}

// UpdateUserName updates a user's name
func UpdateUserName() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateUserName()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		userID := c.User().ID
		if c.Param("userID") != "" {
			var err error
			userID, err = strconv.Atoi(c.Param("userID"))
			if err != nil {
				return c.BadRequest(web.Map{
					"error": "Invalid user ID",
				})
			}
		}

		return c.WithTransaction(func() error {
			change := &cmd.SaveProfileName{UserID: userID, Name: action.Name, Review: env.IsOpenAIModerationEnabled() && userID == c.User().ID}
			if err := bus.Dispatch(c, change); err != nil {
				return c.Failure(err)
			}
			getUser := &query.GetUserByID{UserID: userID}
			if err := bus.Dispatch(c, getUser); err != nil {
				return c.Failure(err)
			}
			if env.Config.UserList.Enabled && !change.Pending {
				c.Enqueue(tasks.UserListUpdateUser(userID, action.Name, ""))
			}
			return c.Ok(web.Map{"name": getUser.Result.Name, "pending": change.Pending})
		})
	}
}

// UpdateUserSettings handles the action of updating user settings
func UpdateUserSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateUserSettings()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.UpdateCurrentUserSettings{
				Settings: action.Settings,
			}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// UpdateUserAvatar updates a user's avatar
func UpdateUserAvatar() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateUserAvatar()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		userID := c.User().ID
		if c.Param("userID") != "" {
			var err error
			userID, err = strconv.Atoi(c.Param("userID"))
			if err != nil {
				return c.BadRequest(web.Map{
					"error": "Invalid user ID",
				})
			}
		}

		return c.WithTransaction(func() error {
			getUser := &query.GetUserByID{UserID: userID}
			if err := bus.Dispatch(c, getUser); err != nil {
				return c.Failure(err)
			}
			if !getUser.Result.AllowedActions(c.User(), c.Tenant()).EditAvatar {
				return c.Forbidden()
			}
			blobKey := getUser.Result.AvatarBlobKey
			if action.AvatarType != enum.AvatarTypeCustom {
				blobKey = ""
			}
			if action.AvatarType == enum.AvatarTypeCustom && action.Avatar != nil {
				if action.Avatar.Remove {
					blobKey = ""
					action.AvatarType = enum.AvatarTypeLetter
				} else if action.Avatar.Upload != nil {
					if err := bus.Dispatch(c, &cmd.UploadImage{Image: action.Avatar, Folder: "avatars"}); err != nil {
						return c.Failure(err)
					}
					blobKey = action.Avatar.BlobKey
				}
			}
			if action.AvatarType == enum.AvatarTypeCustom && blobKey == "" {
				return c.BadRequest(web.Map{"errors": []web.Map{{"field": "avatar", "message": "Choose an image for your custom avatar."}}})
			}
			change := &cmd.SaveProfileAvatar{UserID: userID, AvatarType: action.AvatarType, BlobKey: blobKey,
				Review: env.IsOpenAIModerationEnabled() && userID == c.User().ID}
			if err := bus.Dispatch(c, change, getUser); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{"avatarURL": getUser.Result.AvatarURL, "avatarType": getUser.Result.AvatarType, "pending": change.Pending})
		})
	}
}

// ChangeUserRole changes given user role
func ChangeUserRole() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.ChangeUserRole)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			changeRole := &cmd.ChangeUserRole{
				UserID: action.UserID,
				Role:   action.Role,
			}

			if err := bus.Dispatch(c, changeRole); err != nil {
				return c.Failure(err)
			}

			// Handle userlist
			if env.Config.UserList.Enabled {
				c.Enqueue(tasks.UserListAddOrRemoveUser(action.UserID, action.Role))
			}

			return c.Ok(web.Map{
				"id":                 changeRole.Result.ID,
				"role":               changeRole.Result.Role,
				"visualRole":         changeRole.Result.GetVisualRole(),
				"visualRoleOverride": changeRole.Result.VisualRole,
				"permissions":        changeRole.Result.Permissions,
			})
		})
	}
}

// ChangeUserVisualRole changes given user visual role
func ChangeUserVisualRole() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.ChangeUserVisualRole)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			changeVisualRole := &cmd.ChangeUserVisualRole{
				UserID:     action.UserID,
				VisualRole: action.VisualRole,
			}

			if err := bus.Dispatch(c, changeVisualRole); err != nil {
				return c.Failure(err)
			}

			updated := &query.GetUserByID{UserID: action.UserID}
			if err := bus.Dispatch(c, updated); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{
				"id": updated.Result.ID,
				"visualRole": updated.Result.GetVisualRole(),
				"visualRoleOverride": updated.Result.VisualRole,
			})
		})
	}
}

// DeleteUser erases current user personal data and sign them out
func DeleteUser() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteCurrentUser{}); err != nil {
				return c.Failure(err)
			}

			c.RemoveCookie(web.CookieAuthName)

			// Handle userlist (easiest way is to demote them which will remove them from the userlist)
			if env.Config.UserList.Enabled {
				c.Enqueue(tasks.UserListAddOrRemoveUser(c.User().ID, enum.RoleVisitor))
			}

			return c.Ok(web.Map{})
		})
	}
}

// RegenerateAPIKey regenerates current user's API Key
func RegenerateAPIKey() web.HandlerFunc {
	return func(c *web.Context) error {
		if !entity.Can(c.User(), c.Tenant(), entity.ManageAPIKeys) {
			return c.Forbidden()
		}
		return c.WithTransaction(func() error {
			regenerateAPIKey := &cmd.RegenerateAPIKey{}
			if err := bus.Dispatch(c, regenerateAPIKey); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{
				"apiKey": regenerateAPIKey.Result,
			})
		})
	}
}

// UserProfile is the current user's profile page
func UserProfile() web.HandlerFunc {
	return func(c *web.Context) error {
		settings := &query.GetCurrentUserSettings{}
		if err := bus.Dispatch(c, settings); err != nil {
			return err
		}

		return c.Page(http.StatusOK, web.Props{
			Page:        "UserProfile/UserProfile.page",
			Title:       "Profile",
			Description: "View and manage your profile",
			Data: web.Map{
				"user": web.Map{
					"id":        c.User().ID,
					"name":      c.User().Name,
					"role":      c.User().Role,
					"avatarURL": c.User().AvatarURL,
					"status":    c.User().Status,
					"permissions": c.User().AllowedActions(c.User(), c.Tenant()),
				},
				"userSettings": settings.Result,
			},
		})
	}
}
