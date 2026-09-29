package sse_test

import (
	"encoding/json"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/stretchr/testify/require"
)

func nextPageEvent(t *testing.T, client *sse.PageClient) sse.PageEvent {
	t.Helper()
	select {
	case data, open := <-client.Send():
		require.True(t, open)
		var event sse.PageEvent
		require.NoError(t, json.Unmarshal(data, &event))
		return event
	default:
		t.Fatal("expected a Page event")
		return sse.PageEvent{}
	}
}

func TestPagePresenceStaysWithinItsTenantAndPage(t *testing.T) {
	var hub sse.PageHub
	alice, registered := hub.Register(1, 10, 100, sse.PageEditor{ID: 1, Name: "Alice"})
	require.True(t, registered)
	bob, registered := hub.Register(1, 10, 200, sse.PageEditor{ID: 2, Name: "Bob"})
	require.True(t, registered)
	otherPage, _ := hub.Register(1, 11, 300, sse.PageEditor{ID: 3})
	otherTenant, _ := hub.Register(2, 10, 400, sse.PageEditor{ID: 4})
	t.Cleanup(func() {
		for _, client := range []*sse.PageClient{alice, bob, otherPage, otherTenant} {
			hub.Unregister(client)
		}
	})

	position := &sse.PagePosition{Name: "content", Item: &sse.PagePositionID{Client: 100, Clock: 2}}
	require.False(t, hub.Cursor(1, 10, 2, 100, position, position))
	require.True(t, hub.Cursor(1, 10, 1, 100, position, position))
	event := nextPageEvent(t, bob)
	require.Equal(t, "cursor", event.Type)
	require.Equal(t, "Alice", event.User.Name)
	require.Equal(t, position, event.Anchor)
	_ = nextPageEvent(t, alice)

	for _, client := range []*sse.PageClient{otherPage, otherTenant} {
		select {
		case <-client.Send():
			t.Fatal("presence crossed a Page or tenant boundary")
		default:
		}
	}

	hub.Unregister(alice)
	leave := nextPageEvent(t, bob)
	require.Equal(t, "leave", leave.Type)
	require.Equal(t, uint64(100), leave.ClientID)
}

func TestPageReconnectKeepsCurrentConnectionAndPresence(t *testing.T) {
	var hub sse.PageHub
	first, _ := hub.Register(1, 10, 100, sse.PageEditor{ID: 1, Name: "Alice"})
	position := &sse.PagePosition{Name: "content"}
	require.True(t, hub.Cursor(1, 10, 1, 100, position, position))
	_ = nextPageEvent(t, first)

	_, registered := hub.Register(1, 10, 100, sse.PageEditor{ID: 2, Name: "Bob"})
	require.False(t, registered)
	observer, _ := hub.Register(1, 10, 200, sse.PageEditor{ID: 2, Name: "Bob"})
	require.Equal(t, "Alice", nextPageEvent(t, observer).User.Name)

	replacement, registered := hub.Register(1, 10, 100, sse.PageEditor{ID: 1, Name: "Alice"})
	require.True(t, registered)
	hub.Unregister(first)
	require.True(t, hub.Cursor(1, 10, 1, 100, position, position))
	require.Equal(t, "cursor", nextPageEvent(t, observer).Type)

	hub.Unregister(replacement)
	hub.Unregister(observer)
}

func TestSlowPageReaderDisconnectsAndRemovesItsCursor(t *testing.T) {
	var hub sse.PageHub
	slow, _ := hub.Register(1, 10, 100, sse.PageEditor{ID: 1, Name: "Alice"})
	reader, _ := hub.Register(1, 10, 200, sse.PageEditor{ID: 2, Name: "Bob"})
	position := &sse.PagePosition{Name: "content"}
	require.True(t, hub.Cursor(1, 10, 1, 100, position, position))
	_ = nextPageEvent(t, reader)

	left := false
	for range 40 {
		hub.Broadcast(1, 10, sse.PageEvent{Type: "update", Update: []byte{0, 0}})
		require.Equal(t, "update", nextPageEvent(t, reader).Type)
		select {
		case data := <-reader.Send():
			var event sse.PageEvent
			require.NoError(t, json.Unmarshal(data, &event))
			require.Equal(t, "leave", event.Type)
			require.Equal(t, uint64(100), event.ClientID)
			left = true
		default:
		}
	}
	require.True(t, left)
	require.False(t, hub.Cursor(1, 10, 1, 100, position, position))
	hub.Unregister(slow)
	hub.Unregister(reader)

	reconnected, registered := hub.Register(1, 10, 100, sse.PageEditor{ID: 1, Name: "Alice"})
	require.True(t, registered)
	require.True(t, hub.Cursor(1, 10, 1, 100, position, position))
	hub.Unregister(reconnected)
}
