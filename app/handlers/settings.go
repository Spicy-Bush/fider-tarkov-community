package handlers

import (
	"net/http"
	"strconv"
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
		if c.User().Role != enum.RoleAdministrator && c.User().Role != enum.RoleCollaborator {
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

		// Get userID from URL parameter, default to current user's ID if not provided
		userID := c.User().ID
		if c.Param("userID") != "" {
			var err error
			userID, err = strconv.Atoi(c.Param("userID"))
			if err != nil {
				return c.BadRequest(web.Map{
					"error": "Invalid user ID",
				})
			}

			// You can only update your own name, unless you are privileged
			if userID != c.User().ID {
				if c.User().Role != enum.RoleAdministrator &&
					c.User().Role != enum.RoleCollaborator &&
					c.User().Role != enum.RoleModerator {
					return c.Forbidden()
				}

				// If user is a moderator, they can't update collaborators or admins
				if c.User().Role == enum.RoleModerator {
					getUser := &query.GetUserByID{UserID: userID}
					if err := bus.Dispatch(c, getUser); err != nil {
						return c.Failure(err)
					}

					if getUser.Result.Role == enum.RoleAdministrator ||
						getUser.Result.Role == enum.RoleCollaborator {
						return c.Forbidden()
					}
				}
			}
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.UpdateUser{
				UserID: userID,
				Name:   action.Name,
			}); err != nil {
				return c.Failure(err)
			}

			if env.Config.UserList.Enabled {
				c.Enqueue(tasks.UserListUpdateUser(userID, action.Name, ""))
			}

			return c.Ok(web.Map{})
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

			// Check if user is trying to update someone else's avatar
			if userID != c.User().ID {
				// Only allow staff to update other users' avatars
				if c.User().Role != enum.RoleAdministrator &&
					c.User().Role != enum.RoleCollaborator &&
					c.User().Role != enum.RoleModerator {
					return c.Forbidden()
				}

				// If user is a moderator, they can't update collaborators or admins
				if c.User().Role == enum.RoleModerator {
					getUser := &query.GetUserByID{UserID: userID}
					if err := bus.Dispatch(c, getUser); err != nil {
						return c.Failure(err)
					}

					if getUser.Result.Role == enum.RoleAdministrator ||
						getUser.Result.Role == enum.RoleCollaborator {
						return c.Forbidden()
					}
				}
			}
		}

		return c.WithTransaction(func() error {
			if action.Avatar != nil && action.Avatar.Upload != nil {
				if err := bus.Dispatch(c, &cmd.UploadImage{
					Image:  action.Avatar,
					Folder: "avatars",
				}); err != nil {
					return c.Failure(err)
				}
			}

			// Different dispatch based on whether it's updating current user or another user
			if userID == c.User().ID {
				if err := bus.Dispatch(c, &cmd.UpdateCurrentUser{
					Avatar:     action.Avatar,
					AvatarType: action.AvatarType,
				}); err != nil {
					return c.Failure(err)
				}
			} else {
				if err := bus.Dispatch(c, &cmd.UpdateUserAvatar{
					UserID:     userID,
					AvatarType: action.AvatarType,
					Avatar:     action.Avatar,
				}); err != nil {
					return c.Failure(err)
				}
			}

			return c.Ok(web.Map{})
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

			return c.Ok(web.Map{})
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

			return c.Ok(web.Map{})
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
				},
				"userSettings": settings.Result,
			},
		})
	}
}
