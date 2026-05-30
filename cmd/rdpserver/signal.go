package main

import (
	"context"
	"os/signal"
	"syscall"
)

func signalNotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}
