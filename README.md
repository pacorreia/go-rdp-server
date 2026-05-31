# go-rdp-server

`go-rdp-server` is a channel-based Go service that bridges browser WebSocket clients to Windows RDP sessions using a pure-Go RDP client and temporary local user accounts.

## Full review snapshot

Current implementation is production-oriented and includes:

- Session admission control (`MAX_SESSIONS`) through a dedicated manager goroutine
- Temporary Windows account lifecycle management through broker events
- Pure-Go RDP client (`github.com/nakagami/grdp`) — no external `guacd` daemon needed
- JSON WebSocket wire protocol with base64-encoded JPEG tile updates and canvas-based browser UI
- Graceful shutdown propagation across broker, manager, and HTTP server
- Native Windows Service mode (`go-rdp-server`) with SCM lifecycle handling
- CI for pull requests and release automation for tagged binaries

## Architecture at a glance

1. `internal/web` upgrades `/ws/rdp` requests and creates session workers.
2. `internal/session` enforces capacity and proxies WebSocket and RDP traffic.
3. `internal/broker` provisions and cleans temporary Windows users per session.
4. `internal/display` wraps the `grdp` RDP client and exposes a `RDPSession` interface.

## Prerequisites

- Windows host with Remote Desktop enabled
- Go 1.24+

## Configuration

| Flag | Env variable | Default | Description |
| --- | --- | --- | --- |
| `-rdp-host` | `RDP_HOST` | `127.0.0.1` | RDP target hostname/IP |
| `-rdp-port` | `RDP_PORT` | `3389` | RDP target port |
| `-http-port` | `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `-max-sessions` | `MAX_SESSIONS` | `10` | Concurrent session cap |
| `-log-level` | — | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `-install-service` | — | — | Install as Windows Service and exit (Windows only) |
| `-uninstall-service` | — | — | Uninstall Windows Service and exit (Windows only) |

## Local run

```bash
go run ./cmd/rdpserver
```

Open `http://localhost:8080`.

## Windows Service run

When started by SCM, the binary runs in service mode and handles `Start`, `Stop`, and `Shutdown` events.

### Install

```powershell
go build -o rdpserver.exe ./cmd/rdpserver
.\rdpserver.exe -install-service
```

### Recommended hardening

- Use a dedicated least-privilege service account
- Configure automatic recovery:

  ```powershell
  sc.exe failure go-rdp-server reset= 86400 actions= restart/5000/restart/5000/restart/5000
  ```

- Restrict inbound access to trusted networks

### Service management

```powershell
sc.exe start go-rdp-server
sc.exe stop go-rdp-server
sc.exe query go-rdp-server
```

## Validation

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build ./...
```

## Documentation

- Source docs: [`/docs`](./docs)
- Zensical guide: [`/docs/zensical.md`](./docs/zensical.md)
- GitHub Pages is published from the `gh-pages` branch via workflow.
