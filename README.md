# go-rdp-server

Channel-based Go service that bridges WebSocket clients to `guacd` and RDP with temporary local Windows credentials.

## Features

- HTML5 web endpoint at `/` with a Guacamole-based client
- WebSocket endpoint `/ws/rdp`
- Session workers that request temporary credentials through a broker goroutine
- Windows temp account lifecycle through Win32 API wrappers (`NetUserAdd`, `NetUserDel`, `NetLocalGroupAddMembers`, `LogonUser`)
- Session manager with `MAX_SESSIONS` admission control
- Graceful shutdown broadcast to all components

## Prerequisites

- Windows host with Remote Desktop enabled
- `guacd` installed and running with admin privileges on Windows
- Go 1.22+

## Configuration

Environment variables:

- `GUACD_HOST` (default `127.0.0.1`)
- `GUACD_PORT` (default `4822`)
- `RDP_HOST` (default `127.0.0.1`)
- `RDP_PORT` (default `3389`)
- `HTTP_PORT` (default `8080`)
- `MAX_SESSIONS` (default `10`)

## Run

```bash
go run ./cmd/rdpserver
```

Open `http://localhost:8080` in a browser and connect through the web client.
