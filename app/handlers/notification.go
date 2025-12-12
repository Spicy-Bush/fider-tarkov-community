package handlers

import (
	"net/http"
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// GetAllNotifications will get all the notifications for the new modal
func GetAllNotifications() web.HandlerFunc {
	return func(c *web.Context) error {
		notificationType := c.QueryParam("type")

		pageParam := c.QueryParam("page")
		page := 1
		if pageParam != "" {
			var err error
			page, err = strconv.Atoi(pageParam)
			if err != nil || page < 1 {
				page = 1
			}
		}

		perPageParam := c.QueryParam("perPage")
		perPage := 10
		if perPageParam != "" {
			var err error
			perPage, err = strconv.Atoi(perPageParam)
			if err != nil || perPage < 1 {
				perPage = 10
			}
		}

		q := &query.GetActiveNotifications{
			Type:    notificationType,
			Page:    page,
			PerPage: perPage,
		}

		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"notifications": q.Result,
			"total":         q.TotalCount,
			"page":          q.Page,
			"perPage":       q.PerPage,
		})
	}
}

// TotalUnreadNotifications returns the total number of unread notifications
// For staff members, it also includes the pending reports count and post queue count
func TotalUnreadNotifications() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.CountUnreadNotifications{}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		response := web.Map{
			"total": q.Result,
		}

		if c.User() != nil && (c.User().IsCollaborator() || c.User().IsModerator() || c.User().IsAdministrator()) {
			reportCount := &query.CountPendingReports{}
			if err := bus.Dispatch(c, reportCount); err == nil {
				response["pendingReports"] = reportCount.Result
			}
		}

		if c.User() != nil && (c.User().IsHelper() || c.User().IsCollaborator() || c.User().IsModerator() || c.User().IsAdministrator()) {
			queueCount := &query.CountUntaggedPosts{}
			if err := bus.Dispatch(c, queueCount); err == nil {
				response["queueCount"] = queueCount.Result
			}
		}

		return c.Ok(response)
	}
}

// Notifications is the home for unread and recent notifications
func Notifications() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.GetActiveNotifications{
			Page:    1,
			PerPage: 10,
		}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "MyNotifications/MyNotifications.page",
			Title: "Notifications",
			Data: web.Map{
				"notifications": q.Result,
				"total":         q.TotalCount,
				"page":          q.Page,
				"perPage":       q.PerPage,
			},
		})
	}
}

// ReadNotification marks it as read and redirect to its content
func ReadNotification() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.Failure(err)
		}

		q := &query.GetNotificationByID{ID: id}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		// store link before transaction since we need it afterwards too
		link := q.Result.Link

		err = c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.MarkNotificationAsRead{ID: q.Result.ID}); err != nil {
				return c.Failure(err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		return c.Redirect(c.BaseURL() + link)
	}
}

// ReadAllNotifications marks all unread notifications as read
func ReadAllNotifications() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.MarkAllNotificationsAsRead{}); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// PurgeReadNotifications purges all read notifications for current user
func PurgeReadNotifications() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.WithTransaction(func() error {
			purgeCmd := &cmd.PurgeReadNotifications{}
			if err := bus.Dispatch(c, purgeCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{
				"purgedCount": purgeCmd.NumOfPurgedNotifications,
			})
		})
	}
}
