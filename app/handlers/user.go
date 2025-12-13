package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// BlockUser is used to block an existing user from using Fider
func BlockUser() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			err = bus.Dispatch(c, &cmd.BlockUser{UserID: userID})
			if err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// UnblockUser is used to unblock an existing user so they can use Fider again
func UnblockUser() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			err = bus.Dispatch(c, &cmd.UnblockUser{UserID: userID})
			if err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// ViewUserProfile handles viewing another user's profile
func ViewUserProfile() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.NotFound()
		}

		getUser := &query.GetUserByID{UserID: userID}
		if err := bus.Dispatch(c, getUser); err != nil {
			return c.NotFound()
		}

		// Regular users can only view their own profile, admins can view any profile
		isPrivileged := c.User().Role == enum.RoleAdministrator || c.User().Role == enum.RoleCollaborator || c.User().Role == enum.RoleModerator
		if !isPrivileged && c.User().ID != userID {
			return c.NotFound()
		}

		return c.Page(http.StatusOK, web.Props{
			Page:        "UserProfile/UserProfile.page",
			Title:       fmt.Sprintf("%s's Profile", getUser.Result.Name),
			Description: fmt.Sprintf("View %s's profile and activity", getUser.Result.Name),
			Data: web.Map{
				"user": web.Map{
					"id":         getUser.Result.ID,
					"name":       getUser.Result.Name,
					"role":       getUser.Result.Role,
					"visualRole": getUser.Result.GetVisualRole(),
					"avatarURL":  getUser.Result.AvatarURL,
					"status":     getUser.Result.Status,
				},
			},
		})
	}
}

// DeleteWarning handles the deletion of a warning
func DeleteWarning() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		warningID, err := c.ParamAsInt("warningID")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.DeleteWarning{
			UserID:    userID,
			WarningID: warningID,
		}

		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteWarning{
				UserID:    userID,
				WarningID: warningID,
			}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// DeleteMute handles the deletion of a mute
func DeleteMute() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		muteID, err := c.ParamAsInt("muteID")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.DeleteMute{
			UserID: userID,
			MuteID: muteID,
		}

		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteMute{
				UserID: userID,
				MuteID: muteID,
			}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// ExpireWarning handles expiring a warning early (keeps history)
func ExpireWarning() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		warningID, err := c.ParamAsInt("warningID")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.ExpireWarning{
				UserID:    userID,
				WarningID: warningID,
			}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// ExpireMute handles expiring a mute early (keeps history)
func ExpireMute() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		muteID, err := c.ParamAsInt("muteID")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.ExpireMute{
				UserID: userID,
				MuteID: muteID,
			}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}
