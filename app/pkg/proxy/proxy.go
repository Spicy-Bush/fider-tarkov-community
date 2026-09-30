package proxy

import (
	"net/http"
	"net/netip"
	"strings"
)

func TrustedPeer(request *http.Request, networks []string) bool {
	peer, err := netip.ParseAddrPort(request.RemoteAddr)
	if err != nil {
		return false
	}

	for _, network := range networks {
		prefix, err := netip.ParsePrefix(network)
		if err == nil && prefix.Contains(peer.Addr().Unmap()) {
			return true
		}
	}

	return false
}

// Trusted proxies must replace these headers, not append client-supplied values.
func Header(request *http.Request, name string, networks []string) string {
	if !TrustedPeer(request, networks) {
		return ""
	}

	values := request.Header.Values(name)
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return ""
	}

	return strings.TrimSpace(values[0])
}
