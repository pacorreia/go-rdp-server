package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
)

// config holds all runtime settings, resolved from CLI flags with environment
// variable fallbacks.
type config struct {
	rdpHost          string
	rdpPort          string
	httpPort         string
	maxSessions      int
	logLevel         string
	installService   bool
	uninstallService bool
}

// parseFlags parses command-line flags, using environment variables as defaults
// when flags are not explicitly set.  CLI flags always take precedence.
func parseFlags() *config {
	cfg := &config{}

	flag.StringVar(&cfg.rdpHost, "rdp-host", getEnv("RDP_HOST", "127.0.0.1"), "RDP target host")
	flag.StringVar(&cfg.rdpPort, "rdp-port", getEnv("RDP_PORT", "3389"), "RDP target port")
	flag.StringVar(&cfg.httpPort, "http-port", getEnv("HTTP_PORT", "8080"), "HTTP/WebSocket listen port")
	flag.IntVar(&cfg.maxSessions, "max-sessions", getEnvInt("MAX_SESSIONS", 10), "Maximum concurrent sessions (must be > 0)")
	flag.StringVar(&cfg.logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.BoolVar(&cfg.installService, "install-service", false, "Install as a Windows Service and exit (Windows only)")
	flag.BoolVar(&cfg.uninstallService, "uninstall-service", false, "Uninstall the Windows Service and exit (Windows only)")

	flag.Parse()
	return cfg
}

// setupLogging initialises the default slog handler with the requested level.
func setupLogging(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
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
