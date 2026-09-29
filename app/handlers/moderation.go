package handlers

import (
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ApprovePostModeration() web.HandlerFunc {
	return func(c *web.Context) error {
		postID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			setPending := &cmd.SetModerationPending{
				ContentType: "post",
				ContentID:   postID,
				Pending:     false,
			}
			if err := bus.Dispatch(c, setPending); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func ApproveCommentModeration() web.HandlerFunc {
	return func(c *web.Context) error {
		commentID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			setPending := &cmd.SetModerationPending{
				ContentType: "comment",
				ContentID:   commentID,
				Pending:     false,
			}
			if err := bus.Dispatch(c, setPending); err != nil {
				return c.Failure(err)
			}

			return api.CommentResponse(c, setPending.Discussion, setPending.Comment)
		})
	}
}

func HidePostModeration() web.HandlerFunc {
	return func(c *web.Context) error {
		postID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			setPending := &cmd.SetModerationPending{
				ContentType: "post",
				ContentID:   postID,
				Pending:     true,
			}
			if err := bus.Dispatch(c, setPending); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func HideCommentModeration() web.HandlerFunc {
	return func(c *web.Context) error {
		commentID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			setPending := &cmd.SetModerationPending{
				ContentType: "comment",
				ContentID:   commentID,
				Pending:     true,
			}
			if err := bus.Dispatch(c, setPending); err != nil {
				return c.Failure(err)
			}

			return api.CommentResponse(c, setPending.Discussion, setPending.Comment)
		})
	}
}

func ListModerationChecks() web.HandlerFunc {
	return func(c *web.Context) error {
		checks := &cmd.ListModerationFailures{}
		if err := bus.Dispatch(c, checks); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{"checks": checks.Result, "total": checks.Total, "failed": checks.Failed, "enabled": env.IsOpenAIModerationEnabled()})
	}
}

func RetryModerationChecks() web.HandlerFunc {
	return func(c *web.Context) error {
		if !env.IsOpenAIModerationEnabled() {
			return c.BadRequest(web.Map{"errors": []web.Map{{"message": "Automatic checks are disabled."}}})
		}
		retry := &cmd.RetryModerationFailures{}
		if err := bus.Dispatch(c, retry); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{"count": retry.Count})
	}
}

func ProfileModerationStatus() web.HandlerFunc {
	return func(c *web.Context) error {
		userID := c.User().ID
		checks := &cmd.GetProfileModeration{UserID: userID}
		user := &query.GetUserByID{UserID: userID}

		if err := bus.Dispatch(c, checks, user); err != nil {
			return c.Failure(err)
		}

		changes := make([]web.Map, 0, len(checks.Result))

		for _, check := range checks.Result {
			unreviewed := check.State == "pending" || check.State == "running" || check.State == "failed"
			matchesName := check.ContentType == "name" && check.Text == user.Result.Name
			matchesAvatar := check.ContentType == "avatar" && len(check.BlobKeys) == 1 &&
				check.BlobKeys[0] == user.Result.AvatarBlobKey
			change := web.Map{
				"field":     check.ContentType,
				"state":     check.State,
				"value":     check.Text,
				"revision":  check.Revision,
				"published": unreviewed && (matchesName || matchesAvatar),
			}

			if check.ContentType == "avatar" && len(check.BlobKeys) == 1 && check.State != "rejected" {
				change["previewURL"] = fmt.Sprintf("/api/user/moderation/avatar?revision=%d", check.Revision)
			}

			changes = append(changes, change)
		}

		return c.Ok(web.Map{
			"changes":    changes,
			"name":       user.Result.Name,
			"avatarURL":  user.Result.AvatarURL,
			"avatarType": user.Result.AvatarType,
		})
	}
}

func PreviewProfileAvatar() web.HandlerFunc {
	return func(c *web.Context) error {
		c.Response.Header().Set("Cache-Control", "private, no-store")
		checks := &cmd.GetProfileModeration{UserID: c.User().ID}
		if err := bus.Dispatch(c, checks); err != nil {
			return c.Failure(err)
		}
		for _, check := range checks.Result {
			if check.ContentType == "avatar" && (check.State == "pending" || check.State == "running" || check.State == "failed") &&
				fmt.Sprint(check.Revision) == c.QueryParam("revision") {
				image := &query.GetBlobByKey{Key: check.BlobKeys[0], AllowUnpublishedAvatar: true, MaxBytes: imagic.MaxImageBytes}
				if err := bus.Dispatch(c, image); err != nil {
					return c.Failure(err)
				}
				return c.Image(image.Result.ContentType, image.Result.Content)
			}
		}
		return c.NotFound()
	}
}
