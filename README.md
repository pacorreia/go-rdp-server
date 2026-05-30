# go-rdp-server

`go-rdp-server` is a channel-based Go service that bridges browser WebSocket clients to `guacd` and Windows RDP sessions using temporary local user accounts.

## Full review snapshot

Current implementation is production-oriented and includes:

- Session admission control (`MAX_SESSIONS`) through a dedicated manager goroutine
- Temporary Windows account lifecycle management through broker events
- WebSocket ↔ `guacd` proxy workers for each active session
- Graceful shutdown propagation across broker, manager, and HTTP server
- Native Windows Service mode (`go-rdp-server`) with SCM lifecycle handling
- CI for pull requests and release automation for tagged binaries

## Architecture at a glance

1. `internal/web` upgrades `/ws/rdp` requests and creates session workers.
2. `internal/session` enforces capacity and proxies WebSocket and `guacd` traffic.
3. `internal/broker` provisions and cleans temporary Windows users per session.
4. `internal/guacd` handles Guacamole protocol instruction encode/decode and TCP IO.

## Prerequisites

- Windows host with Remote Desktop enabled
- `guacd` installed and running with required privileges
- Go 1.22+

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `GUACD_HOST` | `127.0.0.1` | `guacd` host |
| `GUACD_PORT` | `4822` | `guacd` TCP port |
| `RDP_HOST` | `127.0.0.1` | RDP target hostname/IP |
| `RDP_PORT` | `3389` | RDP target port |
| `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `MAX_SESSIONS` | `10` | Concurrent session cap |

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
sc.exe create go-rdp-server binPath= "C:\path\to\rdpserver.exe" start= auto
sc.exe description go-rdp-server "WebSocket to guacd RDP bridge service"
```

### Recommended hardening

- Use a dedicated least-privilege service account
- Configure automatic recovery:

  ```powershell
  sc.exe failure go-rdp-server reset= 86400 actions= restart/5000/restart/5000/restart/5000
  ```

- Restrict inbound access to trusted networks
- Ensure `guacd` starts before this service

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
