# PM-031: Complete Gin API Migration

PM-031 finishes the incremental API transport migration started by PM-030. The goal is to move every `/api/` route from the legacy Go `http.ServeMux` fallback to Gin while preserving public method, path, status, header, body, error, WebSocket, and streaming behavior. The final state has Gin as the only API router and keeps the embedded SPA handler outside Gin.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                 |
|-------------------------------|--------------------------|---------------------------------------------------------------------------------------------|
| [PM-030](../PM-030/README.md) | Direct predecessor       | Added Gin shell, typed errors, first route migration, cache/concurrency pilots, and checks. |
| [PM-004](../PM-004/README.md) | Reliability contract     | Established health, audit, safety checks, and recovery hints that must remain compatible.   |
| [PM-029](../PM-029/README.md) | Verification route input | Added runtime and automation verification routes with bounded concurrency in PM-030.        |
| [PM-003](../PM-003/README.md) | Architecture baseline    | Established behavior-preserving refactors and package-boundary direction.                   |

## Glossary

| Term            | Meaning                                                                                       | Code                                            |
|-----------------|-----------------------------------------------------------------------------------------------|-------------------------------------------------|
| Gin-only API    | Final state where `/api/` routes are registered on Gin without legacy mux fallback.           | `internal/server/api`                           |
| Legacy Fallback | Current PM-030 state where non-migrated routes fall through Gin `NoRoute` to `http.ServeMux`. | `newTransport`                                  |
| Route Family    | Group of routes sharing a domain, risk profile, and test strategy.                            | navigation, system, state, workspace, item, Git |
| Parity Harness  | Tests that lock method, path, status, content type, JSON shape, and important side effects.   | backend tests                                   |
| Cutover Gate    | Required condition before deleting fallback code for a route family.                          | implementation phases                           |
| Streaming Route | Route with Server-Sent Events, WebSocket upgrade, or long-lived response behavior.            | stream-create, embedded AI channel              |

## Migration Order

| Order | Route Family                   | Risk   | Reason                                                                 |
|-------|--------------------------------|--------|------------------------------------------------------------------------|
| 1     | Navigation and system config   | medium | Small controllers, limited domain dependencies, strong request shapes. |
| 2     | State, search, AI settings     | medium | Read-heavy JSON routes with simple payloads.                           |
| 3     | Workspace read routes          | medium | File/tree/search reads need path and query parity.                     |
| 4     | Item read routes               | medium | Item detail, files, diff, Jira reads need richer fixtures.             |
| 5     | Workspace and item writes      | high   | Writes must preserve refresh, stale-hash, scan, and audit behavior.    |
| 6     | Verification, knowledge, Git   | high   | Long-running operations and external commands need failure parity.     |
| 7     | Streaming and WebSocket routes | high   | Upgrade/lifecycle behavior must be tested last.                        |
| 8     | Gin-only cutover and cleanup   | high   | Remove legacy fallback only after every family is covered.             |

## Data Flow

Request -> Gin middleware -> route family handler -> decoder -> domain service or adapter -> typed error/result -> response mapper -> existing frontend contract.

During migration, non-migrated requests continue to pass through the legacy fallback. At cutover, the fallback is removed and missing API routes fail tests instead of silently routing through `ServeMux`.

## Design Decisions

| Decision                                          | Alternatives Considered              | Rationale                                                                 |
|---------------------------------------------------|--------------------------------------|---------------------------------------------------------------------------|
| Route-family migration                            | One-shot rewrite                     | Keeps reviews small and isolates regressions by domain.                   |
| Keep SPA outside Gin                              | Serve all assets through Gin         | The current embedded SPA handler is stable and not part of API migration. |
| Migrate streaming last                            | Convert WebSocket and streams early  | Streaming behavior has distinct lifecycle and upgrade risks.              |
| Preserve envelope shape                           | Standardize all payloads immediately | Avoids frontend regressions and keeps PM-030 compatibility.               |
| Delete fallback only after inventory reaches zero | Keep dual stack indefinitely         | Prevents permanent duplicate transport behavior.                          |
| Use tests as cutover gates                        | Rely on manual route checks          | Full migration needs automated confidence for method/path drift.          |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Infrastructure Design](design/design-02-infrastructure.md)
- [Pipeline Design](design/design-03-pipeline.md)
- [Implementation Plan](implementation-plan.md)
