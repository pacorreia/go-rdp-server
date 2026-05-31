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
		{name: "no origin", host: "localhost:8080", origin: "", allow: false},
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

func TestIPTrackerAcquireRespectsLimit(t *testing.T) {
	var tracker ipTracker
	ip := "127.0.0.1"
	if !tracker.acquire(ip, 2) {
		t.Fatalf("first acquire should succeed")
	}
	if !tracker.acquire(ip, 2) {
		t.Fatalf("second acquire should succeed")
	}
	if tracker.acquire(ip, 2) {
		t.Fatalf("third acquire should fail when limit is reached")
	}
	tracker.release(ip)
	if !tracker.acquire(ip, 2) {
		t.Fatalf("acquire should succeed after release")
	}
}

func TestIPTrackerAcquireDisabledLimit(t *testing.T) {
	var tracker ipTracker
	ip := "127.0.0.1"
	for i := 0; i < 100; i++ {
		if !tracker.acquire(ip, 0) {
			t.Fatalf("acquire should succeed when limit is disabled")
		}
	}
	tracker.release(ip)
}
