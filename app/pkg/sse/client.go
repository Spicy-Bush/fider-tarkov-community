package sse

type Channel string

const (
	ChannelReports Channel = "reports"
	ChannelQueue   Channel = "queue"
)

type Client struct {
	send     chan []byte
	tenantID int
	userID   int
	userName string
	channel  Channel
}

func NewClient(tenantID, userID int, userName string, channel Channel) *Client {
	return &Client{
		send:     make(chan []byte, 32),
		tenantID: tenantID,
		userID:   userID,
		userName: userName,
		channel:  channel,
	}
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
