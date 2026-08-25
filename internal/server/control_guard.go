package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

func allowControlRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return false
	}
	requestHost := r.Host
	if parsed, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		requestHost = parsed
	}
	requestIP := net.ParseIP(strings.Trim(requestHost, "[]"))
	if !strings.EqualFold(requestHost, "localhost") && (requestIP == nil || !requestIP.IsLoopback()) {
		return false
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true // CLI/curl cục bộ không gửi Origin.
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}
