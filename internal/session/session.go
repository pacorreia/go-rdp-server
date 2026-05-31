package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/display"
)

const websocketWriteTimeout = 30 * time.Second

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

// Session is a worker goroutine handling one WebSocket ↔ RDP tunnel.
type Session struct {
	ID string

	Conn *websocket.Conn

	// RDPAddr is the "host:port" of the Windows machine running RDP.
	RDPAddr string

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

	cred, ok := s.requestCredentials(ctx)
	if !ok {
		return
	}
	if cred.Err != nil {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: cred.Err}
		return
	}

	rdp, err := s.dialRDP(ctx, cred)
	if err != nil {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: fmt.Errorf("rdp connect: %w", err)}
		return
	}
	defer rdp.Close()

	inputDone := s.startInputReader(ctx, rdp)
	s.tileLoop(ctx, rdp, inputDone)
}

// requestCredentials asks the broker for temporary Windows credentials.
func (s *Session) requestCredentials(ctx context.Context) (broker.CredResponse, bool) {
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
				rdp.KeyDown(msg.Scancode)
			case "keyup":
				rdp.KeyUp(msg.Scancode)
			case "mousemove":
				rdp.MouseMove(msg.X, msg.Y)
			case "mousedown":
				rdp.MouseDown(msg.Button, msg.X, msg.Y)
			case "mouseup":
				rdp.MouseUp(msg.Button, msg.X, msg.Y)
			case "mousewheel":
				rdp.MouseWheel(msg.Delta)
			}
		}
	}()
	return done
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
