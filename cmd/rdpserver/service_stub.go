//go:build !windows

package main

import "errors"

const (
	serviceName        = "go-rdp-server"
	serviceDescription = "Browser-based RDP gateway service"
)

func runAsWindowsService(_ *config) (bool, error) {
	return false, nil
}

func installService(_, _ string) error {
	return errors.New("service installation is only supported on Windows")
}

func uninstallService(_ string) error {
	return errors.New("service uninstallation is only supported on Windows")
}
