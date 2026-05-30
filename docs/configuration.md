# Configuration

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

- Keep `MAX_SESSIONS` aligned with host capacity and RDP licensing.
- Use non-public network placement for `HTTP_PORT`.
- Configure host firewall to allow only trusted client ranges.
