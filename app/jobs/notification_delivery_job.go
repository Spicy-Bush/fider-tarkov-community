package jobs

import (
	"context"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

type NotificationDeliveryJob struct {
	running sync.Mutex
}

func (j *NotificationDeliveryJob) Run() {
	if !j.running.TryLock() {
		return
	}

	defer j.running.Unlock()

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		found, err := tasks.DeliverPendingNotification(ctx)
		if err != nil {
			log.Error(ctx, err)
		}

		if !found {
			return
		}
	}
}
