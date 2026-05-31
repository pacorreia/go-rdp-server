---
title: Home
---

# go-rdp-server

[![Build](https://github.com/pacorreia/go-rdp-server/actions/workflows/build.yml/badge.svg)](https://github.com/pacorreia/go-rdp-server/actions/workflows/build.yml)
[![Latest Release](https://img.shields.io/github/v/release/pacorreia/go-rdp-server?sort=semver)](https://github.com/pacorreia/go-rdp-server/releases)
[![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/pacorreia/go-rdp-server)](https://github.com/pacorreia/go-rdp-server/blob/main/LICENSE)

A lightweight, browser-based RDP gateway that brokers temporary Windows credentials and tunnels WebSocket traffic through the Guacamole protocol to `guacd`.

## Features

🌐 **Browser-Based RDP**

- Zero client installation — pure WebSocket from the browser
- Guacamole protocol bridge between browser and `guacd`
- Embedded HTML/JS client served directly from the binary

🔐 **Temporary Credential Brokering**

- Provisions short-lived local Windows accounts per session
- Credentials are automatically removed on session close or error
- Isolates each session with a unique account identity

🔄 **Session Management**

- Configurable maximum concurrent sessions (`MAX_SESSIONS`)
- Admission control rejects connections when capacity is reached
- Graceful cleanup on disconnect, error, or shutdown

🪟 **Windows Service Ready**

- Runs as a native Windows Service via `golang.org/x/sys/windows/svc`
- SCM start/stop/shutdown signal handling
- Automatic recovery configuration support

## Quick Links

- **[Architecture](architecture.md)** — Runtime flow, component diagram, and shutdown behaviour
- **[Configuration](configuration.md)** — All environment variables and recommended defaults
- **[Windows Service](windows-service.md)** — Install, operate, and harden the service
- **[Development](development.md)** — Build, test, and release workflows

## Architecture Overview

```mermaid
flowchart TD
    Browser["Browser (WebSocket)"] -->|"/ws/rdp"| Web["internal/web\nHTTP + WebSocket"]
    Web -->|CredRequest chan| Broker["internal/broker\nCredential broker"]
    Broker -->|Win32 NetUserAdd| WinAccounts["Windows local accounts"]
    Web -->|SessionEvent chan| Manager["internal/session\nSession manager"]
    Web -->|TCP| Guacd["guacd\n(Guacamole daemon)"]
    Guacd -->|RDP| WinRDP["Windows RDP Server"]
```

## Getting Started

### Prerequisites

- A Windows host with RDP enabled and `guacd` reachable
- Go 1.24+ (for building from source)

### Installation

=== "Binary (Windows)"

    Download the pre-built binary from the [releases page](https://github.com/pacorreia/go-rdp-server/releases):

    ```powershell
    # Set configuration via environment variables
    $env:GUACD_HOST = "127.0.0.1"
    $env:GUACD_PORT = "4822"
    $env:RDP_HOST   = "127.0.0.1"
    $env:RDP_PORT   = "3389"
    $env:HTTP_PORT  = "8080"
    $env:MAX_SESSIONS = "10"

    # Run the server
    .\rdpserver.exe
    ```

=== "Build from Source"

    ```bash
    git clone https://github.com/pacorreia/go-rdp-server
    cd go-rdp-server

    # Cross-compile for Windows
    GOOS=windows GOARCH=amd64 go build -o rdpserver.exe ./cmd/rdpserver
    ```

=== "Windows Service"

    ```powershell
    # Build and register as a Windows Service
    go build -o rdpserver.exe ./cmd/rdpserver
    sc.exe create go-rdp-server binPath= "C:\path\to\rdpserver.exe" start= auto
    sc.exe description go-rdp-server "WebSocket to guacd RDP bridge service"
    sc.exe start go-rdp-server
    ```

    See [Windows Service operations](windows-service.md) for full details.

### First Use

After starting the server, open `http://<host>:8080` in your browser. The embedded client will connect automatically over WebSocket to `/ws/rdp`.

```bash
# Verify the server is running
curl http://localhost:8080/
```
