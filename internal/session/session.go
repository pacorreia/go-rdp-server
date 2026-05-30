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
}

func (s *Session) Run(ctx context.Context) {
	defer func() {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionClosed}
		_ = s.Conn.Close()
	}()

	responseCh := make(chan broker.CredResponse, 1)
	s.CredRequests <- broker.CredRequest{SessionID: s.ID, Reply: responseCh}

	var cred broker.CredResponse
	select {
	case <-ctx.Done():
		return
	case <-s.Shutdown:
		return
	case cred = <-responseCh:
	}
	if cred.Err != nil {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: cred.Err}
		return
	}

	client, err := guacd.NewClient(ctx, s.GuacdAddr)
	if err != nil {
		s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: err}
		return
	}

	client.Send <- &guacd.Instruction{Opcode: "select", Args: []string{"rdp"}}
	client.Send <- &guacd.Instruction{Opcode: "size", Args: []string{"1920", "1080", "96"}}
	client.Send <- &guacd.Instruction{Opcode: "connect", Args: []string{
		"hostname", s.RDPHost,
		"port", s.RDPPort,
		"username", cred.Username,
		"password", cred.Password,
	}}

	wsToGuacd := make(chan *guacd.Instruction, 64)
	guacdToWS := make(chan []byte, 64)

	go func() {
		defer close(wsToGuacd)
		for {
			_, message, err := s.Conn.ReadMessage()
			if err != nil {
				return
			}
			instruction, err := guacd.Decode(strings.TrimSpace(string(message)))
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-s.Shutdown:
				return
			case wsToGuacd <- instruction:
			}
		}
	}()

	go func() {
		defer close(guacdToWS)
		for instruction := range client.Recv {
			select {
			case <-ctx.Done():
				return
			case <-s.Shutdown:
				return
			case guacdToWS <- []byte(instruction.Encode()):
			}
		}
	}()

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
			case client.Send <- instruction:
			}
		case payload, ok := <-guacdToWS:
			if !ok {
				return
			}
			_ = s.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := s.Conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				s.Events <- broker.SessionEvent{SessionID: s.ID, Type: broker.SessionError, Err: fmt.Errorf("websocket write failed: %w", err)}
				return
			}
		}
	}
}
