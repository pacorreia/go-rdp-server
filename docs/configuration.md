---
title: Configuration
---

# Configuration

All runtime behaviour is controlled through environment variables. No configuration file is required.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `GUACD_HOST` | `127.0.0.1` | Host where `guacd` is reachable |
| `GUACD_PORT` | `4822` | `guacd` listening port |
| `RDP_HOST` | `127.0.0.1` | Target RDP host |
| `RDP_PORT` | `3389` | Target RDP port |
| `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `MAX_SESSIONS` | `10` | Maximum concurrent active sessions |

## Recommended defaults for production

!!! tip "Session capacity"
    Keep `MAX_SESSIONS` aligned with the Windows RDP CAL count and available host memory. Excess requests are rejected at the WebSocket handshake before any credential is provisioned.

!!! warning "Network exposure"
    Use non-public network placement for `HTTP_PORT`. The embedded client performs no authentication — protect the endpoint with a reverse proxy or network policy.

!!! note "RDP target"
    `RDP_HOST` and `RDP_PORT` point `guacd` at the Windows RDP server. In a single-host setup both `go-rdp-server` and the RDP target are the same machine (`127.0.0.1`).

## Example: Docker environment

```yaml
services:
  go-rdp-server:
    image: ghcr.io/pacorreia/go-rdp-server:latest
    environment:
      - GUACD_HOST=guacd
      - GUACD_PORT=4822
      - RDP_HOST=rdp-target
      - RDP_PORT=3389
      - HTTP_PORT=8080
      - MAX_SESSIONS=5
    ports:
      - "8080:8080"
```

## Example: PowerShell (Windows Service)

```powershell
[System.Environment]::SetEnvironmentVariable("GUACD_HOST", "127.0.0.1", "Machine")
[System.Environment]::SetEnvironmentVariable("GUACD_PORT", "4822",      "Machine")
[System.Environment]::SetEnvironmentVariable("RDP_HOST",   "127.0.0.1", "Machine")
[System.Environment]::SetEnvironmentVariable("RDP_PORT",   "3389",      "Machine")
[System.Environment]::SetEnvironmentVariable("HTTP_PORT",  "8080",      "Machine")
[System.Environment]::SetEnvironmentVariable("MAX_SESSIONS","10",       "Machine")
```
