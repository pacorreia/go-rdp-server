package session

import (
	"context"
	"testing"
	"time"

	"github.com/pacorreia/go-rdp-server/internal/broker"
)

func TestManagerAdmitAndRelease(t *testing.T) {
	events := make(chan broker.SessionEvent, 8)
	shutdown := make(chan struct{})
	manager := NewManager(1, events, shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Run(ctx)

	if err := manager.Admit("session-1"); err != nil {
		t.Fatalf("expected first admission to succeed: %v", err)
	}

	if err := manager.Admit("session-2"); err != ErrMaxSessionsReached {
		t.Fatalf("expected ErrMaxSessionsReached, got %v", err)
	}

	events <- broker.SessionEvent{SessionID: "session-1", Type: broker.SessionClosed}
	time.Sleep(10 * time.Millisecond)

	if err := manager.Admit("session-2"); err != nil {
		t.Fatalf("expected admission after close to succeed: %v", err)
	}

	close(shutdown)
}
