package web

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	ui "github.com/pacorreia/go-rdp-server/ui"
)

const (
	// httpReadHeaderTimeout is the maximum time allowed to read request headers.
	// This protects against Slowloris-style attacks where a client sends headers
	// very slowly to hold a connection indefinitely.
	httpReadHeaderTimeout = 10 * time.Second

	// httpIdleTimeout is the maximum time an idle keep-alive connection is kept
	// open. After the WebSocket handshake, the connection is hijacked by gorilla
	// and managed independently, so this only affects plain HTTP connections.
	httpIdleTimeout = 120 * time.Second
)

type Server struct {
	httpServer *http.Server
}

func NewServer(addr string, handlers *Handlers) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/config", handlers.HandleConfig)
	mux.HandleFunc("/ws/rdp", handlers.HandleRDPWebSocket)

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           securityHeaders(mux),
			ReadHeaderTimeout: httpReadHeaderTimeout,
			IdleTimeout:       httpIdleTimeout,
		},
	}
}

// securityHeaders wraps h with a middleware that sets defensive HTTP response
// headers on every response:
//   - X-Frame-Options: DENY prevents the UI from being embedded in an iframe
//     (clickjacking protection).
//   - X-Content-Type-Options: nosniff prevents browsers from MIME-sniffing
//     response bodies away from the declared Content-Type.
//   - Content-Security-Policy: restricts what resources the page may load and
//     explicitly forbids framing via frame-ancestors 'none'.
//   - Referrer-Policy: limits referrer information sent to other origins.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// The UI relies on inline <script> and <style> tags (hence 'unsafe-inline'),
		// blob: URLs for JPEG tile rendering, and ws:/wss: for the WebSocket tunnel.
		h.Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' blob:; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	content, err := fs.ReadFile(ui.Assets, "index.html")
	if err != nil {
		http.Error(w, "unable to load ui", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
