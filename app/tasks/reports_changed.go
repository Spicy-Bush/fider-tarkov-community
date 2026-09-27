package tasks

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

func ReportsChanged() worker.Task {
	return worker.Task{
		Name: "Refresh report views",
		Job: func(ctx *worker.Context) error {
			sse.GetHub().BroadcastToTenant(ctx.Tenant().ID, sse.MsgReportsChanged, nil)
			return nil
		},
	}
}
