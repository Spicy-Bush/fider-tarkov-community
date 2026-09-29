package sse

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	MsgConnectionReady = "connection.ready"
	MsgReportsChanged  = "reports.changed"

	MsgQueuePostNew      = "queue.post_new"
	MsgQueuePostTagged   = "queue.post_tagged"
	MsgQueueViewerJoined = "queue.viewer_joined"
	MsgQueueViewerLeft   = "queue.viewer_left"
)

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type ClientInfo struct {
	UserID     int    `json:"userId"`
	UserName   string `json:"userName"`
	AvatarURL  string `json:"avatarURL,omitempty"`
	AvatarType string `json:"avatarType,omitempty"`
	Role       string `json:"role,omitempty"`
	Status     string `json:"status,omitempty"`
}

type QueueEventPayload struct {
	PostID           int `json:"postId"`
	PostNumber       int `json:"postNumber,omitempty"`
	TaggedByUserID   int `json:"taggedByUserId,omitempty"`
	UntaggedByUserID int `json:"untaggedByUserId,omitempty"`
}

type QueueViewerEventPayload struct {
	PostID   int    `json:"postId"`
	UserID   int    `json:"userId"`
	UserName string `json:"userName"`
}

type TenantHub struct {
	clients map[string]*Client
}

type Hub struct {
	tenants map[int]*TenantHub
	mu      sync.RWMutex
}

var defaultHub *Hub
var once sync.Once

func GetHub() *Hub {
	once.Do(func() {
		defaultHub = &Hub{}
		go defaultHub.presenceSweep()
	})
	return defaultHub
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	th := h.tenants[client.tenantID]
	if th == nil {
		th = &TenantHub{clients: make(map[string]*Client)}
		if h.tenants == nil {
			h.tenants = make(map[int]*TenantHub)
		}
		h.tenants[client.tenantID] = th
	}
	th.clients[client.id] = client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	th := h.tenants[client.tenantID]
	if th == nil || th.clients[client.id] != client {
		return
	}
	delete(th.clients, client.id)
	close(client.send)
	if client.itemID != 0 && !hasViewer(th, client.channel, client.userID, client.itemID) {
		broadcastPresence(th, client, client.itemID, false)
	}
	if len(th.clients) == 0 {
		delete(h.tenants, client.tenantID)
	}
}

func getChannelForMessage(messageType string) Channel {
	switch messageType {
	case MsgReportsChanged:
		return ChannelReports
	case MsgQueuePostNew, MsgQueuePostTagged, MsgQueueViewerJoined, MsgQueueViewerLeft:
		return ChannelQueue
	default:
		return ""
	}
}

func (h *Hub) BroadcastToTenant(tenantID int, messageType string, payload interface{}) {
	data, err := json.Marshal(&Message{Type: messageType, Payload: payload})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	if th := h.tenants[tenantID]; th != nil {
		broadcast(th, getChannelForMessage(messageType), data)
	}
}

func broadcast(th *TenantHub, channel Channel, data []byte) {
	for _, client := range th.clients {
		if client.channel == channel {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

func hasViewer(th *TenantHub, channel Channel, userID, itemID int) bool {
	for _, client := range th.clients {
		if client.channel == channel && client.userID == userID && client.itemID == itemID {
			return true
		}
	}
	return false
}

func broadcastPresence(th *TenantHub, client *Client, itemID int, joined bool) {
	message := Message{Type: MsgReportsChanged}
	if client.channel == ChannelQueue {
		message.Type = MsgQueueViewerLeft
		if joined {
			message.Type = MsgQueueViewerJoined
		}
		message.Payload = QueueViewerEventPayload{PostID: itemID, UserID: client.userID, UserName: client.userName}
	}
	data, _ := json.Marshal(message)
	broadcast(th, client.channel, data)
}

func (h *Hub) UpdatePresence(tenantID, userID int, connectionID string, channel Channel, itemID int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	th := h.tenants[tenantID]
	if th == nil {
		return false
	}
	client := th.clients[connectionID]
	if client == nil || client.userID != userID || client.channel != channel {
		return false
	}
	client.lastSeen = time.Now()
	if client.itemID == itemID {
		return true
	}

	previous := client.itemID
	client.itemID = 0
	if previous != 0 && !hasViewer(th, channel, userID, previous) {
		broadcastPresence(th, client, previous, false)
	}
	if itemID != 0 {
		alreadyPresent := hasViewer(th, channel, userID, itemID)
		client.itemID = itemID
		if !alreadyPresent {
			broadcastPresence(th, client, itemID, true)
		}
	}
	return true
}

type ReportViewers struct {
	ReportID int           `json:"reportId"`
	Viewers  []*ClientInfo `json:"viewers"`
}

func (h *Hub) GetAllActiveViewers(tenantID int) []ReportViewers {
	byItem := h.getViewers(tenantID, ChannelReports)
	result := make([]ReportViewers, 0, len(byItem))
	for id, viewers := range byItem {
		result = append(result, ReportViewers{ReportID: id, Viewers: viewers})
	}
	return result
}

type QueuePostViewers struct {
	PostID  int           `json:"postId"`
	Viewers []*ClientInfo `json:"viewers"`
}

func (h *Hub) GetAllQueueViewers(tenantID int) []QueuePostViewers {
	byItem := h.getViewers(tenantID, ChannelQueue)
	result := make([]QueuePostViewers, 0, len(byItem))
	for id, viewers := range byItem {
		result = append(result, QueuePostViewers{PostID: id, Viewers: viewers})
	}
	return result
}

func (h *Hub) getViewers(tenantID int, channel Channel) map[int][]*ClientInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	byItem := make(map[int][]*ClientInfo)
	th := h.tenants[tenantID]
	if th == nil {
		return byItem
	}
	seen := make(map[[2]int]bool)
	for _, client := range th.clients {
		if client.channel != channel || client.itemID == 0 {
			continue
		}
		key := [2]int{client.itemID, client.userID}
		if seen[key] {
			continue
		}
		seen[key] = true
		byItem[client.itemID] = append(byItem[client.itemID], &ClientInfo{UserID: client.userID, UserName: client.userName})
	}
	return byItem
}

func (h *Hub) presenceSweep() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.sweepStalePresence()
	}
}

func (h *Hub) sweepStalePresence() {
	h.mu.Lock()
	defer h.mu.Unlock()

	threshold := time.Now().Add(-time.Minute)
	for _, th := range h.tenants {
		for _, client := range th.clients {
			if client.itemID == 0 || !client.lastSeen.Before(threshold) {
				continue
			}
			itemID := client.itemID
			client.itemID = 0
			if !hasViewer(th, client.channel, client.userID, itemID) {
				broadcastPresence(th, client, itemID, false)
			}
		}
	}
}
