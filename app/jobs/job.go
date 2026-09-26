package jobs

import (
	"context"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

type Handler interface {
	Run(ctx Context) error
	Schedule() string
}

type fiderJob struct {
	Name    string
	Handler Handler
}

func NewJob(ctx context.Context, name string, handler Handler) (string, fiderJob) {
	schedule := handler.Schedule()
	log.Debugf(ctx, "Job '@{JobName}' scheduled to run '@{Schedule}'", dto.Props{
		"JobName":  name,
		"Schedule": schedule,
	})
	return schedule, fiderJob{Name: name, Handler: handler}
}

func (j fiderJob) Run() {
	ctx, trx, err := newJobContext()
	if err != nil {
		log.Error(ctx, err)
		return
	}
	defer trx.Rollback()

	start := time.Now()

	logFinish := func() {
		elapsedMs := time.Since(start).Nanoseconds() / int64(time.Millisecond)
		log.Debugf(ctx, "Job '@{JobName}' finished in @{ElapsedMs:magenta}ms", dto.Props{
			"ElapsedMs": elapsedMs,
			"JobName":   j.Name,
		})

	}

	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, trx.RollbackWithCause(errors.Panicked(r)))
			setLastFailedRun(j.Name, start)
		}
	}()

	log.Debugf(ctx, "Job '@{JobName}' started", dto.Props{
		"JobName": j.Name,
	})

	locked, err := dbx.TryLock(trx, j.Name)
	if err != nil {
		log.Error(ctx, err)
		return
	}
	if !locked {
		log.Debugf(ctx, "Job '@{JobName}' skipped, could not acquire lock", dto.Props{
			"JobName": j.Name,
		})
		return
	}

	defer logFinish()
	ctx.LastSuccessfulRun = getLastSuccessfulRun(ctx, j.Name)

	err = j.Handler.Run(ctx)
	// Avoid releasing the lock while other replicas are starting this tick.
	if remaining := time.Second - time.Since(start); remaining > 0 {
		time.Sleep(remaining)
	}
	if err != nil {
		log.Error(ctx, trx.RollbackWithCause(err))
		setLastFailedRun(j.Name, start)
		return
	}
	if err := trx.Commit(); err != nil {
		log.Error(ctx, err)
		setLastFailedRun(j.Name, start)
		return
	}
	setLastSuccessfulRun(j.Name, start)
}

func newJobContext() (Context, *dbx.Trx, error) {
	ctx := context.Background()
	ctx = log.WithProperties(ctx, dto.Props{
		log.PropertyKeyContextID: rand.String(32),
		log.PropertyKeyTag:       "JOBS",
	})

	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		log.Error(ctx, err)
		return Context{Context: ctx}, nil, err
	}

	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	return Context{Context: ctx}, trx, nil
}
