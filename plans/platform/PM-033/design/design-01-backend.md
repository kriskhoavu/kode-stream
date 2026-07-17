# Backend Design: Configurable App-State Storage For Kode Stream

## Overview

The backend composes app-owned state repositories from the configured storage option. Local mode supports `database`
and `datadir`. Cloud mode uses `database` with Postgres. The domain model keeps Git repositories as the source of truth
for planning content and uses app-state storage only for metadata and derived indexes.

## Storage Boundary

| Domain Area | Repository Responsibility                                              |
|-------------|------------------------------------------------------------------------|
| Workspace   | Persist workspace metadata, registration mode, sources, and runtime.   |
| Item index  | Persist branch-scoped item summaries, details, warnings, and state.    |
| Workstream  | Query and refresh branch indexes through repository interfaces.        |
| Audit       | Append and query operation events.                                     |
| Navigation  | Persist saved filters and recent items.                                |
| AI settings | Persist provider settings and launch preferences.                      |
| Knowledge   | Persist derived knowledge index metadata without source file content.  |
| Storage     | Report active backend and run explicit local sync between store types. |

## Runtime Configuration

| Setting                      | Accepted Values              | Required In | Behavior                                                     |
|------------------------------|------------------------------|-------------|--------------------------------------------------------------|
| `KODE_STREAM_STORAGE_OPTION` | `database`, `datadir`        | optional    | Admin-facing backend choice.                                 |
| `KODE_STREAM_STORAGE_DRIVER` | `file`, `sqlite`, `postgres` | optional    | Low-level compatibility override for repository composition. |
| `KODE_STREAM_SQLITE_PATH`    | filesystem path              | optional    | Overrides Local SQLite path.                                 |
| `KODE_STREAM_DATABASE_URL`   | Postgres URL                 | Cloud       | Connects hosted API to Postgres.                             |
| `KODE_STREAM_MIGRATIONS`     | `auto`, `manual`             | database    | Runs embedded migrations or requires operator action.        |

Local mode defaults to `datadir`. Local `database` uses SQLite, and Local `datadir` uses YAML/JSONL repositories.
Cloud mode requires `database` with Postgres and fails startup when the database URL is missing or unreachable.

## Data Model

### Workspace

| Field                  | Type   | Purpose                                                                   |
|------------------------|--------|---------------------------------------------------------------------------|
| `id`                   | string | Stable workspace identifier.                                              |
| `name`                 | string | User-facing workspace name.                                               |
| `path_label`           | string | Local path or redacted Cloud Agent label.                                 |
| `baseline_branch`      | string | Default branch for workspace views.                                       |
| `registration_mode`    | string | `local_path`, `remote_clone`, `existing_workspace`, or agent-backed mode. |
| `remote_url`           | string | Git remote metadata when available.                                       |
| `clone_path_managed`   | bool   | Whether Kode Stream owns a local clone directory.                         |
| `last_selected_branch` | string | Last branch selected by the user.                                         |
| `sources`              | array  | Workspace-relative source directories.                                    |
| `created_at`           | time   | Registration time.                                                        |
| `last_scanned_at`      | time   | Latest successful scan time.                                              |

### Branch Scan

| Field                       | Type   | Purpose                                                     |
|-----------------------------|--------|-------------------------------------------------------------|
| `workspace_id`              | string | Workspace owner.                                            |
| `branch`                    | string | Selected branch name.                                       |
| `branch_ref`                | string | Resolved Git ref.                                           |
| `commit`                    | string | Resolved commit SHA.                                        |
| `source_mode`               | string | `working_tree` or `snapshot`.                               |
| `editable`                  | bool   | Whether item files can be edited from this branch context.  |
| `source_configuration_hash` | string | Hash of workspace source settings relevant to scanning.     |
| `working_tree_hash`         | string | Hash of editable working-tree source file timestamps/sizes. |
| `scanned_at`                | time   | Scan completion time.                                       |

### Indexed Item

| Field           | Type   | Purpose                                                  |
|-----------------|--------|----------------------------------------------------------|
| `id`            | string | Stable item identifier used by existing APIs.            |
| `workspace_id`  | string | Workspace owner.                                         |
| `branch`        | string | Branch that produced this indexed row.                   |
| `scope`         | string | Source scope.                                            |
| `identifier`    | string | Item identifier.                                         |
| `title`         | string | Board/search title.                                      |
| `status`        | string | `unsorted`, `draft`, `in_progress`, `review`, or `done`. |
| `item_path`     | string | Workspace-relative item root path.                       |
| `source_mode`   | string | `working_tree` or `snapshot`.                            |
| `editable`      | bool   | Whether file actions are allowed.                        |
| `metadata_json` | JSON   | Parsed plan metadata and document references.            |
| `updated_at`    | time   | Latest item update timestamp.                            |

## Database Schema

```sql
CREATE TABLE workspaces (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  path_label TEXT NOT NULL,
  baseline_branch TEXT NOT NULL,
  registration_mode TEXT NOT NULL,
  remote_url TEXT,
  clone_path_managed BOOLEAN NOT NULL DEFAULT FALSE,
  last_selected_branch TEXT,
  sources_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  last_scanned_at TIMESTAMP
);

CREATE TABLE branch_scans (
  workspace_id TEXT NOT NULL,
  branch TEXT NOT NULL,
  branch_ref TEXT,
  commit_sha TEXT,
  source_mode TEXT NOT NULL,
  editable BOOLEAN NOT NULL,
  source_configuration_hash TEXT,
  working_tree_hash TEXT,
  scanned_at TIMESTAMP NOT NULL,
  PRIMARY KEY (workspace_id, branch)
);

CREATE TABLE indexed_items (
  id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  branch TEXT NOT NULL,
  scope TEXT NOT NULL,
  identifier TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  item_path TEXT NOT NULL,
  source_mode TEXT NOT NULL,
  editable BOOLEAN NOT NULL,
  metadata_json TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (workspace_id, branch, id)
);

CREATE INDEX indexed_items_workspace_branch_status
  ON indexed_items (workspace_id, branch, status);
```

