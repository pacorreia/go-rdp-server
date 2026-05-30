//go:build windows

package main

import (
	"context"
	"log"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "go-rdp-server"

func runAsWindowsService() (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isService {
		return false, nil
	}
	return true, svc.Run(serviceName, &rdpService{})
}

type rdpService struct{}

func (s *rdpService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- runServer(ctx)
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
					log.Printf("service stopped with error: %v", err)
				}
				return false, 0
			}
		case err := <-runDone:
			if err != nil {
				log.Printf("service exited with error: %v", err)
			}
			return false, 0
		}
	}
}
