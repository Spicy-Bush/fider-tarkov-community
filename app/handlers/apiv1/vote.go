package apiv1

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func AddVote() web.HandlerFunc {
	return vote(cmd.UpvotePost)
}

func AddDownVote() web.HandlerFunc {
	return vote(cmd.DownvotePost)
}

func RemoveVote() web.HandlerFunc {
	return vote(cmd.RemovePostVote)
}

func ToggleVote() web.HandlerFunc {
	return vote(cmd.TogglePostVote)
}

func vote(operation cmd.VoteOperation) web.HandlerFunc {
	return func(c *web.Context) error {
		if !c.IsAuthenticated() {
			return c.Unauthorized()
		}
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}
		var input struct {
			Revision  *int64 `json:"revision"`
		}
		change := &cmd.ApplyPostVote{Number: number, Operation: operation}
		if operation.Legacy() && c.Request.Body != "" {
			if err := c.Bind(&input); err != nil || (input.Revision != nil && *input.Revision < 0) {
				return c.BadRequest(web.Map{"message": "Invalid vote revision."})
			}
			if input.Revision != nil {
				change.Operation = cmd.SetPostVote
				change.Revision = *input.Revision
				if operation == cmd.UpvotePost {
					change.Direction = 1
				} else if operation == cmd.DownvotePost {
					change.Direction = -1
				}
			}
		}
		err = bus.Dispatch(c, change)
		if err != nil {
			return c.Failure(err)
		}
		if change.Rejection == "locked" {
			return c.BadRequest(web.Map{})
		}
		if change.Rejection == "forbidden" {
			return c.Forbidden()
		}
		if change.State.Applied || change.Operation.Legacy() {
			if change.State.Direction != 0 || operation == cmd.UpvotePost || operation == cmd.DownvotePost {
				metrics.TotalVotes.Inc()
			}
			postcache.InvalidateTenantRankings(c.Tenant().ID)
			if change.Unarchived {
				postcache.InvalidateCountPerStatus(c.Tenant().ID)
			}
		}
		if change.Operation.Legacy() {
			return c.Ok(web.Map{})
		}
		if operation == cmd.TogglePostVote {
			return c.Ok(web.Map{"voted": change.State.Direction == 1})
		}
		return c.Ok(change.State)
	}
}
