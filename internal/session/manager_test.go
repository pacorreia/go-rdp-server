package session

import (
	"context"
	"testing"
	"time"

	"github.com/pacorreia/go-rdp-server/internal/broker"
)

const admissionRetryInterval = 10 * time.Millisecond

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

	deadline := time.After(1 * time.Second)
	ticker := time.NewTicker(admissionRetryInterval)
	defer ticker.Stop()
	for {
		err := manager.Admit("session-2")
		if err == nil {
			break
		}
		if err != ErrMaxSessionsReached {
			t.Fatalf("expected ErrMaxSessionsReached while waiting, got %v", err)
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for session close event processing")
		case <-ticker.C:
		}
	}

	close(shutdown)
}
