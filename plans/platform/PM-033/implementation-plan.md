# Implementation Plan: PM-033 - Configurable App-State Storage For Kode Stream

## Overview

Implement configurable app-owned storage for Kode Stream. Local mode supports `database` and `datadir`. Local
`database` uses SQLite. Local `datadir` uses YAML/JSONL files. Cloud mode uses Postgres. Existing domain workflows keep
their API contracts while repository services run behind storage-capable interfaces. Branch indexes remain derived data
and are re-indexed when branch freshness metadata no longer matches Git state.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `StorageDriver`
- `StorageOption`
- `SQLStore`
- `SQLiteStore`
- `PostgresStore`
- `DataDirStore`
- `StorageSync`
- `BranchScanMetadata`
- `BranchIndex`
- `AppOwnedState`
- `RepositoryContent`
- `Reindex`
- `MigrationVersion`

Avoid:

- `server checkout`
- `cloud clone`
- `terminal storage`
- `source blob`

## Phases Summary

| Phase | Name                                      | Track    | Status  |
|-------|-------------------------------------------|----------|---------|
| B1    | Storage interfaces and composition        | Backend  | Done    |
| B2    | SQL migrations and migration runner       | Backend  | Done    |
| B3    | SQLite and datadir local repositories     | Backend  | Done    |
| B4    | Branch re-index correctness               | Backend  | Done    |
| B5    | Postgres Cloud repositories               | Backend  | Done    |
| B6    | Storage option and manual sync API        | Backend  | Done    |
| F1    | Storage Settings option and sync controls | Frontend | Done    |
| C1    | Local and Cloud storage operations        | DevOps   | Planned |
| C2    | Pipeline and release gates                | DevOps   | Planned |

## Backend Phases

### Phase B1: Storage Interfaces And Composition

**Deliverables:**

- [x] Add repository interfaces for workspace registry, item index, branch scan metadata, audit, navigation, settings, and storage status.
- [x] Update server composition to choose storage repositories from runtime configuration.
- [x] Keep YAML-backed implementations available as the Local `datadir` backend.
- [x] Preserve existing API route contracts and response payloads.

**Verification:** `rtk go test ./internal/server/... ./internal/workspace/... ./internal/item/... ./internal/workstream/...`

**Commit:** `PM-033: Add storage interfaces and runtime composition`

---

### Phase B2: SQL Migrations And Migration Runner

**Deliverables:**

- [x] Add embedded SQL migrations for SQLite and Postgres.
- [x] Add migration runner with schema version tracking.
- [x] Add startup validation for storage driver, SQLite path, Postgres URL, and migration mode.
- [x] Add health integration for database connectivity and migration version.

**Verification:** `rtk go test ./internal/storage/... ./internal/server/...`

**Commit:** `PM-033: Add SQL migrations and database health`

---

### Phase B3: SQLite And Datadir Local Repositories

**Deliverables:**

- [x] Implement SQLite repositories for workspaces, item indexes, branch scans, scan warnings, audit, navigation, and settings.
- [x] Keep data-dir repositories for Local `datadir` storage.
- [x] Make SQLite the Local `database` backend.
- [x] Preserve data-dir file behavior when `datadir` is selected.

**Verification:** `rtk go test ./internal/workspace/... ./internal/item/... ./internal/audit/... ./internal/navigation/... ./internal/search/...`

**Commit:** `PM-033: Add SQLite and datadir local storage`

---

### Phase B4: Branch Re-index Correctness

**Deliverables:**

- [x] Port current branch index behavior to configured storage repositories.
- [x] Re-index a selected branch when commit, source configuration hash, or working-tree hash does not match SQL metadata.
- [x] Replace item rows, warnings, and branch scan metadata for one workspace branch in a single transaction.
- [x] Add tests where the same item identifier has different content on different branches.
- [x] Ensure Git branch switch refreshes the active branch index, then returns board/search data.

**Verification:** `rtk go test ./internal/workstream/... ./internal/git/... ./internal/item/...`

**Commit:** `PM-033: Add SQL branch reindexing`

---

### Phase B5: Postgres Cloud Repositories

**Deliverables:**

- [x] Implement Postgres repositories using the same storage interfaces.
- [x] Fail Cloud startup when Postgres config is missing, unreachable, or not migrated.
- [x] Persist Cloud user, agent, workspace, branch index, audit, and settings metadata in Postgres.
- [x] Accept Cloud Agent scan metadata and derived item rows without storing repository source content.
- [x] Add Postgres integration tests gated by `KODE_STREAM_DATABASE_URL`.

**Verification:** `KODE_STREAM_DATABASE_URL=... rtk go test ./internal/storage/... ./internal/server/...`

