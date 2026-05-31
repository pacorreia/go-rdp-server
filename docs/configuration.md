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
| `-rdp-user` | `RDP_USER` | _(none)_ | Static RDP username; bypasses temporary account creation and per-user login |
| `-rdp-pass` | `RDP_PASS` | _(none)_ | Static RDP password; used together with `-rdp-user` |
| `-per-user-login` | `PER_USER_LOGIN` | `true` | Show a per-user login form; each browser session supplies its own credentials |
| `-allow-passwordless` | `ALLOW_PASSWORDLESS` | `false` | Deprecated compatibility flag; empty passwords are rejected in per-user login mode |
| `-http-port` | `HTTP_PORT` | `8080` | HTTP/WebSocket listen port |
| `-max-sessions` | `MAX_SESSIONS` | `10` | Maximum concurrent active sessions |
| `-max-conns-per-ip` | `MAX_CONNS_PER_IP` | `3` | Maximum concurrent WebSocket sessions per source IP (`0` disables the per-IP limit) |
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
    Use non-public network placement for `-http-port`. In the default per-user login mode the browser login form provides credential-based access control, but this should not be the only security boundary — protect the endpoint with a reverse proxy or network policy.

!!! warning "Passwordless account workaround"
    Empty passwords are rejected in per-user login mode. The `-allow-passwordless` flag is retained only for compatibility with previous releases and has no effect.

!!! warning "Reverse proxy deployments"
    The per-IP WebSocket limiter keys on the request source IP. In reverse-proxy deployments, configure trusted client-IP forwarding at the proxy layer and set `-max-conns-per-ip=0` if you need to disable the built-in per-IP limiter.

!!! note "Static credentials vs per-user login"
    `-rdp-user` / `-rdp-pass` take precedence over `-per-user-login`: when a static username is set, all browser sessions share the same RDP credentials and no login form is shown.

!!! note "RDP target"
    `-rdp-host` and `-rdp-port` point the built-in RDP client at the Windows RDP server. In a single-host setup the RDP target is the same machine (`127.0.0.1`).

## Example: explicit flags

```powershell
.\rdpserver.exe -rdp-host 192.168.1.10 -rdp-port 3389 -http-port 8080 -max-sessions 5 -log-level debug
```

## Example: static credentials (shared RDP account)

```powershell
.\rdpserver.exe -rdp-user admin -rdp-pass "S3cret!" -per-user-login=false
```

## Example: environment variables (Windows Service)

```powershell
[System.Environment]::SetEnvironmentVariable("RDP_HOST",    "127.0.0.1", "Machine")
[System.Environment]::SetEnvironmentVariable("RDP_PORT",    "3389",      "Machine")
[System.Environment]::SetEnvironmentVariable("HTTP_PORT",   "8080",      "Machine")
[System.Environment]::SetEnvironmentVariable("MAX_SESSIONS","10",        "Machine")
# Optional: enable per-user login (default is true)
[System.Environment]::SetEnvironmentVariable("PER_USER_LOGIN","true",    "Machine")
# Optional: enable passwordless-account workaround (disabled by default)
[System.Environment]::SetEnvironmentVariable("ALLOW_PASSWORDLESS","true","Machine")
```
