---
title: Windows Service Operations
---

# Windows service operations

Service name: `go-rdp-server`

## Install

=== "Built-in flag (recommended)"

    ```powershell
    # Build the binary, then install it as a service in one step
    go build -o rdpserver.exe ./cmd/rdpserver
    .\rdpserver.exe -install-service
    ```

=== "Manual sc.exe"

    ```powershell
    go build -o rdpserver.exe ./cmd/rdpserver
    sc.exe create go-rdp-server binPath= "C:\path\to\rdpserver.exe" start= auto
    sc.exe description go-rdp-server "Browser-based RDP gateway service"
    ```

## Uninstall

```powershell
.\rdpserver.exe -uninstall-service
```

## Operate

```powershell
# Start the service
sc.exe start go-rdp-server

# Stop the service
sc.exe stop go-rdp-server

# Query service status
sc.exe query go-rdp-server
```

## Harden

!!! warning "Service account"
    Run the service under a dedicated least-privilege account, not `LocalSystem`. Restrict the account to the minimum rights needed to create local users and connect to `guacd`.

!!! tip "Automatic restart"
    Configure automatic restart on transient failures to keep the gateway available:

    ```powershell
    sc.exe failure go-rdp-server reset= 86400 actions= restart/5000/restart/5000/restart/5000
    ```

!!! note "Dependency ordering"
    Ensure the RDP port is reachable before the service starts.

## Firewall

Restrict inbound HTTP/WebSocket traffic to trusted origins. Example with Windows Firewall:

```powershell
# Allow only a specific management subnet on port 8080
New-NetFirewallRule -DisplayName "go-rdp-server" `
    -Direction Inbound -Protocol TCP -LocalPort 8080 `
    -RemoteAddress 10.0.0.0/24 -Action Allow
```
