# Backend Design: Embedded AI Terminal

## Overview

Add a PTY session manager and loopback WebSocket transport around PM-018 launch preparation. The subsystem remains a local service wired through the existing Go server.

## Session Model

| Type                  | Key Fields                                                                |
|-----------------------|---------------------------------------------------------------------------|
| `EmbeddedSession`     | `id`, `itemId`, `workspaceId`, `provider`, `intent`, `state`, `startedAt` |
| `SessionChannelGrant` | `sessionId`, `token`, `expiresAt`                                         |
| `TerminalSize`        | `columns`, `rows`                                                         |
| `SessionEvent`        | `type`, `data`, `exitCode`, `message`                                     |

Session states are `starting`, `running`, `exited`, `cancelled`, and `failed`. Session IDs and channel tokens are cryptographically random. Tokens are compared in constant time and scoped to one session.

## API Contract

| Method | Endpoint                               | Request                         | Response                  |
|--------|----------------------------------------|---------------------------------|---------------------------|
| POST   | `/api/items/{id}/ai-sessions/embedded` | Provider, intent, terminal size | Session and channel grant |
| GET    | `/api/ai/sessions/{sessionId}`         | None                            | Session status            |
| DELETE | `/api/ai/sessions/{sessionId}`         | None                            | Final session status      |
| WS     | `/api/ai/sessions/{sessionId}/channel` | Channel token                   | Terminal event stream     |

The channel accepts typed input, resize, heartbeat, and cancel messages. The server emits output, state, warning, and exit messages. Binary frames are rejected.

## Controls

- Reuse PM-018 workspace, eligibility, provider, template, and manifest validation.
- Start the process directly in the canonical workspace root without a shell unless the provider adapter explicitly requires one.
- Apply maximum concurrent sessions, output frame, retained buffer, and terminal dimension limits.
- Keep a short reconnect buffer and grace period; terminate after lease expiry.
- Kill the process group on cancellation, expiry, server shutdown, or failed startup.
- Redact channel grants and terminal content from logs and audit events.

## PTY Dependency

`github.com/creack/pty` v1.1.24 provides the maintained Go PTY API. It supports Unix PTYs on Linux, macOS, FreeBSD, DragonFly, and OpenBSD. Windows does not expose the required package implementation, so embedded mode is unavailable there while external launch remains the compatibility path.

## Design Decisions

| Decision                  | Rationale                                                  |
|---------------------------|------------------------------------------------------------|
| Session manager owns PTY  | Centralizes lifecycle, limits, cleanup, and audit behavior |
| One WebSocket per session | Keeps routing and authorization narrow                     |
