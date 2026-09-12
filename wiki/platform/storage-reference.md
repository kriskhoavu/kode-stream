---
slug: platform-storage-reference
title: Storage Reference
pageType: REFERENCE
topics: storage, sqlite, postgres, datadir, synchronization
summary: Reference for app-owned storage selection, repository composition, and local manual synchronization.
sourceRef: |
  PM-033 (plan retired) | Storage options
  PM-033 (plan retired) | Repository composition
  PM-033 (plan retired) | Runtime configuration
  PM-033 (plan retired) | Verification
  PM-033 (plan retired) | Delivery
lastVerified: 2026-09-12
---

## Backend Selection
<!-- chunkId: platform-storage-reference-backend-selection -->
<!-- keywords: storage option, driver override, sqlite, postgres -->

[live] Local mode defaults to file-backed `datadir`, Cloud defaults to Postgres-backed `database`, and an explicit driver overrides the effective option and locks the setting. Evidence: `internal/storage/storage.go:128`, `internal/storage/storage.go:160`.

Low-level Local Postgres configuration is possible when a database URL is supplied, even though the user-facing Local choices are datadir and SQLite. Cloud rejects every driver except Postgres.

## Repository Composition
<!-- chunkId: platform-storage-reference-repository-composition -->
<!-- keywords: repository bundle, canvas, session records, knowledge index -->

[live] Workspace, item, audit, navigation, AI settings, Canvas, and session-record repositories switch to the selected SQL bundle; the knowledge index remains file-backed. Evidence: `internal/storage/storage.go:209`, `internal/storage/storage.go:236`.

Repository-owned source and Git history stay outside app-owned storage. SQL migrations can run automatically or be left to an operator.

## Manual Synchronization
<!-- chunkId: platform-storage-reference-manual-synchronization -->
<!-- keywords: local sync, confirmation, inactive target, backup, knowledge skipped -->

[live] Manual synchronization is Local-only, requires confirmation, rejects concurrent runs and Local Postgres, and can write only from the active backend to the inactive backend. Evidence: `internal/storage/sync.go:98`, `internal/storage/sync.go:110`.

The service stages a complete snapshot, backs up the target, then publishes it. Canvas and session records participate; the knowledge store is explicitly skipped. The routes are `/api/storage/status`, `/api/storage/option`, and `/api/storage/sync`.

## Not supported
<!-- chunkId: platform-storage-reference-not-supported -->
<!-- keywords: dual write, cloud sync, local postgres sync -->

[live] Dual writes, Cloud manual sync, Local Postgres manual sync, and knowledge-index migration are not supported. Evidence: `internal/storage/sync.go:110`, `internal/storage/sync.go:126`.

## Known conflicts
<!-- chunkId: platform-storage-reference-known-conflicts -->
<!-- keywords: stale documentation, postgres canvas, deferred claim -->

[live] `docs/domain/storage/storage-architecture.md` still marks Postgres Canvas and session persistence as deferred, although the SQL repository bundle now includes both. Evidence: `internal/storage/storage.go:226`, `internal/storage/storage_test.go:599`.

## Related pages

See [[platform-cloud-runtime-reference]], [[workspace-terminal-canvas-reference]], and [[platform-documentation-audit]].
