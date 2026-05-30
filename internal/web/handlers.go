package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/session"
)

type Handlers struct {
	Manager      *session.Manager
	CredRequests chan<- broker.CredRequest
	SessionEvent chan<- broker.SessionEvent
	Shutdown     <-chan struct{}

	GuacdAddr string
	RDPHost   string
	RDPPort   string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     isAllowedOrigin,
}

func isAllowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(originURL.Host, r.Host)
}

func (h *Handlers) HandleRDPWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	sessionID := uuid.NewString()
	if err := h.Manager.Admit(sessionID); err != nil {
		if errors.Is(err, session.ErrMaxSessionsReached) {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "max sessions reached"), time.Now().Add(time.Second))
		}
		_ = conn.Close()
		return
	}

	h.SessionEvent <- broker.SessionEvent{SessionID: sessionID, Type: broker.SessionOpened}
	worker := &session.Session{
		ID:           sessionID,
		Conn:         conn,
		GuacdAddr:    h.GuacdAddr,
		RDPHost:      h.RDPHost,
		RDPPort:      h.RDPPort,
		CredRequests: h.CredRequests,
		Events:       h.SessionEvent,
		Shutdown:     h.Shutdown,
	}

	go worker.Run(context.Background())
}
