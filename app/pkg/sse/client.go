package sse

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

type Channel string

const (
	ChannelReports Channel = "reports"
	ChannelQueue   Channel = "queue"
)

type Client struct {
	id       string
	send     chan []byte
	tenantID int
	userID   int
	userName string
	channel  Channel
	itemID   int
	lastSeen time.Time
}

func NewClient(tenantID, userID int, userName string, channel Channel) *Client {
	return &Client{
		id:       rand.String(32),
		send:     make(chan []byte, 32),
		tenantID: tenantID,
		userID:   userID,
		userName: userName,
		channel:  channel,
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) Send() <-chan []byte {
	return c.send
}

func (c *Client) TenantID() int {
	return c.tenantID
}

func (c *Client) UserID() int {
	return c.userID
}

func (c *Client) UserName() string {
	return c.userName
}

func (c *Client) Channel() Channel {
	return c.channel
}
