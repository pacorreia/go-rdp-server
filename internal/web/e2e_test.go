package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/display"
	"github.com/pacorreia/go-rdp-server/internal/session"
)

// fakeAccounts satisfies broker.Accounts without hitting the OS.
type fakeAccounts struct{}

func (fakeAccounts) CreateTempUser(username, password string) error { return nil }
func (fakeAccounts) DeleteTempUser(username string) error           { return nil }
func (fakeAccounts) AddToRDPGroup(username string) error            { return nil }

// fakeRDPSession is a mock display.RDPSession that sends a single tile then idles.
type fakeRDPSession struct {
	tiles chan display.Tile
}

func newFakeRDPSession(t *testing.T) *fakeRDPSession {
	t.Helper()
	// Build a tiny 4×4 red JPEG to send as a tile.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("fakeRDPSession: jpeg encode: %v", err)
	}

	s := &fakeRDPSession{tiles: make(chan display.Tile, 1)}
	s.tiles <- display.Tile{X: 0, Y: 0, W: 4, H: 4, Data: buf.Bytes()}
	return s
}

func (f *fakeRDPSession) Tiles() <-chan display.Tile      { return f.tiles }
func (f *fakeRDPSession) KeyDown(_ int)                  {}
func (f *fakeRDPSession) KeyUp(_ int)                    {}
func (f *fakeRDPSession) MouseMove(_, _ int)             {}
func (f *fakeRDPSession) MouseDown(_, _, _ int)          {}
func (f *fakeRDPSession) MouseUp(_, _, _ int)            {}
func (f *fakeRDPSession) MouseWheel(_, _, _ int)         {}
func (f *fakeRDPSession) Close()                         {}

func TestE2EWebsocketFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	credRequests := make(chan broker.CredRequest, 8)
	sessionEvents := make(chan broker.SessionEvent, 16)
	shutdown := make(chan struct{})

	brokerWorker := &broker.Broker{
		Requests: requestsReadOnly(credRequests),
		Events:   sessionEvents,
		Shutdown: shutdown,
		Accounts: fakeAccounts{},
		PasswordGenerator: func() (string, error) {
			return "test-password", nil
		},
	}
	manager := session.NewManager(2, sessionEvents, shutdown)

	fakeRDP := newFakeRDPSession(t)
	handlers := &Handlers{
		Manager:      manager,
		CredRequests: credRequests,
		SessionEvent: sessionEvents,
		Shutdown:     shutdown,
		Ctx:          ctx,
		RDPAddr:      "127.0.0.1:3389",
		// Inject the fake RDP session so no real RDP connection is needed.
		RDPDial: func(_ context.Context, _, _, _, _ string, _, _ int) (display.RDPSession, error) {
			return fakeRDP, nil
		},
	}

	serverAddr := freeLocalAddress(t)
	srv := NewServer(serverAddr, handlers)

	go brokerWorker.Run(ctx)
	go manager.Run(ctx)
	go func() { _ = srv.Start() }()
	t.Cleanup(func() {
		close(shutdown)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	})

	waitForHTTP(t, "http://"+serverAddr+"/")

	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://"+serverAddr)
	conn, _, err := dialer.Dial("ws://"+serverAddr+"/ws/rdp", header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	// The fake RDP session immediately emits one tile; we expect to receive it
	// as a JSON tile message on the WebSocket.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message failed: %v", err)
	}

	var msg struct {
		Type string `json:"type"`
		X    int    `json:"x"`
		Y    int    `json:"y"`
		W    int    `json:"w"`
		H    int    `json:"h"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal tile message: %v", err)
	}
	if msg.Type != "tile" {
		t.Fatalf("expected type=tile, got %q", msg.Type)
	}
	if msg.W != 4 || msg.H != 4 {
		t.Fatalf("expected 4×4 tile, got %d×%d", msg.W, msg.H)
	}
	if _, err := base64.StdEncoding.DecodeString(msg.Data); err != nil {
		t.Fatalf("tile data is not valid base64: %v", err)
	}
}

func TestE2ENativeRDPClient(t *testing.T) {
	if os.Getenv("ENABLE_NATIVE_RDP_E2E_TEST") != "1" {
		t.Skip("set ENABLE_NATIVE_RDP_E2E_TEST=1 to run native RDP client check")
	}
	rdpHost := os.Getenv("RDP_HOST")
	rdpPort := os.Getenv("RDP_PORT")
	if rdpHost == "" || rdpPort == "" {
		t.Skip("RDP_HOST and RDP_PORT are required for native RDP check")
	}
	if _, err := exec.LookPath("xfreerdp"); err != nil {
		t.Skip("xfreerdp not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xfreerdp", "/v:"+rdpHost+":"+rdpPort, "/u:invalid", "/p:invalid", "/cert:ignore", "/sec:rdp", "+auth-only")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("native RDP client timed out: %s", output)
	}
	if err == nil {
		return
	}
	if strings.Contains(strings.ToLower(string(output)), "authentication only, exit status 0") {
		return
	}
	t.Fatalf("native RDP client failed: %v (%s)", err, output)
}

func waitForHTTP(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %s", address)
		}
		resp, err := http.Get(address)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate local port: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func requestsReadOnly(ch chan broker.CredRequest) <-chan broker.CredRequest {
	return ch
}
