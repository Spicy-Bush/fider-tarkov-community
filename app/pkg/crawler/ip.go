package crawler

import (
	"net"
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/proxy"
)

func GetRealIP(r *http.Request) net.IP {
	if cfIP := proxy.Header(r, "CF-Connecting-IP", env.Config.TrustedProxies); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}

	return net.ParseIP(host)
}
