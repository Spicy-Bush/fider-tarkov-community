package sse

import (
	"encoding/json"
	"sync"
)

type PagePositionID struct {
	Client uint64 `json:"client"`
	Clock  uint64 `json:"clock"`
}

type PagePosition struct {
	Type  *PagePositionID `json:"type,omitempty"`
	Name  string          `json:"tname,omitempty"`
	Item  *PagePositionID `json:"item,omitempty"`
	Assoc int             `json:"assoc"`
}

func (position *PagePosition) Valid() bool {
	if position == nil {
		return true
	}
	if position.Name != "" && position.Name != "content" {
		return false
	}
	if position.Type != nil || position.Assoc < -1 || position.Assoc > 1 {
		return false
	}
	if position.Item != nil && (position.Item.Client > 1<<53-1 || position.Item.Clock > 1<<53-1) {
		return false
	}
	return position.Name == "content" || position.Item != nil
}

type PageEditor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type PageEvent struct {
	Type     string        `json:"type"`
	ClientID uint64        `json:"clientId,omitempty"`
	User     *PageEditor   `json:"user,omitempty"`
	Update   []byte        `json:"update,omitempty"`
	Anchor   *PagePosition `json:"anchor,omitempty"`
	Head     *PagePosition `json:"head,omitempty"`
}

type pageRoom struct {
	tenantID int
	pageID   int
}

type PageClient struct {
	room     pageRoom
	clientID uint64
	user     PageEditor
	cursor   *PageEvent
	send     chan []byte
}

func (client *PageClient) Send() <-chan []byte {
	return client.send
}

type PageHub struct {
	mu    sync.Mutex
	rooms map[pageRoom]map[uint64]*PageClient
}

var PageEditors PageHub

func (hub *PageHub) Register(tenantID, pageID int, clientID uint64, user PageEditor) (*PageClient, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	room := pageRoom{tenantID: tenantID, pageID: pageID}
	clients := hub.rooms[room]
	if previous := clients[clientID]; previous != nil {
		if previous.user.ID != user.ID {
			return nil, false
		}
		close(previous.send)
	}

	if clients == nil {
		clients = make(map[uint64]*PageClient)
		if hub.rooms == nil {
			hub.rooms = make(map[pageRoom]map[uint64]*PageClient)
		}
		hub.rooms[room] = clients
	}

	client := &PageClient{
		room:     room,
		clientID: clientID,
		user:     user,
		send:     make(chan []byte, len(clients)+32),
	}
	for _, existing := range clients {
		if existing.cursor != nil {
			data, _ := json.Marshal(existing.cursor)
			client.send <- data
		}
	}
	clients[clientID] = client
	return client, true
}

func (hub *PageHub) Unregister(client *PageClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	clients := hub.rooms[client.room]
	if clients[client.clientID] != client {
		return
	}
	delete(clients, client.clientID)
	close(client.send)
	hub.broadcast(client.room, PageEvent{Type: "leave", ClientID: client.clientID})
}

func (hub *PageHub) Cursor(tenantID, pageID, userID int, clientID uint64, anchor, head *PagePosition) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	room := pageRoom{tenantID: tenantID, pageID: pageID}
	client := hub.rooms[room][clientID]
	if client == nil || client.user.ID != userID {
		return false
	}

	client.cursor = &PageEvent{
		Type:     "cursor",
		ClientID: clientID,
		User:     &client.user,
		Anchor:   anchor,
		Head:     head,
	}
	hub.broadcast(room, *client.cursor)
	return true
}

func (hub *PageHub) Broadcast(tenantID, pageID int, event PageEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.broadcast(pageRoom{tenantID: tenantID, pageID: pageID}, event)
}

func (hub *PageHub) broadcast(room pageRoom, event PageEvent) {
	data, _ := json.Marshal(event)
	clients := hub.rooms[room]
	var disconnected []uint64
	for id, client := range clients {
		select {
		case client.send <- data:
		default:
			// Reconnection reloads the document and presence after a slow reader falls behind.
			delete(clients, id)
			close(client.send)
			disconnected = append(disconnected, id)
		}
	}
	if len(clients) == 0 {
		delete(hub.rooms, room)
	}
	for _, id := range disconnected {
		hub.broadcast(room, PageEvent{Type: "leave", ClientID: id})
	}
}
