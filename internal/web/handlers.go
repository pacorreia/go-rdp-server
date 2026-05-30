package web

import (
	"context"
	"errors"
	"log"
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
	Ctx          context.Context

	GuacdAddr string
	RDPHost   string
	RDPPort   string
}

const websocketCloseMessageTimeout = time.Second

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     isAllowedOrigin,
}

func isAllowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
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
		closeCode := websocket.CloseTryAgainLater
		closeMsg := "server at capacity; please retry shortly"
		if !errors.Is(err, session.ErrMaxSessionsReached) {
			log.Printf("session admission error: %v", err)
			closeCode = websocket.CloseInternalServerErr
			closeMsg = "internal server error"
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, closeMsg), time.Now().Add(websocketCloseMessageTimeout))
		_ = conn.Close()
		return
	}

	ctx := h.Ctx
	if ctx == nil {
		ctx = context.Background()
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

	go worker.Run(ctx)
}
