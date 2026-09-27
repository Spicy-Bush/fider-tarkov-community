package api

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func AddVote() web.HandlerFunc {
	return vote(1)
}

func AddDownVote() web.HandlerFunc {
	return vote(-1)
}

func RemoveVote() web.HandlerFunc {
	return vote(0)
}

func vote(direction int) web.HandlerFunc {
	return func(c *web.Context) error {
		if !c.IsAuthenticated() {
			return c.Unauthorized()
		}

		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		var input struct {
			Revision *int64 `json:"revision"`
		}
		if err := c.Bind(&input); err != nil || input.Revision == nil || *input.Revision < 0 {
			return c.BadRequest(web.Map{"message": "Invalid vote revision."})
		}

		change := &cmd.ApplyPostVote{
			Number:    number,
			Direction: direction,
			Revision:  *input.Revision,
		}
		err = bus.Dispatch(c, change)
		if err != nil {
			return c.Failure(err)
		}

		if change.State.Applied {
			if change.State.Direction != 0 {
				metrics.TotalVotes.Inc()
			}

			postcache.InvalidateTenantRankings(c.Tenant().ID)
			if change.Unarchived {
				postcache.InvalidateCountPerStatus(c.Tenant().ID)
			}
		}

		return c.Ok(change.State)
	}
}
