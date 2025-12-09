package sse

// main logic for all SSE events which use a fire and
// forget approach to broadcast to all clients in a tenant

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
)

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type ClientInfo struct {
	UserID   int    `json:"userId"`
	UserName string `json:"userName"`
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

type viewerPresence struct {
	userID    int
	userName  string
	reportID  int
	lastSeen  time.Time
	announced bool
}

type TenantHub struct {
	clients  map[*Client]bool
	presence map[int]*viewerPresence
	mu       sync.RWMutex
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
		clients:  make(map[*Client]bool),
		presence: make(map[int]*viewerPresence),
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
			ReportID: p.reportID,
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

// sends a message to all clients in the tenant
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

	th.mu.RLock()
	defer th.mu.RUnlock()

	for client := range th.clients {
		select {
		case client.send <- data:
		default:
			// Channel full, will skip here
		}
	}
}

func (h *Hub) UpdatePresence(tenantID, userID int, userName string, reportID int) {
	th := h.getTenantHub(tenantID)

	th.mu.Lock()
	defer th.mu.Unlock()

	existing := th.presence[userID]

	if existing != nil && existing.reportID != reportID && existing.announced {
		h.broadcastViewerLeftLocked(th, existing.reportID, userID, existing.userName)
	}

	if reportID == 0 {
		if existing != nil {
			delete(th.presence, userID)
		}
		return
	}

	isNew := existing == nil || existing.reportID != reportID || !existing.announced

	th.presence[userID] = &viewerPresence{
		userID:    userID,
		userName:  userName,
		reportID:  reportID,
		lastSeen:  time.Now(),
		announced: true,
	}

	if isNew {
		h.broadcastViewerJoinedLocked(th, reportID, userID, userName)
	}
}

func (h *Hub) broadcastViewerJoinedLocked(th *TenantHub, reportID, userID int, userName string) {
	msg := &Message{
		Type: MsgReportViewerJoined,
		Payload: ViewerEventPayload{
			ReportID: reportID,
			UserID:   userID,
			UserName: userName,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for client := range th.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

func (h *Hub) broadcastViewerLeftLocked(th *TenantHub, reportID, userID int, userName string) {
	msg := &Message{
		Type: MsgReportViewerLeft,
		Payload: ViewerEventPayload{
			ReportID: reportID,
			UserID:   userID,
			UserName: userName,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for client := range th.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

type ReportViewers struct {
	ReportID int           `json:"reportId"`
	Viewers  []*ClientInfo `json:"viewers"`
}

func (h *Hub) GetAllActiveViewers(tenantID int) []ReportViewers {
	h.mu.RLock()
	th := h.tenants[tenantID]
	h.mu.RUnlock()

	if th == nil {
		return nil
	}

	th.mu.RLock()
	defer th.mu.RUnlock()

	byReport := make(map[int][]*ClientInfo)
	for _, p := range th.presence {
		if p.announced {
			byReport[p.reportID] = append(byReport[p.reportID], &ClientInfo{
				UserID:   p.userID,
				UserName: p.userName,
			})
		}
	}

	result := make([]ReportViewers, 0, len(byReport))
	for reportID, viewers := range byReport {
		result = append(result, ReportViewers{
			ReportID: reportID,
			Viewers:  viewers,
		})
	}
	return result
}

func (h *Hub) presenceSweep() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.sweepStalePresence()
	}
}

func (h *Hub) sweepStalePresence() {
	threshold := time.Now().Add(-45 * time.Second)

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
		var stale []*viewerPresence
		for userID, p := range th.presence {
			if p.lastSeen.Before(threshold) {
				stale = append(stale, p)
				delete(th.presence, userID)
			}
		}
		th.mu.Unlock()

		for _, p := range stale {
			if p.announced {
				h.BroadcastToTenant(tenantID, MsgReportViewerLeft, ViewerEventPayload{
					ReportID: p.reportID,
					UserID:   p.userID,
					UserName: p.userName,
				})
			}
		}
	}
}
