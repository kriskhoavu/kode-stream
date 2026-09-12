---
slug: workspace-lifecycle-reference
title: Workspace Lifecycle Reference
pageType: REFERENCE
topics: workspace API, import preview, managed clone, source structure, git operations, reliability
summary: Reference for workspace registration modes, import boundaries, source configuration, guarded writes, Git operations, and audit behavior.
sourceRef: |
  PM-001 (plan retired) | Backend workspace contract
  PM-002 (plan retired) | Mutation and Git contract
  PM-004 (plan retired) | Audit, health, and recovery contract
  PM-014 (plan retired) | Source structure backend contract
  PM-016 (plan retired) | Clone lifecycle contract
  PM-024 (plan retired) | Import preview and apply contract
lastVerified: 2026-09-12
---

# Workspace Lifecycle Reference

## Registration Modes
<!-- chunkId: workspace-lifecycle-reference-registration -->
<!-- keywords: existing local, remote clone, managed clone, stream create -->

[live] PM-016 distinguishes existing-local registration from `remote_clone`; streamed creation is exposed separately for progress reporting. Evidence: `internal/common/models/models.go:236`, `internal/server/api/gin_routes.go:195`, `internal/workspace/workspace_lifecycle_service.go:46`.

| Mode           | Input                                          | Ownership boundary                                               |
|----------------|------------------------------------------------|------------------------------------------------------------------|
| Existing local | Absolute local repository path                 | Kode Stream records configuration but does not own the checkout  |
| Remote clone   | Credential-free Git URL and managed clone root | Kode Stream owns only a verified managed clone path              |
| Import         | Preview source plus selected candidate IDs     | Each accepted candidate is validated through normal registration |

Remote URLs containing credentials are rejected from persisted metadata. Managed clone deletion requires the recorded
root and verification markers to agree; a local workspace is never treated as disposable clone data.

## Import Contract
<!-- chunkId: workspace-lifecycle-reference-import -->
<!-- keywords: preview import, duplicate, conflict, candidate -->

[live] PM-024 import is a preview-then-apply operation, with preview implemented independently from mutation. Evidence: `internal/workspace/workspace_import_service.go:26`, `internal/workspace/workspace_import_service.go:95`.

The preview normalizes source records, reports records already represented locally, and marks unusable candidates.
Applying an import accepts explicit selections; it does not silently replace an existing workspace.

## Source Structure Contract
<!-- chunkId: workspace-lifecycle-reference-source-structure -->
<!-- keywords: source structure endpoint, cards, fields, path pattern -->

[live] PM-014 source settings contain ordered cards and field mappings, and the workspace API supports read, replace, and reset operations. Evidence: `internal/common/models/models.go:470`, `internal/server/api/gin_routes.go:128`, `internal/server/api/gin_routes.go:157`.

Generated proposals remain previews until saved. Item creation uses the accepted path pattern and field mapping, keeping
discovery and authoring aligned.

## Mutation And Recovery Contract
<!-- chunkId: workspace-lifecycle-reference-mutations -->
<!-- keywords: save file, revert file, commit, synchronization, recovery hint -->

[live] PM-002 and PM-004 expose item save and revert plus guarded fetch, pull, push, commit, and branch-switch services; Git results can require a refresh or carry a recovery hint. Evidence: `internal/item/item_service.go:166`, `internal/item/item_service.go:234`, `internal/git/git_service.go:222`, `internal/git/git_service.go:243`, `internal/git/git_service.go:271`, `internal/git/git_service.go:292`, `internal/git/git_service.go:341`.

| Outcome   | Meaning                                            |
|-----------|----------------------------------------------------|
| `success` | The requested action completed                     |
| `blocked` | A safety precondition prevented the action         |
| `failed`  | Execution failed and may include recovery guidance |

Audit records retain workspace, item, path, operation, duration, status, message, error, and recovery context. An audit
append failure is reported independently so a completed mutation is not falsely presented as undone.

## Health Boundary
<!-- chunkId: workspace-lifecycle-reference-health -->
<!-- keywords: application health, database health, runtime health -->

[live] PM-004 application health is served by the system API, while workspace runtime health is evaluated by the runtime service and is not the same signal. Evidence: `internal/system/system_controller.go:16`, `internal/storage/storage.go:291`, `internal/runtime/runtime_service.go:351`.

## Not supported
<!-- chunkId: workspace-lifecycle-reference-not-supported -->
<!-- keywords: credential URL, overwrite, unguarded mutation -->

[live] PM-016 does not persist credential-bearing clone URLs, and PM-024 import does not overwrite an existing registration implicitly. Evidence: `internal/agent/workspace.go:69`, `internal/workspace/workspace_import_service.go:95`.

## Known conflicts
<!-- chunkId: workspace-lifecycle-reference-known-conflicts -->
<!-- keywords: old endpoint, old UI, plan drift -->

[legacy] The plans name earlier controller packages and screens. Current endpoints are owned by the Gin route manifest and current services, so old route inventories are historical rather than API authority.

## Related pages

See [[workspace-lifecycle]], [[workspace-branch-context-reference]], [[platform-api-transport-reference]], and [[platform-storage-reference]].
