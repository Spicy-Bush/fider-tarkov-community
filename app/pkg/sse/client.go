package sse

type Client struct {
	send     chan []byte
	tenantID int
	userID   int
	userName string
}

func NewClient(tenantID, userID int, userName string) *Client {
	return &Client{
		send:     make(chan []byte, 32),
		tenantID: tenantID,
		userID:   userID,
		userName: userName,
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
