package web

import (
	"context"
	"errors"
	"net/http"
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
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handlers) HandleRDPWebSocket(w http.ResponseWriter, r *http.Request) {
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
