package handlers

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
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

			return c.Ok(web.Map{})
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

			return c.Ok(web.Map{})
		})
	}
}

