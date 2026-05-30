package broker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// CredRequest requests temporary credentials for an RDP session.
type CredRequest struct {
	SessionID string
	Reply     chan CredResponse
}

// CredResponse contains temporary credentials returned by the broker.
type CredResponse struct {
	SessionID string
	Username  string
	Password  string
	Err       error
}

type SessionEventType string

const (
	SessionOpened SessionEventType = "opened"
	SessionClosed SessionEventType = "closed"
	SessionError  SessionEventType = "error"
)

// SessionEvent reports lifecycle events from session workers.
type SessionEvent struct {
	SessionID string
	Type      SessionEventType
	Err       error
}

// Broker handles temp account lifecycle using channels only.
type Broker struct {
	Requests  <-chan CredRequest
	Responses chan<- CredResponse
	Events    <-chan SessionEvent
	Shutdown  <-chan struct{}
}

// Run starts broker loop.
func (b *Broker) Run(ctx context.Context) {
	sessionAccounts := make(map[string]string)
	for {
		select {
		case <-ctx.Done():
			b.cleanupAll(sessionAccounts)
			return
		case <-b.Shutdown:
			b.cleanupAll(sessionAccounts)
			return
		case req := <-b.Requests:
			username := "rdp_tmp_" + uuid.NewString()
			password, err := generatePassword()
			if err == nil {
				err = CreateTempUser(username, password)
			}
			if err == nil {
				err = AddToRDPGroup(username)
			}
			if err != nil {
				_ = DeleteTempUser(username)
				b.respond(req, CredResponse{SessionID: req.SessionID, Err: err})
				continue
			}

			sessionAccounts[req.SessionID] = username
			b.respond(req, CredResponse{
				SessionID: req.SessionID,
				Username:  username,
				Password:  password,
			})
		case event := <-b.Events:
			if event.Type != SessionClosed {
				continue
			}
			username, ok := sessionAccounts[event.SessionID]
			if !ok {
				continue
			}
			_ = DeleteTempUser(username)
			delete(sessionAccounts, event.SessionID)
		}
	}
}

func (b *Broker) respond(req CredRequest, response CredResponse) {
	if req.Reply != nil {
		req.Reply <- response
		return
	}
	if b.Responses != nil {
		b.Responses <- response
	}
}

func (b *Broker) cleanupAll(accounts map[string]string) {
	for sessionID, username := range accounts {
		_ = DeleteTempUser(username)
		delete(accounts, sessionID)
	}
}

func generatePassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("unable to generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
