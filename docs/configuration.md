---
title: Configuration
---

# Configuration

Runtime behaviour can be controlled via **CLI flags** or **environment variables**. CLI flags always take precedence over environment variables.

## CLI flags

Run `rdpserver.exe -help` to see all available flags.

| Flag | Env variable | Default | Purpose |
| --- | --- | --- | --- |
| `-rdp-host` | `RDP_HOST` | `127.0.0.1` | RDP target host |
| `-rdp-port` | `RDP_PORT` | `3389` | RDP target port |
| `-http-port` | `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `-max-sessions` | `MAX_SESSIONS` | `10` | Maximum concurrent active sessions |
| `-log-level` | — | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `-install-service` | — | — | Install as a Windows Service and exit (Windows only) |
| `-uninstall-service` | — | — | Uninstall the Windows Service and exit (Windows only) |

## Service install / uninstall

```powershell
# Install the service (uses the current executable path)
.\rdpserver.exe -install-service

# Uninstall the service
.\rdpserver.exe -uninstall-service
```

!!! note "Privileges"
    Installing or uninstalling a Windows Service requires administrator privileges.

## Recommended defaults for production

!!! tip "Session capacity"
    Keep `-max-sessions` aligned with the Windows RDP CAL count and available host memory. Excess requests are closed with a retry-later WebSocket response before any credential is provisioned.

!!! warning "Network exposure"
    Use non-public network placement for `-http-port`. The embedded client performs no authentication — protect the endpoint with a reverse proxy or network policy.

!!! note "RDP target"
    `-rdp-host` and `-rdp-port` point the built-in RDP client at the Windows RDP server. In a single-host setup the RDP target is the same machine (`127.0.0.1`).

## Example: explicit flags

```powershell
.\rdpserver.exe -rdp-host 192.168.1.10 -rdp-port 3389 -http-port 8080 -max-sessions 5 -log-level debug
```

## Example: environment variables (Windows Service)

```powershell
[System.Environment]::SetEnvironmentVariable("RDP_HOST",    "127.0.0.1", "Machine")
[System.Environment]::SetEnvironmentVariable("RDP_PORT",    "3389",      "Machine")
[System.Environment]::SetEnvironmentVariable("HTTP_PORT",   "8080",      "Machine")
[System.Environment]::SetEnvironmentVariable("MAX_SESSIONS","10",        "Machine")
```
