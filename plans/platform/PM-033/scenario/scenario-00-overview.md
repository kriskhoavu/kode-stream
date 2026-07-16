# Scenarios: PM-033 Overview

## Scenario List

| #   | Title                        | Description                                                             |
|-----|------------------------------|-------------------------------------------------------------------------|
| 1   | Local SQLite startup         | Local app creates or opens SQLite and serves existing workflows.        |
| 2   | Local file import            | Existing YAML/JSONL app state imports into SQL without changing files.  |
| 3   | Branch re-index on switch    | Selected branch content is scanned again when cached metadata is stale. |
| 4   | Cloud Postgres startup       | Cloud app requires Postgres and reports database health.                |
| 5   | Cloud Agent scan persistence | Agent scans locally and Cloud stores derived metadata in Postgres.      |

## Scenario 1: Local SQLite Startup

## Goal

Run Kode Stream locally with SQL storage and no external database setup.

## Starting State

| #   | Title        | Summary                                            |
|-----|--------------|----------------------------------------------------|
| 1   | Runtime mode | Local mode is selected or defaulted.               |
| 2   | Data dir     | `KODE_STREAM_DATA_DIR` resolves to a writable dir. |
| 3   | Database     | SQLite file may exist or may need to be created.   |

## Execution Flow

```text
User starts Kode Stream
    ↓
Backend resolves Local mode and SQLite path
    ↓
Backend opens SQLite and runs migrations
    ↓
Repository services use SQL adapters
    ↓
Browser loads workspaces, board, search, and item views
```

## Expected Result

The local app starts with SQLite-backed app state. Workspace files and Git operations still use the registered local
repository paths.

## Scenario 2: Local File Import

## Goal

Preserve existing local app-owned state when SQL storage is introduced.

## Starting State

| #   | Title       | Summary                                                       |
|-----|-------------|---------------------------------------------------------------|
| 1   | Files       | `workspaces.yaml`, `item-index.yaml`, and JSONL logs exist.   |
| 2   | SQL store   | SQLite has the schema but no imported app state.              |
| 3   | Import mark | SQL has no completed import record for the current data root. |

## Execution Flow

```text
Backend starts with SQLite
    ↓
Migration service checks import status
    ↓
Importer reads app-owned YAML and JSONL files
    ↓
Importer writes SQL rows in transactions
    ↓
Importer records completed import
    ↓
Existing files remain in place
```

## Expected Result

Existing workspaces, branch indexes, audit events, settings, saved filters, and recents are available from SQL.

## Scenario 3: Branch Re-index On Switch

## Goal

Show board and search data for the selected branch when branch content differs.

## Starting State

| #   | Title     | Summary                                                |
|-----|-----------|--------------------------------------------------------|
| 1   | Workspace | Workspace has at least two Git branches.               |
| 2   | SQL index | SQL has branch scan metadata for one branch.           |
| 3   | Git state | User switches to a branch with different item content. |

## Execution Flow

```text
User selects or switches branch
    ↓
Workstream service resolves branch ref and commit
    ↓
Service compares SQL branch scan metadata
    ↓
Metadata is missing or stale
    ↓
Scanner reads selected branch content
    ↓
SQL transaction replaces rows for workspace and branch
    ↓
Board/search views load current branch items
```

## Expected Result

Items from another branch are not shown for the selected branch. Stale rows are replaced atomically.

## Scenario 4: Cloud Postgres Startup

## Goal

Run Cloud mode with durable shared control-plane storage.

## Starting State

| #   | Title      | Summary                                                  |
|-----|------------|----------------------------------------------------------|
| 1   | Runtime    | `KODE_STREAM_MODE=cloud`.                                |
| 2   | Storage    | `KODE_STREAM_STORAGE_DRIVER=postgres`.                   |
| 3   | Connection | `KODE_STREAM_DATABASE_URL` points to reachable Postgres. |

## Execution Flow

```text
Cloud API starts
    ↓
Backend validates Postgres configuration
    ↓
Backend runs or verifies migrations
    ↓
Health endpoint reports database readiness
    ↓
Hosted UI loads metadata through SQL-backed services
```

## Expected Result

Cloud API is ready only when Postgres is reachable and migrated.

## Scenario 5: Cloud Agent Scan Persistence

## Goal

Store derived branch metadata in Cloud without storing repository content or executing commands on the Cloud host.

## Starting State

| #   | Title     | Summary                                                    |
|-----|-----------|------------------------------------------------------------|
| 1   | Agent     | User has a connected Cloud Agent.                          |
| 2   | Workspace | Cloud workspace is backed by a user-machine repository.    |
| 3   | Database  | Postgres stores Cloud user, agent, and workspace metadata. |

## Execution Flow

```text
User loads branch in Cloud UI
    ↓
Cloud API requests scan from owner Cloud Agent
    ↓
Agent resolves Git branch and scans local repository
    ↓
Agent sends item metadata, warnings, and scan freshness to Cloud
    ↓
Cloud API writes derived rows to Postgres
    ↓
Cloud UI renders indexed branch data
```

## Expected Result

Cloud stores safe metadata and indexes only. Repository files, credentials, terminals, AI CLI, and verification commands
stay on the user machine.
