package jobs

import (
	"context"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type MediaInventoryJob struct {
	Context context.Context
	running sync.Mutex
}

func (j *MediaInventoryJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	for j.Context.Err() == nil {
		request := &cmd.ImportMediaInventory{}
		if err := bus.Dispatch(j.Context, request); err != nil {
			log.Error(j.Context, err)
			return
		}
		if !request.Found {
			return
		}
	}
}
