package jobs

import (
	"context"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

type PostNotificationDeliveryJob struct {
	running sync.Mutex
}

func (j *PostNotificationDeliveryJob) Run() {
	if !j.running.TryLock() {
		return
	}

	defer j.running.Unlock()

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		found, err := tasks.DeliverPendingPostNotification(ctx)
		if err != nil {
			log.Error(ctx, err)
		}

		if !found {
			return
		}
	}
}
