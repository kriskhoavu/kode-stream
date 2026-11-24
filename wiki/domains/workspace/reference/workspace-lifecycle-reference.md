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
lastVerified: 2026-11-24
---

# Workspace Lifecycle Reference

## Registration Modes
<!-- chunkId: workspace-lifecycle-reference-registration -->
<!-- keywords: existing local, remote clone, stream create, ownership -->

[live] Existing-local registration and `remote_clone` are separate modes. Streamed creation reports clone progress. Evidence: `internal/common/models/workspace.go:130`, `internal/workspace/workspace_lifecycle_service.go:46`.

| Mode           | Input                                          | Ownership boundary                                               |
|----------------|------------------------------------------------|------------------------------------------------------------------|
| Existing local | Absolute local repository path                 | Kode Stream records configuration but does not own the checkout  |
| Remote clone   | Credential-free Git URL and managed clone root | Kode Stream owns only a verified managed clone path              |
| Import         | Registry file plus selected candidate keys     | Each candidate passes normal registration                        |

A URL with credentials is never persisted. Deleting a managed clone requires the recorded root and its markers to agree. A local workspace is never treated as disposable clone data.

## Import Contract
<!-- chunkId: workspace-lifecycle-reference-import -->
<!-- keywords: preview, candidate key, registry yaml, scan failed -->

[live] Import reads an absolute local path to a current-schema `.yaml` or `.yml` registry file. Preview never writes. Evidence: `internal/workspace/workspace_import_service.go:225`, `internal/server/api/gin_routes.go:154`.

| Method | Path                             | Purpose                                      |
|--------|----------------------------------|----------------------------------------------|
| POST   | `/api/system/select-file`        | Select a YAML file in the native dialog      |
| GET    | `/api/system/config-paths`       | Return the effective `registryFile` path     |
| POST   | `/api/workspaces/import-preview` | Parse and validate candidates (`sourcePath`) |
| POST   | `/api/workspaces/import`         | Register `candidateKeys` from `sourcePath`   |

- Preview returns the canonical source path, the destination path, a source fingerprint, ordered candidates and counts of valid, invalid, duplicate and already-registered entries.
- The whole file is rejected for: a relative path, a non-YAML or non-regular file, more than 1 MiB, more than 500 entries, unknown fields, YAML aliases, malformed YAML or several documents. A bad candidate does not reject the file.
- Import rereads and revalidates the file. It never takes workspace configuration from the browser.
- Valid candidates are written in one atomic registry replacement (mode `0600`), with new IDs, mode `existing_workspace` and `clonePathManaged: false`.

| Status        | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `indexed`     | Registered and scanned                                         |
| `scan_failed` | Registered. Run the normal Scan action again                   |
| `skipped`     | Candidate changed, became invalid or duplicate, or exists      |
| `failed`      | The registry write failed and nothing was registered           |

A scan failure does not roll back a registration or stop the other scans. Messages exclude credentials and raw local errors.

## Source Structure Contract
<!-- chunkId: workspace-lifecycle-reference-source-structure -->
<!-- keywords: cards, field mapping, path pattern, reset -->

[live] Source settings hold ordered cards and field mappings. The workspace API reads, replaces and resets them. Evidence: `internal/common/models/source_structure.go:3`.

A proposal stays a preview until saved. Item creation uses the saved path pattern and fields, so discovery and authoring agree.

## Mutation And Recovery Contract
<!-- chunkId: workspace-lifecycle-reference-mutations -->
<!-- keywords: save, revert, commit, recovery hint, audit -->

[live] Item save and revert, and guarded fetch, pull, push, commit and branch switch, return one of three outcomes. A Git result may require a refresh or carry a recovery hint. Evidence: `internal/item/item_service.go:162`, `internal/git/git_service.go:219`.

| Outcome   | Meaning                                            |
|-----------|----------------------------------------------------|
| `success` | The action completed                               |
| `blocked` | A safety precondition stopped the action           |
| `failed`  | Execution failed, possibly with recovery guidance  |

Audit records keep workspace, item, path, operation, duration, status, message, error and recovery context. A failed audit append is reported on its own, so a completed change never looks undone.

## Health Boundary
<!-- chunkId: workspace-lifecycle-reference-health -->
<!-- keywords: application health, database health, runtime health -->

[live] The system API serves application health. The runtime service evaluates workspace runtime health. They are different signals. Evidence: `internal/system/system_controller.go:16`, `internal/runtime/runtime_service.go:348`.

## Not supported
<!-- chunkId: workspace-lifecycle-reference-not-supported -->
<!-- keywords: credential URL, overwrite, unguarded mutation -->

[live] Credential-bearing clone URLs are never persisted. Import never overwrites an existing registration. Evidence: `internal/agent/workspace.go:67`, `internal/workspace/workspace_import_service.go:82`.

## Known conflicts
<!-- chunkId: workspace-lifecycle-reference-known-conflicts -->
<!-- keywords: route inventory, controller packages, api authority -->

- "Retired plans list the API routes." The Gin route manifest in `internal/server/api/gin_routes.go` is the authority.

## Related pages

See [[workspace-lifecycle]], [[platform-api-transport-reference]] and [[platform-storage-reference]].
