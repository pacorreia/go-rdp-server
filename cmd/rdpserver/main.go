package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/pacorreia/go-rdp-server/internal/broker"
	"github.com/pacorreia/go-rdp-server/internal/session"
	"github.com/pacorreia/go-rdp-server/internal/web"
)

func main() {
	cfg := parseFlags()
	setupLogging(cfg.logLevel)

	if cfg.installService {
		if err := installService(serviceName, serviceDescription); err != nil {
			slog.Error("install service failed", "error", err)
		} else {
			slog.Info("service installed", "name", serviceName)
		}
		return
	}
	if cfg.uninstallService {
		if err := uninstallService(serviceName); err != nil {
			slog.Error("uninstall service failed", "error", err)
		} else {
			slog.Info("service uninstalled", "name", serviceName)
		}
		return
	}

	handled, err := runAsWindowsService(cfg)
	if err != nil {
		slog.Error("windows service setup failed", "error", err)
		return
	}
	if handled {
		return
	}

	if err := runConsole(cfg); err != nil {
		slog.Error("server exited with error", "error", err)
	}
}

func runConsole(cfg *config) error {
	ctx, stop := signalNotifyContext(context.Background())
	defer stop()
	return runServer(ctx, cfg)
}

func runServer(ctx context.Context, cfg *config) error {
	credRequests := make(chan broker.CredRequest)
	sessionEvents := make(chan broker.SessionEvent, 128)
	brokerEvents := make(chan broker.SessionEvent, 128)
	managerEvents := make(chan broker.SessionEvent, 128)
	shutdown := make(chan struct{})

	// Fan-out: every session event is delivered to both the broker and the manager.
	go func() {
		for {
			select {
			case <-shutdown:
				return
			case event, ok := <-sessionEvents:
				if !ok {
					return
				}
				select {
				case brokerEvents <- event:
				case <-shutdown:
					return
				}
				select {
				case managerEvents <- event:
				case <-shutdown:
					return
				}
			}
		}
	}()

	credentialBroker := &broker.Broker{
		Requests: credRequests,
		Events:   brokerEvents,
		Shutdown: shutdown,
	}
	manager := session.NewManager(cfg.maxSessions, managerEvents, shutdown)
	handlers := &web.Handlers{
		Manager:      manager,
		CredRequests: credRequests,
		SessionEvent: sessionEvents,
		Shutdown:     shutdown,
		Ctx:          ctx,
		RDPAddr:      fmt.Sprintf("%s:%s", cfg.rdpHost, cfg.rdpPort),
	}
	server := web.NewServer(":"+cfg.httpPort, handlers)

	go credentialBroker.Run(ctx)
	go manager.Run(ctx)
	errCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server error: %w", err)
		}
	}()

	slog.Info("rdp server listening", "port", cfg.httpPort)

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}

	close(shutdown)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	return runErr
}
