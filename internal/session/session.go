package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/display"
)

const (
	websocketWriteTimeout = 30 * time.Second
	websocketCloseTimeout = time.Second
	authReadTimeout       = 15 * time.Second

	// maxUsernameLen is the maximum accepted username length in per-user login
	// mode. Windows SAM account names are limited to 20 characters, but UPN and
	// domain\user formats can be longer. 256 characters is a safe upper bound.
	maxUsernameLen = 256
)

// RDPDialFunc is the signature used by Session to open an RDP connection.
// addr is "host:port"; domain may be empty for local accounts.
// Provide a custom implementation in tests to inject a fake RDPSession.
type RDPDialFunc func(ctx context.Context, addr, domain, username, password string, width, height int) (display.RDPSession, error)

// serverTile is the JSON message sent from server to browser for each tile update.
type serverTile struct {
	Type string `json:"type"` // always "tile"
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Data string `json:"data"` // base64-encoded JPEG bytes
}

// clientMsg is the JSON message received from the browser.
type clientMsg struct {
	Type     string `json:"type"`               // "keydown"|"keyup"|"mousemove"|"mousedown"|"mouseup"|"mousewheel"
	Scancode int    `json:"scancode,omitempty"` // PS/2 scancode for key events
	Button   int    `json:"button,omitempty"`   // 0=left,1=middle,2=right for mouse button events
	X        int    `json:"x,omitempty"`
	Y        int    `json:"y,omitempty"`
	Delta    int    `json:"delta,omitempty"` // scroll notches (positive=up, negative=down)
}

// authMsg is the first message sent by the browser in per-user login mode.
type authMsg struct {
	Type     string `json:"type"`     // always "auth"
	Username string `json:"username"`
	Password string `json:"password"`
}

// Session is a worker goroutine handling one WebSocket ↔ RDP tunnel.
type Session struct {
	ID string

	Conn *websocket.Conn

	// RDPAddr is the "host:port" of the Windows machine running RDP.
	RDPAddr string

	// StaticUsername and StaticPassword, when non-empty, are used directly as
	// RDP credentials without contacting the broker.
	StaticUsername string
	StaticPassword string

	// PerUserLogin, when true, activates per-user login mode: the session reads
	// the first WebSocket message as an auth message and uses those credentials
	// directly to dial RDP, bypassing the broker entirely. Passwordless Windows
	// accounts are supported by temporarily setting a random password via
	// broker.SetTempPassword.
	PerUserLogin bool

	CredRequests chan<- broker.CredRequest
	Events       chan<- broker.SessionEvent
	Shutdown     <-chan struct{}

	// RDPDial, when non-nil, overrides the default display.Connect call.
	// Used in tests to inject a fake RDPSession.
	RDPDial RDPDialFunc
}

func (s *Session) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionClosed}
		_ = s.Conn.Close()
	}()

	var cred broker.CredResponse
	if s.PerUserLogin {
		username, password, ok := s.readAuth(ctx)
		if !ok {
			return
		}
		// Passwordless Windows accounts cannot authenticate over RDP with an
		// empty password. Temporarily assign a random password for the duration
		// of the session, then restore the empty password on exit.
		if password == "" {
			tempPass, cleanup, err := broker.SetTempPassword(username)
			if err != nil {
				log.Printf("session %s: passwordless workaround failed for %q: %v", s.ID, username, err)
				s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: err}
				s.writeCloseMsg(websocket.CloseInternalServerErr, "authentication failed")
				return
			}
			defer cleanup()
			password = tempPass
		}
		cred = broker.CredResponse{SessionID: s.ID, Username: username, Password: password}
	} else {
		var ok bool
		cred, ok = s.requestCredentials(ctx)
		if !ok {
			s.writeCloseMsg(websocket.CloseGoingAway, "server shutting down")
			return
		}
		if cred.Err != nil {
			log.Printf("session %s: credential error: %v", s.ID, cred.Err)
			s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: cred.Err}
			s.writeCloseMsg(websocket.CloseInternalServerErr, "credential error")
			return
		}
	}

	rdp, err := s.dialRDP(ctx, cred)
	if err != nil {
		log.Printf("session %s: RDP connect error: %v", s.ID, err)
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: fmt.Errorf("rdp connect: %w", err)}
		s.writeCloseMsg(websocket.CloseInternalServerErr, "RDP connection failed")
		return
	}
	defer rdp.Close()

	inputDone := s.startInputReader(ctx, rdp)
	s.tileLoop(ctx, rdp, inputDone)
}

// readAuth reads the first WebSocket message and expects it to be an auth
// message. It returns the username and password extracted from the message.
// On failure it sends an appropriate close frame and returns ok=false.
func (s *Session) readAuth(ctx context.Context) (username, password string, ok bool) {
	_ = s.Conn.SetReadDeadline(time.Now().Add(authReadTimeout))
	_, raw, err := s.Conn.ReadMessage()
	_ = s.Conn.SetReadDeadline(time.Time{}) // clear deadline for subsequent reads
	if err != nil {
		select {
		case <-ctx.Done():
		case <-s.Shutdown:
		default:
			// 4008 is in the application-defined range (4000–4999) and signals
			// that the client failed to send an auth message within the timeout.
			s.writeCloseMsg(4008, "auth timeout")
		}
		return "", "", false
	}
	var msg authMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Type != "auth" || msg.Username == "" || len(msg.Username) > maxUsernameLen {
		s.writeCloseMsg(websocket.CloseUnsupportedData, "expected auth message")
		return "", "", false
	}
	return msg.Username, msg.Password, true
}

