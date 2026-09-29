package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/outbound"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
)

const maxResponseBytes = 8 * 1024 * 1024

var client = outbound.NewClient()

func init() {
	bus.Register(Service{})
}

type Service struct{}

func (s Service) Name() string {
	return "HTTP"
}

func (s Service) Category() string {
	return "httpclient"
}

func (s Service) Enabled() bool {
	return !env.IsTest()
}

func (s Service) Init() {
	bus.AddHandler(requestHandler)
}

func requestHandler(ctx context.Context, c *cmd.HTTPRequest) error {
	req, err := http.NewRequest(c.Method, c.URL, c.Body)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.BasicAuth != nil {
		req.SetBasicAuth(c.BasicAuth.User, c.BasicAuth.Password)
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	respBody, err := readlimit.ReadAll(res.Body, maxResponseBytes)
	if err != nil {
		return fmt.Errorf("read HTTP response (limit %d bytes): %w", maxResponseBytes, err)
	}

	c.ResponseBody = respBody
	c.ResponseStatusCode = res.StatusCode
	c.ResponseHeader = res.Header
	return nil
}
