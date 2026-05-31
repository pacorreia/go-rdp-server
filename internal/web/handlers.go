package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/session"
)

// By default, the server limits concurrent WebSocket connections by source IP
// accepted from a single remote IP address.  This limits slot-holding DoS
// attacks: even if an attacker holds connections open for the full
// authReadTimeout, they can only occupy a handful of slots rather than
// draining the entire session pool.

// ipTracker tracks the number of concurrent WebSocket connections per remote
// IP.  It is embedded by value in Handlers so the zero value is ready to use.
type ipTracker struct {
	mu    sync.Mutex
	conns map[string]int
}

// acquire increments the connection count for ip. It returns false if the
// count would exceed maxConnsPerIP. A maxConnsPerIP of 0 disables the limit.
func (t *ipTracker) acquire(ip string, maxConnsPerIP int) bool {
	if maxConnsPerIP <= 0 {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conns == nil {
		t.conns = make(map[string]int)
	}
	if t.conns[ip] >= maxConnsPerIP {
		return false
	}
	t.conns[ip]++
	return true
}

// release decrements the connection count for ip.
func (t *ipTracker) release(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conns == nil {
		return
	}
	if t.conns[ip] <= 1 {
		delete(t.conns, ip)
	} else {
		t.conns[ip]--
	}
}

type Handlers struct {
	Manager      *session.Manager
	CredRequests chan<- broker.CredRequest
	SessionEvent chan<- broker.SessionEvent
	Shutdown     <-chan struct{}
	Ctx          context.Context

	RDPAddr string

	// StaticRDPUsername and StaticRDPPassword, when non-empty, are used directly
	// as RDP credentials for every session, bypassing the Windows temp-account
	// broker. Set via --rdp-user / --rdp-pass flags or RDP_USER / RDP_PASS env vars.
	StaticRDPUsername string
	StaticRDPPassword string

	// PerUserLogin, when true, activates per-user login mode: the browser shows
	// a login form and the first WebSocket message must be an auth message
	// containing the user's credentials. The broker is not used.
	PerUserLogin bool

	// AllowPasswordless is retained for backwards compatibility with previous
	// config surfaces. Empty passwords are rejected in per-user login mode.
	AllowPasswordless bool

	// RDPDial, when non-nil, is forwarded to each session worker as a test hook.
	RDPDial session.RDPDialFunc

	// MaxConnsPerIP limits concurrent connections per source IP. Set to 0 to disable.
	MaxConnsPerIP int

	// tracker limits concurrent connections per remote IP.
	tracker ipTracker
}

// clientConfig is the JSON payload returned by GET /api/config.
type clientConfig struct {
	PerUserLogin bool `json:"perUserLogin"`
}

const websocketCloseMessageTimeout = time.Second

// maxWebSocketMessageSize is the maximum size (in bytes) accepted for a single
// WebSocket message from a browser client.  Input events are tiny JSON objects;
// 4 KiB is generous and prevents a malicious client from sending very large
// messages to exhaust server memory.
const maxWebSocketMessageSize = 4096

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

// HandleConfig returns a small JSON object that the browser reads on startup
// to decide how to behave (e.g. whether to show the per-user login form).
func (h *Handlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clientConfig{PerUserLogin: h.PerUserLogin && h.StaticRDPUsername == ""})
}

func (h *Handlers) HandleRDPWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce per-IP connection limit before upgrading to WebSocket, so that
	// we can still return a plain HTTP error response.
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "bad remote address", http.StatusBadRequest)
		return
	}
	maxConnsPerIP := h.MaxConnsPerIP
	if maxConnsPerIP < 0 {
		maxConnsPerIP = 0
	}
	if !h.tracker.acquire(remoteIP, maxConnsPerIP) {
		http.Error(w, "too many connections from your IP", http.StatusTooManyRequests)
		return
	}

	conn, upgradeErr := upgrader.Upgrade(w, r, nil)
	if upgradeErr != nil {
		h.tracker.release(remoteIP)
		return
	}
	// Cap incoming message size to prevent a malicious client from sending an
	// arbitrarily large payload that would exhaust server memory.
	conn.SetReadLimit(maxWebSocketMessageSize)

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
		h.tracker.release(remoteIP)
		return
	}

	ctx := h.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	h.SessionEvent <- broker.SessionEvent{SessionID: sessionID, Type: broker.SessionOpened}
	worker := &session.Session{
		ID:                sessionID,
		Conn:              conn,
		RDPAddr:           h.RDPAddr,
		StaticUsername:    h.StaticRDPUsername,
		StaticPassword:    h.StaticRDPPassword,
		PerUserLogin:      h.PerUserLogin,
		AllowPasswordless: h.AllowPasswordless,
		CredRequests:      h.CredRequests,
		Events:            h.SessionEvent,
		Shutdown:          h.Shutdown,
		RDPDial:           h.RDPDial,
	}

	go func() {
		defer h.tracker.release(remoteIP)
		worker.Run(ctx)
	}()
}
