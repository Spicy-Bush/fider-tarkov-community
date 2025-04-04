package handlers

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"

	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// ChangeUserEmail register the intent of changing user email
func ChangeUserEmail() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().Role != enum.RoleAdministrator && c.User().Role != enum.RoleCollaborator {
			return c.Redirect(c.BaseURL() + "/settings")
		}

		action := actions.NewChangeUserEmail()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

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
	}
}

// VerifyChangeEmailKey checks if key is correct and update user's email
func VerifyChangeEmailKey() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().Role != enum.RoleAdministrator && c.User().Role != enum.RoleCollaborator {
			return c.Redirect(c.BaseURL() + "/settings")
		}
		key := c.QueryParam("k")
		result, err := validateKey(enum.EmailVerificationKindChangeEmail, key, c)
		if result == nil {
			return err
		}

		if result.UserID != c.User().ID {
			return c.Redirect(c.BaseURL())
		}

		changeEmail := &cmd.ChangeUserEmail{
			UserID: result.UserID,
			Email:  result.Email,
		}
		if err = bus.Dispatch(c, changeEmail); err != nil {
			return c.Failure(err)
		}

		err = bus.Dispatch(c, &cmd.SetKeyAsVerified{Key: key})
		if err != nil {
			return c.Failure(err)
		}

		if env.Config.UserList.Enabled {
			c.Enqueue(tasks.UserListUpdateUser(c.User().ID, "", result.Email))
		}

		return c.Redirect(c.BaseURL() + "/settings")
	}
}

// UserSettings is the current user's profile settings page
func UserSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		settings := &query.GetCurrentUserSettings{}
		if err := bus.Dispatch(c, settings); err != nil {
			return err
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "MySettings/MySettings.page",
			Title: "Settings",
			Data: web.Map{
				"userSettings": settings.Result,
			},
		})
	}
}

// UpdateUserSettings updates current user settings
func UpdateUserSettings() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUpdateUserSettings()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		if err := bus.Dispatch(c,
			&cmd.UploadImage{
				Image:  action.Avatar,
				Folder: "avatars",
			},
			&cmd.UpdateCurrentUser{
				Name:       action.Name,
				Avatar:     action.Avatar,
				AvatarType: action.AvatarType,
			},
			&cmd.UpdateCurrentUserSettings{
				Settings: action.Settings,
			},
		); err != nil {
			return c.Failure(err)
		}

		if env.Config.UserList.Enabled {
			c.Enqueue(tasks.UserListUpdateUser(c.User().ID, action.Name, ""))
		}

		return c.Ok(web.Map{})
	}
}

// ChangeUserRole changes given user role
func ChangeUserRole() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.ChangeUserRole)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

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

		return c.Ok(web.Map{})
	}
}

// ChangeUserVisualRole changes given user visual role
func ChangeUserVisualRole() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.ChangeUserVisualRole)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		changeVisualRole := &cmd.ChangeUserVisualRole{
			UserID:     action.UserID,
			VisualRole: action.VisualRole,
		}

		if err := bus.Dispatch(c, changeVisualRole); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{})
	}
}

// DeleteUser erases current user personal data and sign them out
func DeleteUser() web.HandlerFunc {
	return func(c *web.Context) error {
		if err := bus.Dispatch(c, &cmd.DeleteCurrentUser{}); err != nil {
			return c.Failure(err)
		}

		c.RemoveCookie(web.CookieAuthName)

		// Handle userlist (easiest way is to demote them which will remove them from the userlist)
		if env.Config.UserList.Enabled {
			c.Enqueue(tasks.UserListAddOrRemoveUser(c.User().ID, enum.RoleVisitor))
		}

		return c.Ok(web.Map{})
	}
}

// RegenerateAPIKey regenerates current user's API Key
func RegenerateAPIKey() web.HandlerFunc {
	return func(c *web.Context) error {
		regenerateAPIKey := &cmd.RegenerateAPIKey{}
		if err := bus.Dispatch(c, regenerateAPIKey); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"apiKey": regenerateAPIKey.Result,
		})
	}
}
