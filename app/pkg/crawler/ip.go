package crawler

import (
	"net"
	"net/http"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func GetRealIP(r *http.Request) net.IP {
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return ip
		}
	}

	if env.IsDevelopment() {
		if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
			if ip := net.ParseIP(xRealIP); ip != nil {
				return ip
			}
		}
	}

	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	return net.ParseIP(host)
}

