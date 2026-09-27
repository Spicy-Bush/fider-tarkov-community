package api

import (
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// ListUsers returns all registered users
func ListUsers() web.HandlerFunc {
	return func(c *web.Context) error {
		allUsers := &query.GetAllUsers{}
		if err := bus.Dispatch(c, allUsers); err != nil {
			return c.Failure(err)
		}

		if !entity.Can(c.User(), c.Tenant(), entity.ReadUserEmails) {
			for _, user := range allUsers.Result {
				user.Email = ""
			}
		}

		return c.Ok(allUsers.Result)
	}
}

func ListTaggableUsers() web.HandlerFunc {
	return func(c *web.Context) error {
		name := c.QueryParam("name")

		limit := 1
		limitStr := c.QueryParam("limit")
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err == nil && parsedLimit > 0 {
				limit = parsedLimit
				if limit > 10 {
					limit = 10
				}
			}
		}

		allUsers := &query.GetAllUsersNames{
			Query: name,
			Limit: limit,
		}
		if err := bus.Dispatch(c, allUsers); err != nil {
			return c.Failure(err)
		}
		return c.Ok(allUsers.Result)
	}
}

// CreateUser is used to create new users
func CreateUser() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateUser)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			var user *entity.User

			getByReference := &query.GetUserByProvider{Provider: "reference", UID: action.Reference}
			err := bus.Dispatch(c, getByReference)
			user = getByReference.Result

			if err != nil && errors.Cause(err) == app.ErrNotFound {
				if action.Email != "" {
					getByEmail := &query.GetUserByEmail{Email: action.Email}
					err = bus.Dispatch(c, getByEmail)
					user = getByEmail.Result
				}
				if err != nil && errors.Cause(err) == app.ErrNotFound {
					user = &entity.User{
						Tenant: c.Tenant(),
						Name:   action.Name,
						Email:  action.Email,
						Role:   enum.RoleVisitor,
					}
					err = bus.Dispatch(c, &cmd.RegisterUser{User: user})
				}
			}

			if err != nil {
				return c.Failure(err)
			}

			if action.Reference != "" && !user.HasProvider("reference") {
				if err := bus.Dispatch(c, &cmd.RegisterUserProvider{
					UserID:       user.ID,
					ProviderName: "reference",
					ProviderUID:  action.Reference,
				}); err != nil {
					return c.Failure(err)
				}
			}

			return c.Ok(web.Map{
				"id": user.ID,
			})
		})
	}
}

// GetUserProfileStats returns the user's activity stats
func GetUserProfileStats() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		permissions, err := profilePermissions(c, userID)
		if err != nil {
			return c.Failure(err)
		}
		if !permissions.ReadProfile {
			return c.NotFound()
		}

		stats := &query.GetUserProfileStats{
			UserID: userID,
		}
		if err := bus.Dispatch(c, stats); err != nil {
			return c.Failure(err)
		}
		return c.Ok(stats.Result)
	}
}

// GetUserProfileStanding returns the user's standing (warnings, mutes, bans)
func GetUserProfileStanding() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		permissions, err := profilePermissions(c, userID)
		if err != nil {
			return c.Failure(err)
		}
		if !permissions.ReadProfile {
			return c.NotFound()
		}

		standing := &query.GetUserProfileStanding{
			UserID: userID,
		}
		if err := bus.Dispatch(c, standing); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{
			"warnings": standing.Result.Warnings,
			"mutes": standing.Result.Mutes,
			"sessionPermissions": entity.PermissionsFor(c.User(), c.Tenant()),
		})
	}
}

// SearchUserContent searches through a user's content
func SearchUserContent() web.HandlerFunc {
	return func(c *web.Context) error {
		userID, err := c.ParamAsInt("userID")
		if err != nil {
			return c.NotFound()
		}

		permissions, err := profilePermissions(c, userID)
		if err != nil {
			return c.Failure(err)
		}
		if !permissions.ReadProfile {
			return c.NotFound()
		}

		limit := 10
		if limitParam, err := c.QueryParamAsInt("limit"); err == nil && limitParam > 0 {
			if limitParam > 10 {
				limit = 10
			} else {
				limit = limitParam
			}
		}

		offset := 0
		if offsetParam, err := c.QueryParamAsInt("offset"); err == nil && offsetParam >= 0 {
			offset = offsetParam
		}

		search := &query.SearchUserContent{
			UserID:      userID,
			Query:       c.QueryParam("q"),
			ContentType: c.QueryParam("contentType"),
			SortBy:      c.QueryParam("sortBy"),
			SortOrder:   c.QueryParam("sortOrder"),
			Limit:       limit,
			Offset:      offset,
		}

		voteType := c.QueryParam("voteType")
		if voteType == "up" {
			search.VoteType = 1
		} else if voteType == "down" {
			search.VoteType = -1
		}

		if err := bus.Dispatch(c, search); err != nil {
			return c.Failure(err)
		}
		return c.Ok(search.Result)
	}
}

func profilePermissions(c *web.Context, userID int) (entity.UserPermissions, error) {
	viewer := c.User()
	if viewer != nil && viewer.ID == userID {
		return viewer.AllowedActions(viewer, c.Tenant()), nil
	}

	target := &query.GetUserByID{UserID: userID}
	if err := bus.Dispatch(c, target); err != nil {
		return entity.UserPermissions{}, err
	}

	return target.Result.AllowedActions(viewer, c.Tenant()), nil
}
