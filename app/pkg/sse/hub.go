package sse

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	MsgReportNew          = "report.new"
	MsgReportAssigned     = "report.assigned"
	MsgReportUnassigned   = "report.unassigned"
	MsgReportResolved     = "report.resolved"
	MsgReportViewerJoined = "report.viewer_joined"
	MsgReportViewerLeft   = "report.viewer_left"

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

type ReportEventPayload struct {
	ReportID     int         `json:"reportId"`
	ReportedType string      `json:"reportedType,omitempty"`
	ReportedID   int         `json:"reportedId,omitempty"`
	Reason       string      `json:"reason,omitempty"`
	Status       string      `json:"status,omitempty"`
	AssignedTo   *ClientInfo `json:"assignedTo,omitempty"`
}

type ViewerEventPayload struct {
	ReportID int    `json:"reportId"`
	UserID   int    `json:"userId"`
	UserName string `json:"userName"`
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

type viewerPresence struct {
	userID    int
	userName  string
	itemID    int
	lastSeen  time.Time
	announced bool
}

type presenceConfig struct {
	channel        Channel
	joinedMessage  string
	leftMessage    string
	payloadBuilder func(itemID, userID int, userName string) interface{}
}

var reportPresenceConfig = presenceConfig{
	channel:       ChannelReports,
	joinedMessage: MsgReportViewerJoined,
	leftMessage:   MsgReportViewerLeft,
	payloadBuilder: func(itemID, userID int, userName string) interface{} {
		return ViewerEventPayload{ReportID: itemID, UserID: userID, UserName: userName}
	},
}

var queuePresenceConfig = presenceConfig{
	channel:       ChannelQueue,
	joinedMessage: MsgQueueViewerJoined,
	leftMessage:   MsgQueueViewerLeft,
	payloadBuilder: func(itemID, userID int, userName string) interface{} {
		return QueueViewerEventPayload{PostID: itemID, UserID: userID, UserName: userName}
	},
}

type TenantHub struct {
	clients       map[*Client]bool
	presence      map[int]*viewerPresence
	queuePresence map[int]*viewerPresence
	mu            sync.RWMutex
}

type Hub struct {
	tenants map[int]*TenantHub
	mu      sync.RWMutex
}

var defaultHub *Hub
var once sync.Once

func GetHub() *Hub {
	once.Do(func() {
		defaultHub = &Hub{
			tenants: make(map[int]*TenantHub),
		}
		go defaultHub.presenceSweep()
	})
	return defaultHub
}

func (h *Hub) getTenantHub(tenantID int) *TenantHub {
	h.mu.RLock()
	th := h.tenants[tenantID]
	h.mu.RUnlock()

	if th != nil {
		return th
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if th = h.tenants[tenantID]; th != nil {
		return th
	}

	th = &TenantHub{
		clients:       make(map[*Client]bool),
		presence:      make(map[int]*viewerPresence),
		queuePresence: make(map[int]*viewerPresence),
	}
	h.tenants[tenantID] = th
	return th
}

func (h *Hub) Register(client *Client) {
	th := h.getTenantHub(client.tenantID)
	th.mu.Lock()
	th.clients[client] = true
	th.mu.Unlock()
}

func (h *Hub) Unregister(client *Client) {
	th := h.getTenantHub(client.tenantID)
	th.mu.Lock()

	if _, ok := th.clients[client]; !ok {
		th.mu.Unlock()
		return
	}

	delete(th.clients, client)
	close(client.send)

	var pendingBroadcast *ViewerEventPayload
	if p, ok := th.presence[client.userID]; ok && p.announced {
		pendingBroadcast = &ViewerEventPayload{
			ReportID: p.itemID,
			UserID:   client.userID,
			UserName: p.userName,
		}
		delete(th.presence, client.userID)
	}

	isEmpty := len(th.clients) == 0
	th.mu.Unlock()

	if pendingBroadcast != nil {
		h.BroadcastToTenant(client.tenantID, MsgReportViewerLeft, *pendingBroadcast)
	}

	if isEmpty {
		h.mu.Lock()
		th.mu.RLock()
		if len(th.clients) == 0 {
			delete(h.tenants, client.tenantID)
		}
		th.mu.RUnlock()
		h.mu.Unlock()
	}
}

func getChannelForMessage(messageType string) Channel {
	switch messageType {
	case MsgReportNew, MsgReportAssigned, MsgReportUnassigned, MsgReportResolved, MsgReportViewerJoined, MsgReportViewerLeft:
		return ChannelReports
	case MsgQueuePostNew, MsgQueuePostTagged, MsgQueueViewerJoined, MsgQueueViewerLeft:
		return ChannelQueue
	default:
		return ""
	}
}

func (h *Hub) BroadcastToTenant(tenantID int, messageType string, payload interface{}) {
	h.mu.RLock()
	th := h.tenants[tenantID]
	h.mu.RUnlock()

	if th == nil {
		return
	}

	msg := &Message{Type: messageType, Payload: payload}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	targetChannel := getChannelForMessage(messageType)

	th.mu.RLock()
	defer th.mu.RUnlock()

	for client := range th.clients {
		if client.channel == targetChannel {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

func (h *Hub) updatePresence(tenantID, userID int, userName string, itemID int, presenceMap *map[int]*viewerPresence, config presenceConfig) {
	th := h.getTenantHub(tenantID)

	th.mu.Lock()
	defer th.mu.Unlock()

	existing := (*presenceMap)[userID]

	if existing != nil && existing.itemID != itemID && existing.announced {
		h.broadcastViewerEventLocked(th, config.leftMessage, config.channel, config.payloadBuilder(existing.itemID, userID, existing.userName))
	}

	if itemID == 0 {
		if existing != nil {
			delete(*presenceMap, userID)
		}
		return
	}

	isNew := existing == nil || existing.itemID != itemID || !existing.announced

	(*presenceMap)[userID] = &viewerPresence{
		userID:    userID,
		userName:  userName,
		itemID:    itemID,
		lastSeen:  time.Now(),
		announced: true,
	}

	if isNew {
		h.broadcastViewerEventLocked(th, config.joinedMessage, config.channel, config.payloadBuilder(itemID, userID, userName))
	}
}

func (h *Hub) broadcastViewerEventLocked(th *TenantHub, messageType string, channel Channel, payload interface{}) {
	msg := &Message{Type: messageType, Payload: payload}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for client := range th.clients {
		if client.channel == channel {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

func (h *Hub) UpdatePresence(tenantID, userID int, userName string, reportID int) {
	th := h.getTenantHub(tenantID)
	h.updatePresence(tenantID, userID, userName, reportID, &th.presence, reportPresenceConfig)
}

func (h *Hub) UpdateQueuePresence(tenantID, userID int, userName string, postID int) {
	th := h.getTenantHub(tenantID)
	h.updatePresence(tenantID, userID, userName, postID, &th.queuePresence, queuePresenceConfig)
}

type ReportViewers struct {
	ReportID int           `json:"reportId"`
	Viewers  []*ClientInfo `json:"viewers"`
}

func (h *Hub) GetAllActiveViewers(tenantID int) []ReportViewers {
	result := h.getViewers(tenantID, func(th *TenantHub) map[int]*viewerPresence { return th.presence },
		func(itemID int, viewers []*ClientInfo) interface{} {
			return ReportViewers{ReportID: itemID, Viewers: viewers}
		})
	if result == nil {
		return []ReportViewers{}
	}
	return result.([]ReportViewers)
}

type QueuePostViewers struct {
	PostID  int           `json:"postId"`
	Viewers []*ClientInfo `json:"viewers"`
}

func (h *Hub) GetAllQueueViewers(tenantID int) []QueuePostViewers {
	result := h.getViewers(tenantID, func(th *TenantHub) map[int]*viewerPresence { return th.queuePresence },
		func(itemID int, viewers []*ClientInfo) interface{} {
			return QueuePostViewers{PostID: itemID, Viewers: viewers}
		})
	if result == nil {
		return []QueuePostViewers{}
	}
	return result.([]QueuePostViewers)
}

func (h *Hub) getViewers(tenantID int, getPresenceMap func(*TenantHub) map[int]*viewerPresence, buildResult func(int, []*ClientInfo) interface{}) interface{} {
	h.mu.RLock()
	th := h.tenants[tenantID]
	h.mu.RUnlock()

	if th == nil {
		return nil
	}

	th.mu.RLock()
	defer th.mu.RUnlock()

	byItem := make(map[int][]*ClientInfo)
	presenceMap := getPresenceMap(th)
	for _, p := range presenceMap {
		if p.announced {
			byItem[p.itemID] = append(byItem[p.itemID], &ClientInfo{
				UserID:   p.userID,
				UserName: p.userName,
			})
		}
	}

	switch buildResult(0, nil).(type) {
	case ReportViewers:
		result := make([]ReportViewers, 0, len(byItem))
		for itemID, viewers := range byItem {
			result = append(result, buildResult(itemID, viewers).(ReportViewers))
		}
		return result
	case QueuePostViewers:
		result := make([]QueuePostViewers, 0, len(byItem))
		for itemID, viewers := range byItem {
			result = append(result, buildResult(itemID, viewers).(QueuePostViewers))
		}
		return result
	}
	return nil
}

func (h *Hub) presenceSweep() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.sweepStalePresence()
	}
}

func (h *Hub) sweepStalePresence() {
	threshold := time.Now().Add(-1 * time.Minute)

	h.mu.RLock()
	tenantIDs := make([]int, 0, len(h.tenants))
	for tid := range h.tenants {
		tenantIDs = append(tenantIDs, tid)
	}
	h.mu.RUnlock()

	for _, tenantID := range tenantIDs {
		h.mu.RLock()
		th := h.tenants[tenantID]
		h.mu.RUnlock()

		if th == nil {
			continue
		}

		th.mu.Lock()
		staleReports := h.collectStalePresence(th.presence, threshold)
		staleQueue := h.collectStalePresence(th.queuePresence, threshold)
		th.mu.Unlock()

		h.broadcastStaleViewerLeft(tenantID, staleReports, reportPresenceConfig)
		h.broadcastStaleViewerLeft(tenantID, staleQueue, queuePresenceConfig)
	}
}

func (h *Hub) collectStalePresence(presenceMap map[int]*viewerPresence, threshold time.Time) []*viewerPresence {
	var stale []*viewerPresence
	for userID, p := range presenceMap {
		if p.lastSeen.Before(threshold) {
			stale = append(stale, p)
			delete(presenceMap, userID)
		}
	}
	return stale
}

func (h *Hub) broadcastStaleViewerLeft(tenantID int, stale []*viewerPresence, config presenceConfig) {
	for _, p := range stale {
		if p.announced {
			h.BroadcastToTenant(tenantID, config.leftMessage, config.payloadBuilder(p.itemID, p.userID, p.userName))
		}
	}
}
