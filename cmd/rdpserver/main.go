package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/session"
	"github.com/pacorreia/go-rdp-server/internal/web"
)

func main() {
	guacdHost := getEnv("GUACD_HOST", "127.0.0.1")
	guacdPort := getEnv("GUACD_PORT", "4822")
	rdpHost := getEnv("RDP_HOST", "127.0.0.1")
	rdpPort := getEnv("RDP_PORT", "3389")
	httpPort := getEnv("HTTP_PORT", "8080")
	maxSessions := getEnvInt("MAX_SESSIONS", 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	credRequests := make(chan broker.CredRequest)
	credResponses := make(chan broker.CredResponse)
	sessionEvents := make(chan broker.SessionEvent, 128)
	shutdown := make(chan struct{})

	credentialBroker := &broker.Broker{
		Requests:  credRequests,
		Responses: credResponses,
		Events:    sessionEvents,
		Shutdown:  shutdown,
	}
	manager := session.NewManager(maxSessions, sessionEvents, shutdown)
	handlers := &web.Handlers{
		Manager:      manager,
		CredRequests: credRequests,
		SessionEvent: sessionEvents,
		Shutdown:     shutdown,
		GuacdAddr:    fmt.Sprintf("%s:%s", guacdHost, guacdPort),
		RDPHost:      rdpHost,
		RDPPort:      rdpPort,
	}
	server := web.NewServer(":"+httpPort, handlers)

	go credentialBroker.Run(ctx)
	go manager.Run(ctx)
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
			cancel()
		}
	}()

	log.Printf("rdp server listening on :%s", httpPort)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case <-sigCh:
	}

	close(shutdown)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := getEnv(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
