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

## Run as a Windows Service

The binary can run interactively or under the Windows Service Control Manager (SCM).  
When started by SCM, it handles service lifecycle events (`Start`, `Stop`, `Shutdown`) and performs graceful shutdown.

### Install

```powershell
go build -o rdpserver.exe ./cmd/rdpserver
sc.exe create go-rdp-server binPath= "C:\path\to\rdpserver.exe" start= auto
sc.exe description go-rdp-server "WebSocket to guacd RDP bridge service"
```

### Service operational best practices

- Run under a dedicated least-privilege service account with only required rights.
- Set recovery actions so transient failures auto-restart the service:

  ```powershell
  sc.exe failure go-rdp-server reset= 86400 actions= restart/5000/restart/5000/restart/5000
  ```

- Define required environment variables (`GUACD_HOST`, `GUACD_PORT`, `RDP_HOST`, `RDP_PORT`, `HTTP_PORT`, `MAX_SESSIONS`) in the service environment.
- Ensure `guacd` is installed and started before this service.
- Restrict inbound network access to the HTTP/WebSocket port to trusted clients.

### Manage

```powershell
sc.exe start go-rdp-server
sc.exe stop go-rdp-server
sc.exe query go-rdp-server
```