// writeCloseMsg sends a WebSocket close frame to the client.
func (s *Session) writeCloseMsg(code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	_ = s.Conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(websocketCloseTimeout))
}

// requestCredentials asks the broker for temporary Windows credentials, or
// returns the statically configured credentials if set.
func (s *Session) requestCredentials(ctx context.Context) (broker.CredResponse, bool) {
	if s.StaticUsername != "" {
		return broker.CredResponse{
			SessionID: s.ID,
			Username:  s.StaticUsername,
			Password:  s.StaticPassword,
		}, true
	}
	responseCh := make(chan broker.CredResponse, 1)
	select {
	case s.CredRequests <- broker.CredRequest{SessionID: s.ID, Reply: responseCh}:
	case <-ctx.Done():
		return broker.CredResponse{}, false
	case <-s.Shutdown:
		return broker.CredResponse{}, false
	}
	select {
	case <-ctx.Done():
		return broker.CredResponse{}, false
	case <-s.Shutdown:
		return broker.CredResponse{}, false
	case cred := <-responseCh:
		return cred, true
	}
}

// dialRDP opens an RDP session using either RDPDial (test hook) or display.Connect.
func (s *Session) dialRDP(ctx context.Context, cred broker.CredResponse) (display.RDPSession, error) {
	// Default resolution matches a common 16:9 layout.
	// The browser UI canvas backing store is set to the same dimensions so that
	// tile coordinates and mouse offsets are always consistent.
	const (
		defaultWidth  = 1280
		defaultHeight = 720
	)
	if s.RDPDial != nil {
		return s.RDPDial(ctx, s.RDPAddr, "", cred.Username, cred.Password, defaultWidth, defaultHeight)
	}
	return display.Connect(s.RDPAddr, "", cred.Username, cred.Password, defaultWidth, defaultHeight)
}

// startInputReader reads JSON input events from the WebSocket connection and
// forwards them to the RDP session. It returns a channel that is closed when
// the WebSocket read loop exits.
func (s *Session) startInputReader(ctx context.Context, rdp display.RDPSession) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, raw, err := s.Conn.ReadMessage()
			if err != nil {
				return
			}
			var msg clientMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-s.Shutdown:
				return
			default:
			}
			switch msg.Type {
			case "keydown":
				rdp.KeyDown(clampScancode(msg.Scancode))
			case "keyup":
				rdp.KeyUp(clampScancode(msg.Scancode))
			case "mousemove":
				rdp.MouseMove(msg.X, msg.Y)
			case "mousedown":
				rdp.MouseDown(clampButton(msg.Button), msg.X, msg.Y)
			case "mouseup":
				rdp.MouseUp(clampButton(msg.Button), msg.X, msg.Y)
			case "mousewheel":
				rdp.MouseWheel(clampWheelDelta(msg.Delta))
			}
		}
	}()
	return done
}

// clampScancode ensures a PS/2 scancode sent by the browser is within the
// valid 16-bit range [0x0000, 0xFFFF]. Values outside this range are zeroed
// to prevent unexpected behaviour in the underlying RDP library.
func clampScancode(sc int) int {
	if sc < 0 || sc > 0xFFFF {
		return 0
	}
	return sc
}

// clampButton constrains a mouse-button index to the three values recognised
// by the RDP protocol: 0 (left), 1 (middle), 2 (right). Any other value is
// mapped to 0.
func clampButton(b int) int {
	if b < 0 || b > 2 {
		return 0
	}
	return b
}

// clampWheelDelta constrains a scroll-wheel delta to [-1, 1] (one notch per
// event), matching the values the browser UI already sends. Extreme values
// from a malicious client are clamped rather than forwarded verbatim.
func clampWheelDelta(d int) int {
	switch {
	case d > 0:
		return 1
	case d < 0:
		return -1
	default:
		return 0
	}
}

// tileLoop reads tile updates from the RDP session and writes them as JSON to
// the WebSocket connection. It exits when the context is cancelled, the
// shutdown signal fires, the RDP tile channel closes, or the input reader exits.
func (s *Session) tileLoop(ctx context.Context, rdp display.RDPSession, inputDone <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.Shutdown:
			return
		case <-inputDone:
			return
		case tile, ok := <-rdp.Tiles():
			if !ok {
				return
			}
			msg := serverTile{
				Type: "tile",
				X:    tile.X,
				Y:    tile.Y,
				W:    tile.W,
				H:    tile.H,
				Data: base64.StdEncoding.EncodeToString(tile.Data),
			}
			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			_ = s.Conn.SetWriteDeadline(time.Now().Add(websocketWriteTimeout))
			if err := s.Conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				s.Events <- broker.SessionEvent{
					SessionID: s.ID,
					Type:      broker.SessionError,
					Err:       fmt.Errorf("websocket write failed: %w", err),
				}
				return
			}
		}
	}
}