**Commit:** `PM-033: Add Postgres storage for Cloud mode`

---

## DevOps Phases

### Phase B6: Storage Option And Manual Sync API

**Deliverables:**

- [x] Introduce provider-style storage composition with `StorageProvider`, `RepositoryBundle`, `StorageStatusService`, and `StorageSyncService` boundaries.
- [x] Add `KODE_STREAM_STORAGE_OPTION=database|datadir` and bootstrap `storageOption` support.
- [x] Resolve Local `database` to SQLite, Local `datadir` to file repositories, and Cloud `database` to Postgres.
- [x] Reject Cloud `datadir` startup with a clear validation error.
- [x] Add storage status API with effective option, environment lock state, paths, and database health when applicable.
- [x] Add confirmed manual sync API for `datadir_to_database` and `database_to_datadir`.
- [x] Write timestamped target backups before sync replacement.

**Verification:** `rtk go test ./internal/storage/... ./internal/system/... ./internal/server/...`

**Commit:** `PM-033: Add storage option and sync API`

---

### Phase F1: Storage Settings Option And Sync Controls

**Deliverables:**

- [x] Show effective storage option, data directory, database path, and restart-required messaging in Settings.
- [x] Let local admins save `database` or `datadir` when storage config is not environment-locked.
- [x] Show local-only manual sync actions in both directions with explicit confirmation.
- [x] Display sync summary, warnings, skipped stores, and backup path.
- [x] Hide or disable unsupported sync actions in Cloud mode.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run StorageSettings`

**Commit:** `PM-033: Add storage settings sync controls`

---

### Phase C1: Local And Cloud Storage Operations

**Deliverables:**

- [ ] Document Local `database` and `datadir` startup configuration.
- [ ] Document manual sync, target backups, and restore workflow.
- [ ] Document Postgres provisioning, secrets, migration mode, backup, restore, and health checks.
- [ ] Update `run.sh` to accept or pass through `KODE_STREAM_STORAGE_OPTION=database|datadir`, print the effective test mode, and support quick local smoke runs for both options.
- [ ] Update `run-docker-cloud.sh` to set `KODE_STREAM_STORAGE_OPTION=database` for Cloud smoke runs and fail early if a datadir option is supplied.
- [ ] Add script help text and examples for local database, local datadir, and Cloud database test runs.
- [ ] Update `docs/storage/storage-architecture.md` so data-dir is documented as a supported Local storage option, not deprecated.
- [ ] Update `docs/storage/storage-architecture-diagram.mmd` and regenerated diagram assets to show Local `database`, Local `datadir`, Cloud Postgres, and manual sync.
- [ ] Update root `README.md` and `ARCHITECTURE.md` to describe `KODE_STREAM_STORAGE_OPTION`, Local backend choices, and Cloud database-only behavior.
- [ ] Update Cloud deployment examples to include storage option, Postgres, and database environment variables.
- [ ] Document that Cloud Agents connect only to Cloud API and never to Postgres.

**Verification:** `KODE_STREAM_STORAGE_OPTION=database rtk ./run.sh restart && KODE_STREAM_STORAGE_OPTION=datadir rtk ./run.sh restart && KODE_STREAM_STORAGE_OPTION=database rtk ./run-docker-cloud.sh`

**Commit:** `PM-033: Add configurable storage run scripts and docs`

---

### Phase C2: Pipeline And Release Gates

**Deliverables:**

- [x] Add SQLite migration and repository tests to normal CI.
- [ ] Add Local datadir repository and storage-option resolution tests to normal CI.
- [ ] Add script smoke coverage or documented manual smoke steps for `run.sh` with both Local storage options.
- [x] Add optional local Postgres integration checks and required Cloud release Postgres checks.
- [ ] Add fixture-based manual sync tests for YAML, JSONL, and SQLite app-owned state.
- [x] Add Cloud release smoke for `/api/health` database readiness.
- [ ] Update release checklist with storage option, sync backup, migration version, and branch re-index smoke checks.

**Verification:** `rtk go test ./internal/... && rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-033: Add database pipeline gates`

---

## Post-Implementation Checklist

- [ ] PM-033 docs match implemented package names and environment variables.
- [ ] PM-032 docs still describe Cloud Agent execution without Cloud-hosted clones or commands.
- [ ] Local mode starts with `database` and `datadir` options.
- [ ] Manual sync works in both directions and creates target backups.
- [ ] Cloud mode starts only with reachable Postgres.
- [ ] Branch switch and branch load return current branch items once re-index completes.
- [ ] App-state storage does not include repository source files, Git credentials, SSH keys, terminal transcripts, or prompts.
- [ ] Full verification passes: `rtk go test ./internal/... && rtk npm run typecheck && rtk npm test -- --run`.
