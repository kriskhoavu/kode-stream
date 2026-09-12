---
slug: platform-documentation-audit
title: Documentation Overlap Audit
pageType: REFERENCE
topics: documentation, wiki, architecture, operations, migration
summary: Audit of the repository documentation surface and its intended boundary with the evidence-aware wiki.
sourceRef: |
  docs/README.md | Documentation map
  docs/architecture/README.md | Architecture documents
  docs/domain/README.md | Domain documents
  docs/development.md | Development guide
  docs/specs/requirement.md | Historical requirements
lastVerified: 2026-09-12
---

## Boundary
<!-- chunkId: platform-documentation-audit-boundary -->
<!-- keywords: wiki authority, architecture diagrams, operator procedures, migration -->

[live] The repository currently has 28 Markdown and Mermaid files under `docs/`, with domain behavior overlapping the wiki while architecture diagrams, development setup, and operator procedures have distinct uses. Evidence: `docs/README.md:1`, `docs/domain/README.md:1`.

The proposed boundary is: wiki pages own durable product behavior and current evidence; `docs/architecture/` keeps implementation diagrams; deployment folders keep operator procedures; historical requirements remain labeled historical. No `docs/` file was deleted during this audit.

## Inventory And Disposition
<!-- chunkId: platform-documentation-audit-inventory-disposition -->
<!-- keywords: docs inventory, overlap classification, proposed destination, deletion decision -->

[live] Every file in the current `docs/` tree has been classified below. Evidence: `docs/README.md:1`, `docs/architecture/README.md:1`, `docs/domain/README.md:1`.

| Current path                                                 | Classification                           | Proposed disposition                                                                       |
|--------------------------------------------------------------|------------------------------------------|--------------------------------------------------------------------------------------------|
| `docs/README.md`                                             | Navigation                               | Keep, then point domain behavior to wiki                                                   |
| `docs/development.md`                                        | Developer procedure                      | Keep                                                                                       |
| `docs/specs/requirement.md`                                  | Historical requirements                  | Keep with an explicit historical label                                                     |
| `docs/architecture/README.md`                                | Architecture navigation                  | Keep                                                                                       |
| `docs/architecture/overview.md`                              | System architecture                      | Keep                                                                                       |
| `docs/architecture/deployment-adapters.md`                   | Adapter architecture                     | Keep                                                                                       |
| `docs/architecture/local-mode/README.md`                     | Runtime architecture                     | Keep                                                                                       |
| `docs/architecture/local-mode/local-mode.mmd`                | Diagram source                           | Keep                                                                                       |
| `docs/architecture/local-mode/storage-options.mmd`           | Diagram                                  | Keep                                                                                       |
| `docs/architecture/local-mode/sqlite-storage.mmd`            | Diagram                                  | Keep                                                                                       |
| `docs/architecture/local-mode/datadir-storage.mmd`           | Diagram                                  | Keep                                                                                       |
| `docs/architecture/cloud-mode/README.md`                     | Runtime architecture                     | Keep after correcting capability claims                                                    |
| `docs/architecture/cloud-mode/cloud-mode.mmd`                | Diagram                                  | Keep                                                                                       |
| `docs/architecture/cloud-mode/postgres-storage.mmd`          | Diagram                                  | Keep                                                                                       |
| `docs/architecture/cloud-mode/agentless-remote-snapshot.mmd` | Diagram                                  | Keep                                                                                       |
| `docs/architecture/cloud-mode/agent-backed.mmd`              | Diagram                                  | Keep after correcting command delivery                                                     |
| `docs/architecture/chrome-extension/README.md`               | Runtime architecture                     | Keep                                                                                       |
| `docs/architecture/chrome-extension/chrome-extension.mmd`    | Diagram                                  | Keep                                                                                       |
| `docs/domain/README.md`                                      | Competing domain index                   | Retire after its links move to wiki                                                        |
| `docs/domain/cloud/cloud-agent.md`                           | Product behavior and operator guidance   | Split: behavior to wiki, packaging procedure to deploy docs                                |
| `docs/domain/cloud/cloud-deployment.md`                      | Product behavior and operator guidance   | Split and correct stale OIDC and Agent claims                                              |
| `docs/domain/cloud/cloud-modes.md`                           | Product behavior                         | Replace with [[platform-cloud-runtime-reference]] after link audit                         |
| `docs/domain/cloud/remote-snapshot-operations.md`            | Product behavior and planned operations  | Split current behavior from future operator work                                           |
| `docs/domain/storage/storage-architecture.md`                | Product behavior and operations          | Replace behavior with [[platform-storage-reference]]; retain operator procedures elsewhere |
| `docs/domain/terminal/canvas-sessions.md`                    | Product behavior                         | Merge remaining facts into [[workspace-terminal-canvas-reference]], then retire            |
| `docs/domain/terminal/session-focus.md`                      | Product behavior with a factual conflict | Replace with [[workspace-session-focus]] after inbound-link audit                          |
| `docs/domain/verification/canvas-freshness.md`               | Product behavior                         | Merge into [[workspace-terminal-canvas-reference]], then retire                            |
| `docs/domain/workspace/workspace-import-api.md`              | Unique API reference                     | Migrate to a new wiki reference before retiring                                            |

## Confirmed Conflicts
<!-- chunkId: platform-documentation-audit-confirmed-conflicts -->
<!-- keywords: stale oidc, cloud commands, focus hiding, postgres persistence -->

[live] Four material conflicts require correction before any consolidation: app-owned OIDC is unavailable, Agent commands return 503, Focus hides inactive nodes, and SQL storage includes Canvas and session repositories. Evidence: `internal/system/runtime_config.go:117`, `internal/server/api/cloud_workspace_access.go:48`, `web/src/features/canvas/CanvasBoard.tsx:128`, `internal/storage/storage.go:226`.

## Migration Gate
<!-- chunkId: platform-documentation-audit-migration-gate -->
<!-- keywords: inbound links, unique content, docs deletion, operator review -->

[open] Which proposed moves should be executed as a separate documentation consolidation change? Before deletion, migrate the unique workspace-import contract, preserve operator commands, rewrite every inbound link, and rerun repository and wiki gates.

## Not supported
<!-- chunkId: platform-documentation-audit-not-supported -->
<!-- keywords: automatic deletion, duplicate authority, silent migration -->

[live] This audit does not authorize automatic deletion or leave two unlabeled sources claiming authority for the same product behavior. Evidence: `wiki/conventions.md:1`, `docs/domain/README.md:1`.

## Known conflicts
<!-- chunkId: platform-documentation-audit-known-conflicts -->
<!-- keywords: documentation authority, local agent guide, normative standard -->

[open] The repository agent guide currently describes `docs/domain/` as durable domain knowledge, while the requested standard assigns that role to the wiki. Consolidation needs an explicit repository-level decision after this audit.

## Related pages

See [[platform-cloud-runtime-reference]], [[platform-storage-reference]], [[workspace-session-focus]], and [[knowledge-index]].
