---
title: Configuration
---

# Configuration

All runtime behaviour is controlled through environment variables. No configuration file is required.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `RDP_HOST` | `127.0.0.1` | Target RDP host |
| `RDP_PORT` | `3389` | Target RDP port |
| `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `MAX_SESSIONS` | `10` | Maximum concurrent active sessions |

## Recommended defaults for production

!!! tip "Session capacity"
    Keep `MAX_SESSIONS` aligned with the Windows RDP CAL count and available host memory. Excess requests are closed with a retry-later WebSocket response before any credential is provisioned.

!!! warning "Network exposure"
    Use non-public network placement for `HTTP_PORT`. The embedded client performs no authentication — protect the endpoint with a reverse proxy or network policy.

!!! note "RDP target"
    `RDP_HOST` and `RDP_PORT` point the built-in RDP client at the Windows RDP server. In a single-host setup the RDP target is the same machine (`127.0.0.1`).

## Example: PowerShell (Windows Service)

```powershell
[System.Environment]::SetEnvironmentVariable("RDP_HOST",   "127.0.0.1", "Machine")
[System.Environment]::SetEnvironmentVariable("RDP_PORT",   "3389",      "Machine")
[System.Environment]::SetEnvironmentVariable("HTTP_PORT",  "8080",      "Machine")
[System.Environment]::SetEnvironmentVariable("MAX_SESSIONS","10",       "Machine")
```
