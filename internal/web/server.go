package web

import (
	"context"
	"io/fs"
	"net/http"

	ui "github.com/pacorreia/go-rdp-server/ui"
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
			Addr:    addr,
			Handler: mux,
		},
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
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
