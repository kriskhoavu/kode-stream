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
lastVerified: 2026-11-24
---

## Backend Selection
<!-- chunkId: platform-storage-reference-backend-selection -->
<!-- keywords: storage option, driver override, sqlite, postgres -->

[live] App-owned state lives outside managed repositories. `KODE_STREAM_STORAGE_OPTION` picks the backend at startup. Local defaults to `datadir`. Cloud accepts only `database` on Postgres and fails startup otherwise. Evidence: `internal/storage/storage.go:126`, `internal/storage/storage.go:190`.

| Runtime | Option     | Driver     | Location                                  |
|---------|------------|------------|-------------------------------------------|
| Local   | `datadir`  | `file`     | YAML and JSONL files in the data dir      |
| Local   | `database` | `sqlite`   | `<data dir>/kode-stream.db`               |
| Cloud   | `database` | `postgres` | `KODE_STREAM_DATABASE_URL`                |

| Variable                          | Local                                         | Cloud                                                  |
|-----------------------------------|-----------------------------------------------|--------------------------------------------------------|
| `KODE_STREAM_STORAGE_OPTION`      | `datadir` (default) or `database`             | Must be `database`                                     |
| `KODE_STREAM_STORAGE_DRIVER`      | Optional override that locks the option       | `postgres`                                             |
| `KODE_STREAM_SQLITE_PATH`         | Optional SQLite file path                     | Unused                                                 |
| `KODE_STREAM_DATABASE_URL`        | Unused                                        | Required, secret-managed                               |
| `KODE_STREAM_MIGRATIONS`          | `auto` (default) or `manual`                  | `auto` or operator-run `manual`                        |
| `KODE_STREAM_DATA_DIR`            | App-state files, SQLite and backups           | Diagnostics and rollback files                         |

A set driver overrides the option. Settings then shows the option but cannot change it. A Local Postgres driver works when a database URL is given, but Settings offers only `datadir` and SQLite.

The data dir resolves in this order: `KODE_STREAM_DATA_DIR`, then `bootstrap.yaml` in the default OS directory, then the default OS directory. Changing `dataDir` or `storageOption` needs a restart.

## Repository Composition
<!-- chunkId: platform-storage-reference-repository-composition -->
<!-- keywords: repository bundle, canvas, session records, knowledge index -->

[live] With SQL storage, the workspace, item, audit, navigation, AI settings, Canvas and session-record repositories all use the SQL bundle. The knowledge index stays file-backed. Evidence: `internal/storage/storage.go:220`, `internal/storage/storage.go:249`.

| Data                          | `datadir` file            | SQLite / Postgres |
|-------------------------------|---------------------------|-------------------|
| Workspace registry            | `workspaces.yaml`         | SQL               |
| Item index and branch scans   | `item-index.yaml`         | SQL               |
| Audit events                  | `audit-log.jsonl`         | SQL               |
| Saved filters, recent items   | `saved-filters.yaml`, `recent-items.yaml` | SQL |
| AI settings                   | `ai-settings.yaml`        | SQL               |
| Canvas layouts and placements | `canvases.yaml`           | SQL               |
| AI session records            | `ai-session-records.yaml` | SQL               |
| Knowledge index               | `knowledge-index.yaml`    | `knowledge-index.yaml` |
| Harness                       | Unsupported               | SQL               |

Evidence: `internal/system/system_config_repository.go:52`.

Repository source and Git history never enter app-owned storage. Canvas storage holds layout identity, viewport, coordinates, collapsed state, references and revisions only. `/api/health` reports database health and `migrationVersion`. `/api/storage/status` reports the effective option, the lock, the data dir and database health.

## Manual Synchronization
<!-- chunkId: platform-storage-reference-manual-synchronization -->
<!-- keywords: local sync, confirmation, inactive target, backup -->

[live] Settings can copy `datadir_to_database` or `database_to_datadir`. Sync is Local-only and confirmed. It rejects concurrent runs and Local Postgres, and writes only from the active backend to the inactive one. Evidence: `internal/storage/sync.go:19`, `internal/storage/sync.go:97`.

The service stages a full snapshot, backs up the target under `<data dir>/backups/storage-sync/`, then replaces it. Canvas and session records take part. The knowledge store is skipped. After a sync, restart with the intended option. Routes: `/api/storage/status`, `/api/storage/option`, `/api/storage/sync`.

## Backup And Restore
<!-- chunkId: platform-storage-reference-backup-restore -->
<!-- keywords: sqlite dump, pg_dump, manual migrations, restore check -->

- SQLite: stop Kode Stream and copy `kode-stream.db`, or run `sqlite3 kode-stream.db .dump > kode-stream.sql`.
- `datadir`: stop Kode Stream and copy all YAML and JSONL files together.
- Postgres: use a managed snapshot or `pg_dump`.
- Restore: stop Kode Stream, replace the files or database, start with the matching option, then check `/api/health` and the workspace list.
- Manual migrations: run the binary once with `KODE_STREAM_MIGRATIONS=auto` against the target database, then deploy with `KODE_STREAM_MIGRATIONS=manual`.
- Local checks: `KODE_STREAM_STORAGE_OPTION=database ./run.sh restart`, and `./run.sh smoke-storage` for both options.

## Not supported
<!-- chunkId: platform-storage-reference-not-supported -->
<!-- keywords: dual write, cloud sync, cloud datadir -->

[live] Dual writes, Cloud sync, Local Postgres sync, knowledge-index migration and Cloud `datadir`. Evidence: `internal/storage/sync.go:99`, `internal/storage/sync.go:102`.

## Known conflicts
<!-- chunkId: platform-storage-reference-known-conflicts -->
<!-- keywords: postgres canvas, deferred claim, harness datadir -->

- "Postgres Canvas and session persistence is deferred." Both use the SQL repositories. Evidence: `internal/storage/storage.go:271`.

## Related pages

See [[platform-cloud-runtime-reference]] and [[workspace-terminal-canvas-reference]].
