package handlers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestRealtimeAccessBoundsBatchesAndKeepsResultsWithTheirViewer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	canceled, cancelRequest := context.WithCancel(ctx)
	cancelRequest()
	requests := make(chan realtimeAccessRequest, 130)
	results := make([]chan realtimeAccessResult, 130)
	for index := range results {
		requestContext := ctx
		if index == 3 {
			requestContext = canceled
		}
		results[index] = make(chan realtimeAccessResult, 1)
		requests <- realtimeAccessRequest{
			ctx:    requestContext,
			viewer: query.RealtimeViewer{TenantID: index % 3, UserID: index},
			result: results[index],
		}
	}

	var calls atomic.Int32
	bus.AddHandler(func(ctx context.Context, q *query.GetRealtimeAccess) error {
		calls.Add(1)
		if len(q.Viewers) > realtimeAccessBatchSize {
			t.Errorf("oversized batch: %d", len(q.Viewers))
		}
		q.Result = make([]query.RealtimeAccess, len(q.Viewers))
		for index, viewer := range q.Viewers {
			if viewer.UserID == 3 {
				t.Error("canceled queued request reached the database")
			}
			q.Result[index].Queue = viewer.TenantID == 1
			q.Result[index].Reports = viewer.UserID%2 == 0
		}
		return nil
	})
	finished := make(chan struct{})
	go func() {
		serveRealtimeAccess(ctx, requests)
		close(finished)
	}()
	defer func() {
		cancel()
		<-finished
	}()

	for index, result := range results {
		if index == 3 {
			continue
		}
		select {
		case result := <-result:
			if result.err != nil || result.access.Queue != (index%3 == 1) || result.access.Reports != (index%2 == 0) {
				t.Fatalf("viewer %d got another viewer's result: %+v", index, result)
			}
		case <-time.After(time.Second):
			t.Fatal("batch did not finish")
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("130 pending checks used %d batches", calls.Load())
	}
}

func TestRealtimeAccessCancellationRevocationAndRecovery(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var calls atomic.Int32
	var allowed atomic.Bool
	var unavailable atomic.Bool
	allowed.Store(true)
	lookupFailure := errors.New("permission lookup unavailable")
	bus.AddHandler(func(ctx context.Context, q *query.GetRealtimeAccess) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		if unavailable.Load() {
			return lookupFailure
		}
		q.Result = make([]query.RealtimeAccess, len(q.Viewers))
		for index := range q.Result {
			q.Result[index] = query.RealtimeAccess{Queue: allowed.Load(), Reports: allowed.Load()}
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	first, cancelFirst := context.WithCancel(ctx)
	defer cancelFirst()
	viewer := query.RealtimeViewer{TenantID: 1, UserID: 1}
	canceled := make(chan error, 1)
	go func() {
		_, err := checkRealtimeAccess(first, viewer)
		canceled <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("permission lookup did not start")
	}
	cancelFirst()
	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost caller cancellation: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("canceled caller waited for the database")
	}
	releaseOnce.Do(func() { close(release) })

	for _, state := range []string{"allowed", "revoked", "database failure", "recovered"} {
		allowed.Store(state != "revoked")
		unavailable.Store(state == "database failure")
		access, err := checkRealtimeAccess(ctx, viewer)
		if state == "database failure" {
			if !errors.Is(err, lookupFailure) {
				t.Fatalf("lookup failure was lost: %v", err)
			}
			continue
		}
		if err != nil || access.Queue != allowed.Load() || access.Reports != allowed.Load() {
			t.Fatalf("%s returned stale or missing access: %+v, %v", state, access, err)
		}
	}
}
