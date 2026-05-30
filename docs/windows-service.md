---
title: Windows Service Operations
---

# Windows service operations

Service name: `go-rdp-server`

## Install

```powershell
go build -o rdpserver.exe ./cmd/rdpserver
sc.exe create go-rdp-server binPath= "C:\path\to\rdpserver.exe" start= auto
sc.exe description go-rdp-server "WebSocket to guacd RDP bridge service"
```

## Operate

```powershell
sc.exe start go-rdp-server
sc.exe stop go-rdp-server
sc.exe query go-rdp-server
```

## Best practices

- Use a dedicated least-privilege account for the service identity.
- Configure automatic restart on transient failures:

  ```powershell
  sc.exe failure go-rdp-server reset= 86400 actions= restart/5000/restart/5000/restart/5000
  ```

- Ensure `guacd` availability before service startup.
- Restrict inbound HTTP/WebSocket traffic to trusted origins and networks.
