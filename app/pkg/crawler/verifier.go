package crawler

import (
	"context"
	"encoding/json"
	"net"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type ipRangeResponse struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix,omitempty"`
		IPv6Prefix string `json:"ipv6Prefix,omitempty"`
	} `json:"prefixes"`
}

var (
	DefaultVerifier *Verifier
	initOnce        sync.Once
)

type Verifier struct {
	sources []*Source
}

func Init() {
	initOnce.Do(func() {
		DefaultVerifier = &Verifier{sources: Sources}
	})
}

func (v *Verifier) IsVerified(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, src := range v.sources {
		if src.Contains(ip) {
			return true
		}
	}
	return false
}

func (v *Verifier) Refresh(ctx context.Context) error {
	var firstErr error

	for _, src := range v.sources {
		if err := v.refreshSource(ctx, src); err != nil {
			log.Errorf(ctx, "Failed to refresh crawler IPs for @{Source}: @{Error}", dto.Props{
				"Source": src.Name,
				"Error":  err.Error(),
			})
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (v *Verifier) refreshSource(ctx context.Context, src *Source) error {
	var allPrefixes []*net.IPNet

	for _, url := range src.URLs {
		prefixes, err := v.fetchPrefixes(ctx, url)
		if err != nil {
			return errors.Wrap(err, "failed to fetch %s", url)
		}
		allPrefixes = append(allPrefixes, prefixes...)
	}

	if len(allPrefixes) > 0 {
		src.SetPrefixes(allPrefixes)
		log.Infof(ctx, "Loaded @{Count} IP prefixes for @{Source}", dto.Props{
			"Count":  len(allPrefixes),
			"Source": src.Name,
		})
	}

	return nil
}

func (v *Verifier) fetchPrefixes(ctx context.Context, url string) ([]*net.IPNet, error) {
	req := &cmd.HTTPRequest{
		URL:    url,
		Method: "GET",
	}

	if err := bus.Dispatch(ctx, req); err != nil {
		return nil, errors.Wrap(err, "http request failed")
	}

	if req.ResponseStatusCode != 200 {
		return nil, errors.New("unexpected status code: %d", req.ResponseStatusCode)
	}

	var resp ipRangeResponse
	if err := json.Unmarshal(req.ResponseBody, &resp); err != nil {
		return nil, errors.Wrap(err, "failed to parse json")
	}

	prefixes := make([]*net.IPNet, 0, len(resp.Prefixes))
	for _, p := range resp.Prefixes {
		cidr := p.IPv4Prefix
		if cidr == "" {
			cidr = p.IPv6Prefix
		}
		if cidr == "" {
			continue
		}

		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		prefixes = append(prefixes, ipnet)
	}

	return prefixes, nil
}
