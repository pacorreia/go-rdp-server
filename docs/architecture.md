---
title: Architecture
---

# Architecture

## Components

| Package | Responsibility |
| --- | --- |
| `cmd/rdpserver` | Process bootstrap, shutdown, and Windows service mode selection |
| `internal/web` | HTTP server, index handler, WebSocket upgrade, and session spawn |
| `internal/session` | Session admission manager and per-session proxy workers |
| `internal/broker` | Temporary Windows account lifecycle and credential broker loop |
| `internal/display` | Pure-Go RDP client interface and `grdp`-backed implementation |
| `ui` | Embedded static HTML/JS canvas client |

## Component diagram

```mermaid
flowchart TD
    Browser["Browser (WebSocket)"] -->|"/ws/rdp"| Web["internal/web\nHTTP + WebSocket"]
    Web -->|CredRequest chan| Broker["internal/broker\nCredential broker"]
    Broker -->|Win32 NetUserAdd| WinAccounts["Windows local accounts"]
    Web -->|SessionEvent chan| Manager["internal/session\nSession manager"]
    Manager -->|display.Connect| Display["internal/display\ngrdp RDP client"]
    Display -->|RDP| WinRDP["Windows RDP Server"]
```

## Runtime flow

1. Client connects to `/ws/rdp`.
2. Session manager enforces `MAX_SESSIONS`.
3. Broker provisions a temporary local user and returns credentials.
4. Session worker opens an RDP connection via the pure-Go `grdp` client.
5. Bitmap tile updates are JPEG-encoded and forwarded to the browser as JSON WebSocket messages.
6. Keyboard and mouse input arrives as JSON messages and is forwarded to the RDP session.
7. On close, error, or shutdown the temporary account is deleted and capacity is released.

```mermaid
sequenceDiagram
    participant Browser
    participant Web
    participant Manager
    participant Broker
    participant Display

    Browser->>Web: WebSocket connect /ws/rdp
    Web->>Manager: Admit(sessionID)
    Web->>Broker: CredRequest
    Broker-->>Web: CredResponse (username, password)
    Web->>Display: display.Connect (grdp)
    loop Proxy
        Display-->>Web: Tile (JPEG bitmap)
        Web-->>Browser: JSON tile message
        Browser->>Web: JSON input message
        Web->>Display: KeyDown/MouseMove/…
    end
    Browser->>Web: close
    Web->>Manager: SessionClosed event
    Web->>Broker: SessionClosed event → delete temp user
```

## Wire protocol

Messages between browser and server are JSON objects sent over WebSocket.

**Server → Browser (tile update)**

```json
{ "type": "tile", "x": 0, "y": 0, "w": 200, "h": 100, "data": "<base64 JPEG>" }
```

**Browser → Server (input events)**

```json
{ "type": "keydown",    "scancode": 28 }
{ "type": "keyup",      "scancode": 28 }
{ "type": "mousemove",  "x": 320, "y": 240 }
{ "type": "mousedown",  "button": 0, "x": 320, "y": 240 }
{ "type": "mouseup",    "button": 0, "x": 320, "y": 240 }
{ "type": "mousewheel", "delta": 3 }
```

## Shutdown behaviour

| Mode | Signal source |
| --- | --- |
| Console | OS signals (`SIGINT`, `SIGTERM`) cancel context |
| Windows Service | SCM stop/shutdown events cancel context |

A shared shutdown channel closes all worker loops and triggers temporary account cleanup before the process exits.
