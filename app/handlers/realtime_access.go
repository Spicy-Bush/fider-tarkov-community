package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

const realtimeAccessBatchSize = 64

type realtimeAccessResult struct {
	access query.RealtimeAccess
	err    error
}

type realtimeAccessRequest struct {
	ctx    context.Context
	viewer query.RealtimeViewer
	result chan realtimeAccessResult
}

var realtimeAccessRequests = sync.OnceValue(func() chan<- realtimeAccessRequest {
	requests := make(chan realtimeAccessRequest, realtimeAccessBatchSize)
	go serveRealtimeAccess(context.Background(), requests)
	return requests
})

func checkRealtimeAccess(ctx context.Context, viewer query.RealtimeViewer) (query.RealtimeAccess, error) {
	request := realtimeAccessRequest{
		ctx:    ctx,
		viewer: viewer,
		result: make(chan realtimeAccessResult, 1),
	}
	select {
	case realtimeAccessRequests() <- request:
	case <-ctx.Done():
		return query.RealtimeAccess{}, ctx.Err()
	}

	select {
	case result := <-request.result:
		if err := ctx.Err(); err != nil {
			return query.RealtimeAccess{}, err
		}
		return result.access, result.err
	case <-ctx.Done():
		return query.RealtimeAccess{}, ctx.Err()
	}
}

func serveRealtimeAccess(ctx context.Context, requests <-chan realtimeAccessRequest) {
	var buffer [realtimeAccessBatchSize]realtimeAccessRequest
	for {
		batch := buffer[:0]
		select {
		case request := <-requests:
			batch = append(batch, request)
		case <-ctx.Done():
			return
		}

	collect:
		for len(batch) < realtimeAccessBatchSize {
			select {
			case request := <-requests:
				batch = append(batch, request)
			default:
				break collect
			}
		}

		access := &query.GetRealtimeAccess{Viewers: make([]query.RealtimeViewer, 0, len(batch))}
		active := batch[:0]
		for _, request := range batch {
			if request.ctx.Err() == nil {
				active = append(active, request)
				access.Viewers = append(access.Viewers, request.viewer)
			}
		}
		if len(active) == 0 {
			clear(batch)
			continue
		}

		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := bus.Dispatch(checkCtx, access)
		cancel()
		for index, request := range active {
			result := realtimeAccessResult{err: err}
			if err == nil {
				result.access = access.Result[index]
			}
			request.result <- result
		}
		clear(batch)
	}
}
