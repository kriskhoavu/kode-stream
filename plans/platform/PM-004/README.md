# PM-004: Reliability, Safety, And Observability

PM-004 makes Kode Stream safer to use when it reads files, writes files, and runs Git commands. It adds local audit history, stronger operation feedback, workspace health checks, and better stale-file protection. It does not add cloud storage.

## Implementation Status

Implemented. The backend provides audit storage, health and safety services, API integration, and regression tests. The frontend provides health, activity, recovery guidance, and responsive operational panels.

Verification: `go test ./...`, `npm run typecheck`, `npm test -- --run`, and `npm run build`.

## Related Plans

| Ticket                        | Relationship   | Key Context                                                                                 |
|-------------------------------|----------------|---------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature | Created workspace registration, scanning, item index, Kanban, and item workspace            |
| [PM-002](../PM-002/README.md) | Parent feature | Added guarded writes, metadata edits, source settings, and Git operations                   |
| [PM-003](../PM-003/README.md) | Foundation     | Split application services, scanner internals, path guards, frontend state, and API modules |

### What Existing Plans Established

- **Workspace**: a local Git repository registered in Kode Stream.
- **Source**: a configured scan root such as `plans` or `docs`.
- **Item**: a planning folder or docs card.
- **Write Guard**: backend checks that keep writes inside configured sources.
- **Git Operation**: a guarded local Git action.
- **App State Version**: `/api/state` changes when registry or indexed item data changes.
- **Application Service**: backend use case layer under `internal/application`.

## Glossary

| Term             | Meaning                                                                 | Code Target                          |
|------------------|-------------------------------------------------------------------------|--------------------------------------|
| Audit Event      | One local record of a scan, save, metadata edit, Git action, or failure | `AuditEvent`                         |
| Operation Result | Stable response for an action with status, message, and related paths   | `OperationResult`                    |
| Workspace Health | Checks that show whether a workspace is readable, valid, and Git-ready  | `WorkspaceHealth`                    |
| Safety Check     | Preflight validation before a risky write or Git action                 | `SafetyCheck`                        |
| Stale File Guard | Check that blocks writes when the file changed after the editor loaded  | existing hash plus new conflict info |
| Recovery Hint    | Short user-facing next step after a failed operation                    | `recoveryHint`                       |
| Audit Log        | Local append-only JSONL file in the app config directory                | `audit-log.jsonl`                    |

## Data Flow

```text
User runs an operation
  -> frontend sends existing API request
  -> application service runs safety checks
  -> operation runs or fails
  -> audit service appends local event
  -> API returns result plus recovery hint when useful
  -> frontend shows clear status and links to affected files
```

## Design Decisions

| Decision                            | Alternatives Considered         | Rationale                                                             |
|-------------------------------------|---------------------------------|-----------------------------------------------------------------------|
| Store audit log locally             | Cloud logging, no logging       | The app is local and should not send file paths or Git data elsewhere |
| Use JSONL for audit events          | YAML, SQLite                    | JSONL is append-friendly and easy to inspect                          |
| Keep existing write APIs            | Replace all responses           | PM-004 should reduce risk without breaking current workflows          |
| Add health checks as read-only APIs | Run checks only during failures | Users need a safe way to diagnose workspace problems before editing   |
| Show recovery hints                 | Show raw errors only            | Git and filesystem errors need actionable next steps                  |
| Keep hash-based stale checks        | Lock files while editing        | Locks are brittle for local Git workflows; hashes fit current design  |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
