# PM-020: Embedded AI Terminal

PM-020 builds on PM-018 by adding managed embedded PTYs as an alternative AI-session surface. It owns terminal process lifecycle, loopback transport, reconnect behavior, and an app-level multi-session terminal dock while preserving external launch as a fallback.

## Related Plans

| Item                                  | Relationship           | Key Context                                                     |
|---------------------------------------|------------------------|-----------------------------------------------------------------|
| [PM-018](../PM-018/README.md)         | Required AI foundation | Reuse providers, settings, intents, eligibility, and manifests  |
| [PM-016](../PM-016/README.md)         | Local operation safety | Reuse audit and explicit-confirmation patterns                  |
| [PM-017](../PM-017/README.md)         | Platform rollout       | Keep external terminal launch available as a compatibility path |
| [PM-021](../pending-PM-021/README.md) | Pending Jira scope     | Owns deferred Jira field, transition, and attachment mutations  |

## Scope

### Goal

Let users interact with a supported AI CLI inside Kode Stream through a bounded, workspace-contained terminal session.

### Non-Goals

- No unattended AI agents or background implementation queue.
- No remote PTY access outside the loopback Kode Stream origin.
- No Jira writes; PM-021 owns Jira editing.
- No replacement of the existing external terminal launch path.

## Glossary

| Term             | Meaning                                                                     |
|------------------|-----------------------------------------------------------------------------|
| Embedded Session | AI provider process attached to a Kode Stream-owned pseudo-terminal        |
| Session Channel  | Loopback WebSocket carrying terminal input, output, resize, and lifecycle   |
| Session Lease    | Bounded ownership period renewed while the browser remains connected        |
| Terminal Dock    | App-level owner for switching, minimizing, maximizing, and closing sessions |

## Data Flow

```text
PM-018 launch request -> embedded mode -> validated PTY process
  <-> loopback WebSocket <-> app-level terminal dock
  -> normal, maximized, or collapsed restore-chip presentation
  -> exit, confirmed close, lease expiry, or shutdown cleanup
```

## Design Decisions

| Decision                            | Alternatives Considered         | Rationale                                                   |
|-------------------------------------|---------------------------------|-------------------------------------------------------------|
| Reuse PM-018 launch contracts       | Separate embedded configuration | Keeps provider behavior consistent across terminal surfaces |
| Keep external launch available      | Replace with embedded terminal  | Preserves a stable fallback and native terminal preference  |
| Opaque sessions with bounded leases | Browser-owned PID               | Prevents process identifier exposure and orphaned processes |
| App-level multi-session dock        | Item-owned terminal modal       | Keeps sessions connected across workspace navigation        |
| Collapsed restore chip              | Small live terminal window      | Fully frees the plan UI while retaining session transports  |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
