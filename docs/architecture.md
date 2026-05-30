---
title: Architecture
---

# Architecture

## Components

- `cmd/rdpserver`: process bootstrap, shutdown, and Windows service mode selection
- `internal/web`: HTTP server, index handler, WebSocket upgrade and session spawn
- `internal/session`: session admission manager and per-session proxy workers
- `internal/broker`: temporary Windows account lifecycle and credential broker loop
- `internal/guacd`: Guacamole protocol instruction codec and TCP client
- `ui`: embedded static HTML client

```mermaid
flowchart TD
    Browser["Browser (WebSocket)"] -->|"/ws/rdp"| Web["internal/web\nHTTP + WebSocket"]
    Web -->|CredRequest chan| Broker["internal/broker\nCredential broker"]
    Broker -->|Win32 NetUserAdd| WinAccounts["Windows local accounts"]
    Web -->|SessionEvent chan| Manager["internal/session\nSession manager"]
    Web -->|TCP| Guacd["guacd\n(Guacamole daemon)"]
    Guacd -->|RDP| WinRDP["Windows RDP Server"]
```

## Runtime flow

1. Client connects to `/ws/rdp`.
2. Session manager enforces `MAX_SESSIONS`.
3. Broker provisions a temporary local user and returns credentials.
4. Session worker connects to `guacd` and sends the RDP handshake.
5. WebSocket and `guacd` traffic are proxied bidirectionally.
6. On close/error/shutdown, temporary account is deleted and capacity is released.

```mermaid
sequenceDiagram
    participant Browser
    participant Web
    participant Manager
    participant Broker
    participant Guacd

    Browser->>Web: WebSocket connect /ws/rdp
    Web->>Manager: Admit(sessionID)
    Web->>Broker: CredRequest
    Broker-->>Web: CredResponse (username, password)
    Web->>Guacd: TCP connect + RDP handshake
    loop Proxy
        Browser->>Web: guacd instruction
        Web->>Guacd: forward
        Guacd-->>Web: guacd instruction
        Web-->>Browser: forward
    end
    Browser->>Web: close
    Web->>Manager: SessionClosed event
    Web->>Broker: SessionClosed event → delete temp user
```

## Shutdown behavior

- Console mode: OS signals cancel context.
- Service mode: SCM stop/shutdown events cancel context.
- Shared shutdown channel closes worker loops and triggers cleanup.
