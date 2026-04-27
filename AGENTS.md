# CloudPing — Agent Instructions

## Project Overview

CloudPing is a distributed network diagnostic platform providing ping, traceroute, TCP ping, DNS probe, and HTTP probe via a hub-agent architecture.

- **Hub**: Central coordinator (`cmd/globalping` + `pkg/hub`), listens :8080 (agent registration), :8082 (public API)
- **Agent**: Edge nodes (`cmd/globalping agent`), connect to hub via QUIC-over-mTLS
- **Web**: React/TypeScript frontend (`web/`), visualizes latency and routes
- **Bot**: Telegram bot interface (`cmd/globalping bot`), supports /ping, /traceroute, /probe
- **Workers**: Background job processors (`workers/email/`)

## Technology Stack

- **Backend**: Go 1.25, `github.com/quic-go/quic-go`, `github.com/gorilla/websocket`, `github.com/golang-jwt/jwt/v5`
- **Frontend**: TypeScript, React (Vite-based, inferred from `web/package.json`)
- **Protocol**: QUIC for hub-agent control; WebSocket for real-time frontend updates
- **Auth**: JWT for public API; mTLS (custom CA) for hub-agent
- **Metrics**: Prometheus
- **Deployment**: Docker + Docker Compose; Cloudflare Tunnel support

## Critical Architecture Rules

- Hub-agent communication **must** use QUIC. Do not suggest falling back to raw TCP or HTTP polling for control messages.
- The mTLS certificate pairs are dual-use (server-auth + client-auth). Do not generate separate client certs unless explicitly requested.
- DNS probes support UDP, TCP, DoT (RFC 7858), and DoH (RFC 8484). Prefer DoH only when user explicitly asks for "secure DNS."
- All probe results stream back via WebSocket to the frontend. The polling fallback in `web/` is deprecated; do not add new polling logic.

## Code Conventions

### Go
- Use `github.com/alecthomas/kong` for CLI flag parsing (already in `go.mod`)
- Prometheus metrics must use `cloudping_` prefix
- Error wrapping: use `fmt.Errorf("...: %w", err)` consistently
- Context cancellation: respect `ctx` in all QUIC and WebSocket handlers

### Frontend (TypeScript)
- Keep latency visualization canvas-based (using `tdewolff/canvas` Go-side rendering or raw Canvas API client-side — check `web/` for current pattern)
- The stargazer count badge in header is fetched client-side from GitHub API; cache aggressively

## Security Constraints

- **NEVER** log JWT signing keys or mTLS private keys at any log level.
- The `scripts/` folder contains dev helpers that generate **throwaway** CA/cert pairs. Do not use these scripts for production deployment.
- `.env` files are gitignored; `JWT_TOKEN` must be injected at runtime.

The following contents are commented out for transient reasons:

<!--## Testing & Debugging

- Generate test JWT: `bin/globalping jwt sign 2&gt;/dev/null`
- Local dev requires `/etc/hosts` entries: `agent1.example.one`, `hub.example.com` → `127.0.0.1`
- Hub and agent can be launched via scripts in `scripts/`

## MCP & External Tooling

If the agent supports MCP servers, the project exposes no custom MCP server yet. Use standard `github` MCP for release management and `fetch` MCP for reading IETF RFCs (7858, 8484) when implementing protocol changes.-->
