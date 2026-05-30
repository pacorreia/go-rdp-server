package broker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeAccounts struct {
	createErr error
	deleteCh  chan string
}

func (f *fakeAccounts) CreateTempUser(username, password string) error {
	if f.createErr != nil {
		return f.createErr
	}
	return nil
}

func (f *fakeAccounts) DeleteTempUser(username string) error {
	if f.deleteCh != nil {
		f.deleteCh <- username
	}
	return nil
}

func (f *fakeAccounts) AddToRDPGroup(username string) error {
	return nil
}

func TestBrokerCreatesAndDeletesTempAccount(t *testing.T) {
	requests := make(chan CredRequest, 1)
	events := make(chan SessionEvent, 1)
	shutdown := make(chan struct{})
	accounts := &fakeAccounts{deleteCh: make(chan string, 2)}

	b := &Broker{
		Requests: requests,
		Events:   events,
		Shutdown: shutdown,
		Accounts: accounts,
		PasswordGenerator: func() (string, error) {
			return "fixed-password", nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	reply := make(chan CredResponse, 1)
	requests <- CredRequest{SessionID: "s1", Reply: reply}
	resp := <-reply
	if resp.Err != nil {
		t.Fatalf("unexpected error from broker: %v", resp.Err)
	}
	if !strings.HasPrefix(resp.Username, "rdp_tmp_") {
		t.Fatalf("expected temp username prefix, got %q", resp.Username)
	}
	if resp.Password != "fixed-password" {
		t.Fatalf("unexpected password: %q", resp.Password)
	}

	events <- SessionEvent{SessionID: "s1", Type: SessionClosed}
	select {
	case deleted := <-accounts.deleteCh:
		if deleted != resp.Username {
			t.Fatalf("expected deleted username %q, got %q", resp.Username, deleted)
		}
	case <-time.After(time.Second):
		t.Fatal("expected temp account deletion on session close")
	}

	close(shutdown)
}

func TestBrokerReturnsCreateError(t *testing.T) {
	requests := make(chan CredRequest, 1)
	events := make(chan SessionEvent, 1)
	shutdown := make(chan struct{})
	accounts := &fakeAccounts{createErr: errors.New("create failed")}

	b := &Broker{
		Requests: requests,
		Events:   events,
		Shutdown: shutdown,
		Accounts: accounts,
		PasswordGenerator: func() (string, error) {
			return "fixed-password", nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	reply := make(chan CredResponse, 1)
	requests <- CredRequest{SessionID: "s1", Reply: reply}
	resp := <-reply
	if resp.Err == nil {
		t.Fatal("expected broker error")
	}
	close(shutdown)
}

func TestGeneratePassword(t *testing.T) {
	password, err := generatePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(password) < 32 {
		t.Fatalf("password too short: %d", len(password))
	}
}
