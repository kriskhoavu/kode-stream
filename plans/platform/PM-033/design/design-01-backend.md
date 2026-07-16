# Backend Design: Database Storage For Kode Stream

## Overview

The backend adds SQL storage adapters behind repository interfaces. Local mode uses SQLite. Cloud mode uses Postgres.
The domain model keeps Git repositories as the source of truth for planning content and uses SQL only for app-owned
state and derived indexes.

## Storage Boundary

| Domain Area | Current Owner                 | SQL Responsibility                                                    |
|-------------|-------------------------------|-----------------------------------------------------------------------|
| Workspace   | `internal/workspace/registry` | Persist workspace metadata, registration mode, sources, runtime.      |
| Item index  | `internal/item/index`         | Persist branch-scoped item summaries, details, warnings, scan state.  |
| Workstream  | `internal/workstream`         | Query and refresh branch indexes through repository interfaces.       |
| Audit       | `internal/audit`              | Append and query operation events.                                    |
| Navigation  | `internal/navigation`         | Persist saved filters and recent items.                               |
| AI settings | `internal/ai`                 | Persist provider settings and launch preferences.                     |
| Knowledge   | `internal/knowledge`          | Persist derived knowledge index metadata without source file content. |

## Runtime Configuration

| Setting                      | Accepted Values      | Required In | Behavior                                              |
|------------------------------|----------------------|-------------|-------------------------------------------------------|
| `KODE_STREAM_STORAGE_DRIVER` | `sqlite`, `postgres` | both modes  | Selects SQL adapter.                                  |
| `KODE_STREAM_SQLITE_PATH`    | filesystem path      | optional    | Overrides Local SQLite path.                          |
| `KODE_STREAM_DATABASE_URL`   | Postgres URL         | Cloud       | Connects hosted API to Postgres.                      |
| `KODE_STREAM_MIGRATIONS`     | `auto`, `manual`     | both modes  | Runs embedded migrations or requires operator action. |

Local mode defaults to SQLite when no storage driver is set. Cloud mode requires Postgres and fails startup when the
database URL is missing or unreachable.

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

The implementation may add tables for scan warnings, audit events, saved filters, recent items, settings, Cloud users,
Cloud agents, and Cloud workspaces using the same migration mechanism.

## Repository Interfaces

| Interface Role         | Required Behavior                                                                    |
|------------------------|--------------------------------------------------------------------------------------|
| Workspace repository   | List, get, create, batch create, update, delete, touch scanned, set selected branch. |
| Item index repository  | Query items, get item detail, replace workspace index, replace branch index, delete. |
| Branch scan repository | Read freshness metadata and atomically update metadata with indexed rows.            |
| Audit repository       | Append events and filter by workspace.                                               |
| Navigation repository  | Save filters and recent items without browser-only persistence.                      |
| Migration repository   | Track schema version and one-time file import status.                                |

Domain services depend on these interfaces. SQL adapters own database details, transactions, and dialect differences.

## Branch Re-index Algorithm

```text
loadBranch(workspace, selectedBranch, force)
  resolve branch ref and commit
  choose working_tree reader when selected branch equals checkout branch
  calculate source configuration hash
  calculate working tree hash for working_tree mode
  read branch scan metadata from SQL
  if force is false and metadata matches current Git state:
    return SQL indexed items for workspace and branch
  scan branch through filesystem or Git tree reader
  replace SQL rows for workspace and branch in one transaction
  update workspace last scanned and last selected branch
  return SQL indexed items for workspace and branch
```

Cloud mode runs the scan in the Cloud Agent. The Cloud API receives scan results and freshness metadata from the owner
agent, then writes only safe metadata and derived rows to Postgres.

## API Contract

Public route shapes remain stable. Database-specific status is exposed through existing health/state surfaces.

| Method | Endpoint                             | Change                                                                |
|--------|--------------------------------------|-----------------------------------------------------------------------|
| GET    | `/api/health`                        | Include database connectivity and migration version in health checks. |
| GET    | `/api/state`                         | Version hash uses SQL workspace and item index state.                 |
| GET    | `/api/workspaces`                    | Reads from configured SQL workspace repository.                       |
| POST   | `/api/workspaces/:id/scan`           | Writes branch index rows through SQL item index repository.           |
| POST   | `/api/workstreams/:id/branches/load` | Re-indexes stale branch rows, then returns items.                     |

## File Import

On first SQL startup, the app imports existing app-owned files from `KODE_STREAM_DATA_DIR`:

| Source File            | SQL Target                          |
|------------------------|-------------------------------------|
| `workspaces.yaml`      | workspace tables                    |
| `item-index.yaml`      | indexed item and branch scan tables |
| `audit-log.jsonl`      | audit events                        |
| `saved-filters.yaml`   | saved filters                       |
| `recent-items.yaml`    | recent items                        |
| `ai-settings.yaml`     | AI settings                         |
| `knowledge-index.yaml` | knowledge index metadata            |

Imports are idempotent and recorded in SQL. Source files are left untouched.

## Error Handling

| Condition                    | Code          | Status |
|------------------------------|---------------|--------|
| Database unavailable         | `unavailable` | 503    |
| Migration required           | `unavailable` | 503    |
| SQL constraint violation     | `validation`  | 400    |
| Workspace missing            | `not_found`   | 404    |
| Stale branch re-index failed | `scan_failed` | 500    |
| Cloud Agent scan unavailable | `unavailable` | 503    |

## Design Decisions

| Decision                              | Rationale                                                                            |
|---------------------------------------|--------------------------------------------------------------------------------------|
| Use embedded migrations               | The binary can create and upgrade SQLite and Postgres schemas consistently.          |
| Keep SQL behind interfaces            | Existing services can move incrementally without storage-specific branches.          |
| Make branch index replacement atomic  | Board/search views should not observe mixed rows from two scans.                     |
| Store JSON for flexible item metadata | Planning metadata evolves faster than board query fields.                            |
| Keep source files out of SQL          | Git remains the source of truth and PM-032 keeps repository access on user machines. |
| Fail Cloud startup without Postgres   | Cloud needs durable shared state and concurrent write safety.                        |
