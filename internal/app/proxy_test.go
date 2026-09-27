package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Invalid JSON reaches validation only after the real write middleware accepts
// the origin. No business records are created by this test.
func TestWriteOriginBehindTLSProxy(t *testing.T) {
	handler := testStore(t).Handler(nil)
	cases := []struct {
		name, target, peer, origin string
		forwarded                  []string
		want                       int
	}{
		{"local HTTP", "http", "127.0.0.1:50000", "http://example.com", nil, 400},
		{"proxy HTTPS", "http", "127.0.0.1:50000", "https://example.com", []string{"https"}, 400},
		{"IPv6 proxy", "http", "[::1]:50000", "https://example.com", []string{"https"}, 400},
		{"direct TLS", "https", "192.0.2.1:50000", "https://example.com", nil, 400},
		{"TLS cannot be downgraded", "https", "127.0.0.1:50000", "https://example.com", []string{"http"}, 400},
		{"foreign origin", "http", "127.0.0.1:50000", "https://evil.example", []string{"https"}, 403},
		{"wrong scheme", "http", "127.0.0.1:50000", "http://example.com", []string{"https"}, 403},
		{"untrusted peer", "http", "192.0.2.1:50000", "https://example.com", []string{"https"}, 403},
		{"missing peer", "http", "", "https://example.com", []string{"https"}, 403},
		{"invalid peer", "http", "127.0.0.1", "https://example.com", []string{"https"}, 403},
		{"missing header", "http", "127.0.0.1:50000", "https://example.com", nil, 403},
		{"header list", "http", "127.0.0.1:50000", "https://example.com", []string{"https, http"}, 403},
		{"duplicate headers", "http", "127.0.0.1:50000", "https://example.com", []string{"https", "http"}, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.target+"://example.com/api/records", strings.NewReader("{"))
			req.RemoteAddr = tc.peer
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			for _, value := range tc.forwarded {
				req.Header.Add("X-Forwarded-Proto", value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}
