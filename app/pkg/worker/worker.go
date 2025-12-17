package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

type MiddlewareFunc func(Job) Job

type Job func(c *Context) error

type Task struct {
	OriginContext context.Context
	Name          string
	Job           Job
}

type Worker interface {
	Run(id string)
	Enqueue(task Task)
	Use(middleware MiddlewareFunc)
	Length() int64
	Shutdown(ctx context.Context) error
}

type BackgroundWorker struct {
	context.Context
	queue      chan Task
	len        atomic.Int64
	middleware MiddlewareFunc
}

var maxQueueSize = 100

func New() *BackgroundWorker {
	ctx := context.Background()

	ctx = log.WithProperties(ctx, dto.Props{
		log.PropertyKeyContextID: rand.String(32),
		log.PropertyKeyTag:       "BGW",
	})

	return &BackgroundWorker{
		Context: ctx,
		queue:   make(chan Task, maxQueueSize),
		middleware: func(next Job) Job {
			return next
		},
	}
}

func (w *BackgroundWorker) Run(workerID string) {
	log.Infof(w, "Starting worker @{WorkerID:magenta}.", dto.Props{
		"WorkerID": workerID,
	})
	for task := range w.queue {
		c := NewContext(w, workerID, task)
		_ = w.middleware(task.Job)(c)
		w.len.Add(-1)
	}
}

func (w *BackgroundWorker) Shutdown(ctx context.Context) error {
	if w.Length() > 0 {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			count := w.Length()
			if count == 0 {
				return nil
			}

			log.Infof(w, "Waiting for work queue: @{Count}", dto.Props{
				"Count": count,
			})

			select {
			case <-ctx.Done():
				return errors.New("timeout waiting for worker queue")
			case <-ticker.C:
			}
		}
	}
	return nil
}

func (w *BackgroundWorker) Enqueue(task Task) {
	w.len.Add(1)
	w.queue <- task
}

func (w *BackgroundWorker) Length() int64 {
	return w.len.Load()
}

func (w *BackgroundWorker) Use(middleware MiddlewareFunc) {
	w.middleware = middleware
}
