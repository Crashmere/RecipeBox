package app

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	Start time.Time
	Count int
}
type limiter struct {
	mu      sync.Mutex
	clients map[string]rateWindow
}

func (l *limiter) allow(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := r.Header.Get("X-Real-IP"); net.ParseIP(real) != nil {
			host = real
		}
	}
	limit := 120
	if strings.Contains(r.URL.Path, "/uploads") {
		host += "/uploads"
		limit = 30
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clients == nil {
		l.clients = map[string]rateWindow{}
	}
	now := time.Now()
	if len(l.clients) > 10000 {
		for k, v := range l.clients {
			if now.Sub(v.Start) > time.Minute {
				delete(l.clients, k)
			}
		}
		if len(l.clients) > 10000 {
			return false
		}
	}
	v := l.clients[host]
	if now.Sub(v.Start) > time.Minute {
		v = rateWindow{Start: now}
	}
	v.Count++
	l.clients[host] = v
	return v.Count <= limit
}
