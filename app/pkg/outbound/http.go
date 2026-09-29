package outbound

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

var errPrivateDestination = errors.New("outbound HTTP requires a public network destination")

var nonPublicRanges = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

var publicIPv6 = netip.MustParsePrefix("2000::/3")

func publicConnection(ctx context.Context, network, address string, connection syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.Zone() != "" || (ip.Is6() && !publicIPv6.Contains(ip)) {
		return errPrivateDestination
	}
	for _, prefix := range nonPublicRanges {
		if prefix.Contains(ip) {
			return errPrivateDestination
		}
	}

	return nil
}

func NewClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		// DNS answers can change before the connection is opened.
		ControlContext: publicConnection,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	// A proxy would resolve the destination outside this boundary.
	transport.Proxy = nil

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}
