package session

import (
	"context"
	"errors"

	"github.com/pacorreia/go-rdp-server/internal/broker"
)

var ErrMaxSessionsReached = errors.New("max sessions reached")

type admissionRequest struct {
	SessionID string
	Reply     chan error
}

// Manager tracks active sessions from a dedicated goroutine.
type Manager struct {
	Events    <-chan broker.SessionEvent
	Shutdown  <-chan struct{}
	admission chan admissionRequest
	max       int
}

func NewManager(max int, events <-chan broker.SessionEvent, shutdown <-chan struct{}) *Manager {
	return &Manager{
		Events:    events,
		Shutdown:  shutdown,
		admission: make(chan admissionRequest),
		max:       max,
	}
}

func (m *Manager) Admit(sessionID string) error {
	reply := make(chan error, 1)
	m.admission <- admissionRequest{SessionID: sessionID, Reply: reply}
	return <-reply
}

func (m *Manager) Run(ctx context.Context) {
	active := make(map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.Shutdown:
			return
		case req := <-m.admission:
			if len(active) >= m.max {
				req.Reply <- ErrMaxSessionsReached
				continue
			}
			active[req.SessionID] = struct{}{}
			req.Reply <- nil
		case event := <-m.Events:
			switch event.Type {
			case broker.SessionClosed, broker.SessionError:
				delete(active, event.SessionID)
			}
		}
	}
}
