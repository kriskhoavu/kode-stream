# Scenarios: PM-020 Embedded AI Terminal

## Scenario List

| #   | Title                        | Expected Result                                         |
|-----|------------------------------|---------------------------------------------------------|
| 1   | Start embedded AI session    | Terminal connects to a workspace-contained provider PTY |
| 2   | Resize and interact          | Input, output, and dimensions propagate without loss    |
| 3   | Reconnect to a session       | Buffered output resumes within the reconnect grace time |
| 4   | Disconnect or cancel session | Lease and cleanup rules prevent orphaned processes      |
| 5   | Fall back to external launch | Existing PM-018 external launch remains available       |

## Flow 1: Embedded AI Session

```text
User chooses Embedded terminal
  -> backend reuses PM-018 validation and context generation
  -> PTY starts provider in registered workspace root
  -> API returns opaque session ID and one-time channel token
  -> browser opens loopback WebSocket
  -> input, output, resize, and lifecycle events flow over channel
  -> exit, cancellation, lease expiry, or server shutdown cleans process
```

## Acceptance Scenarios

- Embedded sessions cannot start outside registered workspace roots.
- Session channel tokens are short-lived, single-session, and never logged.
- Disconnect grace permits a brief reconnect, then terminates the process.
- Navigation never silently leaves an app-owned provider process running.
- External launch remains usable when embedded mode is unavailable.
