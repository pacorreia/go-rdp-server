package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		allow  bool
	}{
		{name: "no origin", host: "localhost:8080", origin: "", allow: true},
		{name: "same host", host: "localhost:8080", origin: "http://localhost:8080", allow: true},
		{name: "different host", host: "localhost:8080", origin: "http://evil.local", allow: false},
		{name: "invalid origin", host: "localhost:8080", origin: "::::", allow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/ws/rdp", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if got := isAllowedOrigin(req); got != tt.allow {
				t.Fatalf("isAllowedOrigin()=%v, want %v", got, tt.allow)
			}
		})
	}
}
