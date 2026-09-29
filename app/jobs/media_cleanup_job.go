package jobs

import (
	"context"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type MediaCleanupJob struct {
	running sync.Mutex
}

func (j *MediaCleanupJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	ctx := context.Background()
	if err := bus.Dispatch(ctx, &cmd.RetryMediaDeletions{}); err != nil {
		log.Error(ctx, err)
	}
}
