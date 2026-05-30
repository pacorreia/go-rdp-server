package web

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/session"
)

type fakeAccounts struct{}

func (fakeAccounts) CreateTempUser(username, password string) error { return nil }
func (fakeAccounts) DeleteTempUser(username string) error           { return nil }
func (fakeAccounts) AddToRDPGroup(username string) error            { return nil }

func TestE2EWebsocketFlowWithGuacd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	guacdAddr, stopFakeGuacd := startFakeGuacd(t)
	defer stopFakeGuacd()

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
	handlers := &Handlers{
		Manager:      manager,
		CredRequests: credRequests,
		SessionEvent: sessionEvents,
		Shutdown:     shutdown,
		GuacdAddr:    guacdAddr,
		RDPHost:      "127.0.0.1",
		RDPPort:      "3389",
	}

	serverAddr := freeLocalAddress(t)
	srv := NewServer(serverAddr, handlers)

	go brokerWorker.Run(ctx)
	go manager.Run(ctx)
	go func() {
		_ = srv.Start()
	}()
	t.Cleanup(func() {
		close(shutdown)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	})

	waitForHTTP(t, "http://"+serverAddr+"/")

	u := "ws://" + serverAddr + "/ws/rdp"
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://"+serverAddr)
	conn, _, err := dialer.Dial(u, header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("4.sync,1.2;")); err != nil {
		t.Fatalf("write websocket message failed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message failed: %v", err)
	}
	if !strings.Contains(string(payload), "sync") {
		t.Fatalf("expected sync instruction from guacd, got %q", string(payload))
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

func startFakeGuacd(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake guacd listener: %v", err)
	}

	stop := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go handleFakeGuacdConn(conn)
		}
	}()

	return listener.Addr().String(), func() {
		close(stop)
		_ = listener.Close()
	}
}

func handleFakeGuacdConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString(';')
		if err != nil {
			return
		}
		if strings.Contains(line, "connect") {
			_, _ = conn.Write([]byte("4.sync,1.1;"))
			return
		}
	}
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
