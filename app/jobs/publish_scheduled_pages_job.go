package jobs

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type PublishScheduledPagesJobHandler struct {
}

func (e PublishScheduledPagesJobHandler) Schedule() string {
	// 0 * * * * * every minute
	return "0 * * * * *"
}

func (e PublishScheduledPagesJobHandler) Run(ctx Context) error {
	log.Debug(ctx, "Publishing scheduled pages")

	c := &cmd.PublishScheduledPages{}
	err := bus.Dispatch(ctx, c)
	if err != nil {
		return err
	}

	if c.Result > 0 {
		log.Debugf(ctx, "@{PagesPublished} scheduled page(s) were published", dto.Props{
			"PagesPublished": c.Result,
		})
	}

	return nil
}
