package app

import (
	"net"
	"net/http"
)

// requestScheme trusts the protocol header only from the local reverse proxy.
// Nginx must overwrite X-Forwarded-Proto, and the app must listen on loopback.
func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && net.ParseIP(host).IsLoopback() {
		values := r.Header.Values("X-Forwarded-Proto")
		if len(values) == 1 && values[0] == "https" {
			return "https"
		}
	}
	return "http"
}
