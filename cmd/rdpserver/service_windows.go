//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "go-rdp-server"
	serviceDescription = "Browser-based RDP gateway service"
)

func runAsWindowsService(cfg *config) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isService {
		return false, nil
	}
	return true, svc.Run(serviceName, &rdpService{cfg: cfg})
}

type rdpService struct {
	cfg *config
}

func (s *rdpService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- runServer(ctx, s.cfg)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case change := <-requests:
			switch change.Cmd {
			case svc.Interrogate:
				changes <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-runDone; err != nil {
					slog.Error("service stopped with error", "error", err)
				}
				return false, 0
			}
		case err := <-runDone:
			if err != nil {
				slog.Error("service exited with error", "error", err)
			}
			return false, 0
		}
	}
}

// installService registers the current executable as a Windows Service.
func installService(name, desc string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	existing, err := m.OpenService(name)
	if err == nil {
		existing.Close()
		return fmt.Errorf("service %q already exists", name)
	}

	s, err := m.CreateService(name, exePath, mgr.Config{
		DisplayName: name,
		Description: desc,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	s.Close()
	return nil
}

// uninstallService removes the Windows Service registration.
func uninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service %q: %w", name, err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service %q: %w", name, err)
	}
	return nil
}
