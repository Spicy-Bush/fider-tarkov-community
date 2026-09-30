package jobs

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

type PurgeSponsorOpportunitiesJobHandler struct{}

func (PurgeSponsorOpportunitiesJobHandler) Schedule() string {
	return "0 15 * * * *"
}

func (PurgeSponsorOpportunitiesJobHandler) Run(ctx Context) error {
	return bus.Dispatch(ctx, &cmd.PurgeSponsorOpportunities{})
}