The database backend may add tables for scan warnings, audit events, saved filters, recent items, settings, Cloud users,
Cloud agents, and Cloud workspaces using the same migration mechanism.

## Repository Interfaces

| Interface Role         | Required Behavior                                                                    |
|------------------------|--------------------------------------------------------------------------------------|
| Workspace repository   | List, get, create, batch create, update, delete, touch scanned, set selected branch. |
| Item index repository  | Query items, get item detail, replace workspace index, replace branch index, delete. |
| Branch scan repository | Read freshness metadata and atomically update metadata with indexed rows.            |
| Audit repository       | Append events and filter by workspace.                                               |
| Navigation repository  | Save filters and recent items without browser-only persistence.                      |
| Migration repository   | Track schema version and storage maintenance status.                                 |
| Storage sync service   | Copy app-owned state between local database and data-dir stores on explicit request. |

Domain services depend on these interfaces. Storage adapters own file layout, database details, transactions, and
dialect differences.

## Storage Provider Design

Storage composition uses provider-style boundaries so each backend can evolve without changing domain services.

| Type Or Interface      | Responsibility                                                                 |
|------------------------|--------------------------------------------------------------------------------|
| `StorageOption`        | Admin-facing selection: `database` or `datadir`.                               |
| `StorageDriver`        | Low-level implementation: `file`, `sqlite`, or `postgres`.                     |
| `StorageProvider`      | Opens and validates one backend, then returns a repository bundle.             |
| `RepositoryBundle`     | Groups workspace, item, audit, navigation, AI settings, knowledge, and health. |
| `StorageStatusService` | Reports effective option, driver, paths, environment lock state, and health.   |
| `StorageSyncService`   | Copies app-owned state between Local providers after explicit confirmation.    |

Provider implementations:

| Provider           | Runtime Use      | Owns                                                                |
|--------------------|------------------|---------------------------------------------------------------------|
| `DataDirProvider`  | Local `datadir`  | YAML/JSONL paths, file-backed repositories, file schema validation. |
| `SQLiteProvider`   | Local `database` | SQLite connection, migrations, SQL repositories, database health.   |
| `PostgresProvider` | Cloud `database` | Postgres connection, migrations, SQL repositories, database health. |

Rules:

- Server composition depends on `StorageProvider` and `RepositoryBundle`, not concrete file or SQL repositories.
- Domain services receive repository interfaces only.
- Manual sync is a separate service and must not introduce runtime dual-write.
- Adding a future backend requires a new provider plus tests, not changes to workspace, item, audit, navigation, or AI
  services.

## Branch Re-index Algorithm

```text
loadBranch(workspace, selectedBranch, force)
  resolve branch ref and commit
  choose working_tree reader when selected branch equals checkout branch
  calculate source configuration hash
  calculate working tree hash for working_tree mode
  read branch scan metadata from configured storage
  if force is false and metadata matches current Git state:
    return indexed items for workspace and branch
  scan branch through filesystem or Git tree reader
  replace stored rows for workspace and branch
  update workspace last scanned and last selected branch
  return indexed items for workspace and branch
```

Cloud mode runs the scan in the Cloud Agent. The Cloud API receives scan results and freshness metadata from the owner
agent, then writes only safe metadata and derived rows to Postgres.

## API Contract

Public route shapes remain stable except for storage settings and sync operations.

| Method | Endpoint                             | Change                                                               |
|--------|--------------------------------------|----------------------------------------------------------------------|
| GET    | `/api/health`                        | Includes database readiness when the active backend is `database`.   |
| GET    | `/api/state`                         | Version hash uses the configured workspace and item repositories.    |
| GET    | `/api/workspaces`                    | Reads from the configured workspace repository.                      |
| POST   | `/api/workspaces/:id/scan`           | Writes branch index rows through the configured item repository.     |
| POST   | `/api/workstreams/:id/branches/load` | Re-indexes stale branch rows, then returns items.                    |
| GET    | `/api/system/storage`                | Returns effective storage option, lock state, paths, and sync state. |
| PUT    | `/api/system/storage`                | Saves Local bootstrap storage option and data directory settings.    |
| POST   | `/api/system/storage/sync`           | Runs confirmed local sync between `database` and `datadir`.          |

## Manual Storage Sync

Local mode supports explicit sync between database and data-dir stores:

| Direction             | Source           | Target           |
|-----------------------|------------------|------------------|
| `datadir_to_database` | YAML/JSONL files | SQLite tables    |
| `database_to_datadir` | SQLite tables    | YAML/JSONL files |

Sync is replace-style and requires confirmation. Before replacing target data, the service writes a timestamped backup
under `KODE_STREAM_DATA_DIR/backups/storage-sync/`. Sync covers workspaces, branch indexes, scan warnings, audit events,
navigation state, AI settings, and knowledge index metadata when those stores are present.

## Error Handling

| Condition                    | Code          | Status |
|------------------------------|---------------|--------|
| Database unavailable         | `unavailable` | 503    |
| Migration required           | `unavailable` | 503    |
| Storage option invalid       | `validation`  | 400    |
| Cloud datadir requested      | `validation`  | 400    |
| Sync confirmation missing    | `validation`  | 400    |
| Sync source unreadable       | `validation`  | 400    |
| Workspace missing            | `not_found`   | 404    |
| Stale branch re-index failed | `scan_failed` | 500    |
| Cloud Agent scan unavailable | `unavailable` | 503    |
