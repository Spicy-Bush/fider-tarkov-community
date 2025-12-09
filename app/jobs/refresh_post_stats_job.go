package jobs

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type RefreshPostStatsJobHandler struct {
}

func (e RefreshPostStatsJobHandler) Schedule() string {
	// every minute
	return "0 */1 * * * *"
}

func (e RefreshPostStatsJobHandler) Run(ctx Context) error {
	log.Debug(ctx, "refreshing post stats for 30-day window metrics")

	c := &cmd.RefreshPostStats{
		Since: ctx.LastSuccessfulRun,
	}
	err := bus.Dispatch(ctx, c)
	if err != nil {
		return err
	}

	log.Debugf(ctx, "@{RowsUpdated} posts were updated", dto.Props{
		"RowsUpdated": c.RowsUpdated,
	})

	return nil
}
