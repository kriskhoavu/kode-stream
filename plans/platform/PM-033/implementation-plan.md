# Implementation Plan: PM-033 - Database Storage For Kode Stream

## Overview

Implement SQL-backed app-owned storage for Kode Stream. Local mode uses SQLite. Cloud mode uses Postgres. Existing
domain workflows keep their API contracts while repository services move behind SQL-capable interfaces. Branch indexes
remain derived data and are re-indexed when branch freshness metadata no longer matches Git state.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `StorageDriver`
- `SQLStore`
- `SQLiteStore`
- `PostgresStore`
- `BranchScanMetadata`
- `BranchIndex`
- `AppOwnedState`
- `RepositoryContent`
- `Reindex`
- `MigrationVersion`

Avoid:

- `database mode`
- `server checkout`
- `cloud clone`
- `terminal storage`
- `source blob`

## Phases Summary

| Phase | Name                                      | Track   | Status  |
|-------|-------------------------------------------|---------|---------|
| B1    | Storage interfaces and composition        | Backend | Done    |
| B2    | SQL migrations and migration runner       | Backend | Done    |
| B3    | SQLite local repositories and file import | Backend | Done    |
| B4    | Branch re-index correctness               | Backend | Done    |
| B5    | Postgres Cloud repositories               | Backend | Done    |
| C1    | Local and Cloud storage operations        | DevOps  | Done    |
| C2    | Pipeline and release gates                | DevOps  | Done    |

## Backend Phases

### Phase B1: Storage Interfaces And Composition

**Deliverables:**

- [x] Add repository interfaces for workspace registry, item index, branch scan metadata, audit, navigation, settings, and import status.
- [x] Update server composition to choose storage driver from runtime configuration.
- [x] Keep YAML-backed implementations available only as import sources or temporary compatibility adapters during transition.
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

### Phase B3: SQLite Local Repositories And File Import

**Deliverables:**

- [x] Implement SQLite repositories for workspaces, item indexes, branch scans, scan warnings, audit, navigation, and settings.
- [x] Add one-time import from existing app-owned YAML and JSONL files under `KODE_STREAM_DATA_DIR`.
- [x] Leave source files untouched and record completed imports in SQL.
- [x] Make SQLite the default Local storage driver.

**Verification:** `rtk go test ./internal/workspace/... ./internal/item/... ./internal/audit/... ./internal/navigation/... ./internal/search/...`

**Commit:** `PM-033: Add SQLite storage and file import`

---

### Phase B4: Branch Re-index Correctness

**Deliverables:**

- [x] Port current branch index behavior to SQL-backed repositories.
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

### Phase C1: Local And Cloud Storage Operations

**Deliverables:**

- [x] Document SQLite file location, backup, restore, and export workflow.
- [x] Document Postgres provisioning, secrets, migration mode, backup, restore, and health checks.
- [x] Update Cloud deployment examples to include Postgres and database environment variables.
- [x] Document that Cloud Agents connect only to Cloud API and never to Postgres.

**Verification:** `rtk rg -n "KODE_STREAM_DATABASE_URL|KODE_STREAM_STORAGE_DRIVER|Postgres|SQLite" docs plans/platform/PM-033`

**Commit:** `PM-033: Document database operations`

---

### Phase C2: Pipeline And Release Gates

**Deliverables:**

- [x] Add SQLite migration and repository tests to normal CI.
- [x] Add optional local Postgres integration checks and required Cloud release Postgres checks.
- [x] Add fixture-based import tests for YAML and JSONL app-owned state.
- [x] Add Cloud release smoke for `/api/health` database readiness.
- [x] Update release checklist with migration version, backup, and branch re-index smoke checks.

**Verification:** `rtk go test ./internal/... && rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-033: Add database pipeline gates`

---

## Post-Implementation Checklist

- [ ] PM-033 docs match implemented package names and environment variables.
- [ ] PM-032 docs still describe Cloud Agent execution without Cloud-hosted clones or commands.
- [ ] Local mode starts with SQLite and existing app-owned files imported.
- [ ] Cloud mode starts only with reachable Postgres.
- [ ] Branch switch and branch load return current branch items once re-index completes.
- [ ] SQL data does not include repository source files, Git credentials, SSH keys, terminal transcripts, or prompts.
- [ ] Full verification passes: `rtk go test ./internal/... && rtk npm run typecheck && rtk npm test -- --run`.
