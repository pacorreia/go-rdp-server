package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/guacd"
)

const websocketWriteTimeout = 30 * time.Second

type guacdClient interface {
	SendChan() chan<- *guacd.Instruction
	RecvChan() <-chan *guacd.Instruction
}

type defaultGuacdClient struct {
	client *guacd.Client
}

func (c defaultGuacdClient) SendChan() chan<- *guacd.Instruction { return c.client.Send }
func (c defaultGuacdClient) RecvChan() <-chan *guacd.Instruction { return c.client.Recv }

// GuacdDial is the signature used by tests to inject a fake guacd client.
// addr is the guacd TCP address; host/port/username/password are the RDP
// connection parameters that the real implementation passes to NewClient.
type GuacdDialFunc func(ctx context.Context, addr, host, port, username, password string) (guacdClient, error)

// Session is a worker goroutine handling one websocket ↔ guacd tunnel.
type Session struct {
	ID string

	Conn *websocket.Conn

	GuacdAddr string
	RDPHost   string
	RDPPort   string

	CredRequests chan<- broker.CredRequest
	Events       chan<- broker.SessionEvent
	Shutdown     <-chan struct{}

	GuacdDial GuacdDialFunc
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

	client, err := s.newGuacdClient(ctx, cred)
	if err != nil {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: err}
		return
	}

	wsToGuacd := s.startWebSocketReader(ctx)
	guacdToWS := s.startGuacdReader(ctx, client.RecvChan())
	s.proxyLoop(ctx, client.SendChan(), wsToGuacd, guacdToWS)
}

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

func (s *Session) newGuacdClient(ctx context.Context, cred broker.CredResponse) (guacdClient, error) {
	if s.GuacdDial != nil {
		return s.GuacdDial(ctx, s.GuacdAddr, s.RDPHost, s.RDPPort, cred.Username, cred.Password)
	}
	client, err := guacd.NewClient(ctx, s.GuacdAddr, s.RDPHost, s.RDPPort, cred.Username, cred.Password)
	if err != nil {
		return nil, err
	}
	return defaultGuacdClient{client: client}, nil
}

func (s *Session) startWebSocketReader(ctx context.Context) <-chan *guacd.Instruction {
	out := make(chan *guacd.Instruction, 64)
	go func() {
		defer close(out)
		for {
			_, message, err := s.Conn.ReadMessage()
			if err != nil {
				return
			}
			instructions, err := guacd.DecodeAll(strings.TrimSpace(string(message)))
			if err != nil {
				continue
			}
			for _, instruction := range instructions {
				select {
				case <-ctx.Done():
					return
				case <-s.Shutdown:
					return
				case out <- instruction:
				}
			}
		}
	}()
	return out
}

func (s *Session) startGuacdReader(ctx context.Context, input <-chan *guacd.Instruction) <-chan []byte {
	out := make(chan []byte, 64)
	go func() {
		defer close(out)
		for instruction := range input {
			select {
			case <-ctx.Done():
				return
			case <-s.Shutdown:
				return
			case out <- []byte(instruction.Encode()):
			}
		}
	}()
	return out
}

func (s *Session) proxyLoop(ctx context.Context, guacdSend chan<- *guacd.Instruction, wsToGuacd <-chan *guacd.Instruction, guacdToWS <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.Shutdown:
			return
		case instruction, ok := <-wsToGuacd:
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-s.Shutdown:
				return
			case guacdSend <- instruction:
			}
		case payload, ok := <-guacdToWS:
			if !ok {
				return
			}
			_ = s.Conn.SetWriteDeadline(time.Now().Add(websocketWriteTimeout))
			if err := s.Conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: fmt.Errorf("websocket write failed: %w", err)}
				return
			}
		}
	}
}
