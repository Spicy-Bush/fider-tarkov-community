package sse

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestQueuePresenceKeepsTwoTabsOnDifferentPosts(t *testing.T) {
	hub := &Hub{tenants: make(map[int]*TenantHub)}
	first := NewClient(1, 10, "Alice", ChannelQueue)
	second := NewClient(1, 10, "Alice", ChannelQueue)
	hub.Register(first)
	hub.Register(second)
	defer hub.Unregister(first)
	defer hub.Unregister(second)

	hub.UpdatePresence(1, 10, first.ID(), ChannelQueue, 100)
	hub.UpdatePresence(1, 10, second.ID(), ChannelQueue, 200)
	viewers := hub.GetAllQueueViewers(1)
	if len(viewers) != 2 {
		t.Fatalf("two tabs lost one viewed post: %+v", viewers)
	}
}

func TestQueueDisconnectRemovesItsPresence(t *testing.T) {
	hub := &Hub{tenants: make(map[int]*TenantHub)}
	viewer := NewClient(1, 10, "Alice", ChannelQueue)
	observer := NewClient(1, 20, "Bob", ChannelQueue)
	hub.Register(viewer)
	hub.Register(observer)
	defer hub.Unregister(observer)
	hub.UpdatePresence(1, 10, viewer.ID(), ChannelQueue, 100)

	hub.Unregister(viewer)
	if viewers := hub.GetAllQueueViewers(1); len(viewers) != 0 {
		t.Fatalf("disconnected viewer remained present: %+v", viewers)
	}
}

func TestPresenceLeavesOnlyAfterTheLastTab(t *testing.T) {
	for _, channel := range []Channel{ChannelQueue, ChannelReports} {
		t.Run(string(channel), func(t *testing.T) {
			hub := &Hub{}
			first := NewClient(1, 10, "Alice", channel)
			second := NewClient(1, 10, "Alice", channel)
			observer := NewClient(1, 20, "Bob", channel)
			for _, client := range []*Client{first, second, observer} {
				hub.Register(client)
				defer hub.Unregister(client)
			}

			hub.UpdatePresence(1, 10, first.ID(), channel, 100)
			hub.UpdatePresence(1, 10, second.ID(), channel, 100)
			viewers := hub.getViewers(1, channel)
			if len(viewers[100]) != 1 || len(observer.send) != 1 {
				t.Fatalf("two tabs duplicated a user or join event: viewers=%v events=%d", viewers, len(observer.send))
			}
			<-observer.send

			hub.Unregister(first)
			if len(hub.getViewers(1, channel)[100]) != 1 || len(observer.send) != 0 {
				t.Fatal("first tab departure removed the other tab's presence")
			}
			hub.Unregister(second)
			if len(hub.getViewers(1, channel)) != 0 || len(observer.send) != 1 {
				t.Fatal("last tab departure did not remove presence exactly once")
			}
		})
	}
}

func TestPresenceOwnershipAndReconnect(t *testing.T) {
	hub := &Hub{}
	old := NewClient(1, 10, "Alice", ChannelQueue)
	replacement := NewClient(1, 10, "Alice", ChannelQueue)
	reports := NewClient(1, 10, "Alice", ChannelReports)
	for _, client := range []*Client{old, replacement, reports} {
		hub.Register(client)
		defer hub.Unregister(client)
	}
	hub.UpdatePresence(1, 10, old.ID(), ChannelQueue, 100)
	hub.Unregister(old)
	if !hub.UpdatePresence(1, 10, replacement.ID(), ChannelQueue, 200) {
		t.Fatal("replacement stream did not accept presence")
	}
	if !hub.UpdatePresence(1, 10, reports.ID(), ChannelReports, 300) {
		t.Fatal("independent report stream did not accept presence")
	}

	for _, request := range []struct {
		tenantID int
		userID   int
		id       string
		channel  Channel
	}{
		{1, 10, old.ID(), ChannelQueue},
		{2, 10, replacement.ID(), ChannelQueue},
		{1, 20, replacement.ID(), ChannelQueue},
		{1, 10, replacement.ID(), ChannelReports},
	} {
		if hub.UpdatePresence(request.tenantID, request.userID, request.id, request.channel, 0) {
			t.Fatalf("unrelated connection changed presence: %+v", request)
		}
	}
	hub.Unregister(old)
	if len(hub.getViewers(1, ChannelQueue)[200]) != 1 || len(hub.getViewers(1, ChannelReports)[300]) != 1 {
		t.Fatal("stale disconnect changed active connections")
	}
}

func TestPresenceTimeoutPreservesActiveTabs(t *testing.T) {
	hub := &Hub{}
	first := NewClient(1, 10, "Alice", ChannelQueue)
	second := NewClient(1, 10, "Alice", ChannelQueue)
	observer := NewClient(1, 20, "Bob", ChannelQueue)
	for _, client := range []*Client{first, second, observer} {
		hub.Register(client)
		defer hub.Unregister(client)
	}
	hub.UpdatePresence(1, 10, first.ID(), ChannelQueue, 100)
	hub.UpdatePresence(1, 10, second.ID(), ChannelQueue, 100)
	<-observer.send
	first.lastSeen = time.Now().Add(-2 * time.Minute)
	hub.sweepStalePresence()
	if len(hub.getViewers(1, ChannelQueue)[100]) != 1 || len(observer.send) != 0 {
		t.Fatal("expired tab removed active tab's presence")
	}

	second.lastSeen = time.Now().Add(-2 * time.Minute)
	hub.sweepStalePresence()
	if len(hub.getViewers(1, ChannelQueue)) != 0 || len(observer.send) != 1 {
		t.Fatal("last expired tab did not announce departure")
	}
	var event Message
	if err := json.Unmarshal(<-observer.send, &event); err != nil || event.Type != MsgQueueViewerLeft {
		t.Fatalf("wrong expiration event: %+v error=%v", event, err)
	}
	if !hub.UpdatePresence(1, 10, first.ID(), ChannelQueue, 100) || len(observer.send) != 1 {
		t.Fatal("visible tab could not restore presence after timeout")
	}
}

func TestConcurrentLastDepartureKeepsNewMembership(t *testing.T) {
	hub := &Hub{}
	for attempt := 0; attempt < 1000; attempt++ {
		old := NewClient(1, 10, "Alice", ChannelQueue)
		next := NewClient(1, 20, "Bob", ChannelQueue)
		hub.Register(old)
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			hub.Unregister(old)
		}()
		go func() {
			defer workers.Done()
			<-start
			hub.Register(next)
		}()
		close(start)
		workers.Wait()

		hub.BroadcastToTenant(1, MsgQueuePostNew, QueueEventPayload{PostID: attempt})
		select {
		case <-next.Send():
		default:
			t.Fatal("new connection was attached to a detached tenant hub")
		}
		hub.Unregister(next)
		if len(hub.tenants) != 0 {
			t.Fatal("empty tenant hub was retained")
		}
	}
}
