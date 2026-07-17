# Scenarios: PM-033 Overview

## Scenario List

| #   | Title                        | Description                                                             |
|-----|------------------------------|-------------------------------------------------------------------------|
| 1   | Local database startup       | Local app starts with SQLite-backed app-owned state.                    |
| 2   | Local datadir startup        | Local app starts with YAML/JSONL-backed app-owned state.                |
| 3   | Storage option change        | Admin changes the Local storage option and restarts the app.            |
| 4   | Manual storage sync          | Admin copies app-owned state between database and datadir stores.       |
| 5   | Branch re-index on switch    | Selected branch content is scanned again when cached metadata is stale. |
| 6   | Cloud Postgres startup       | Cloud app requires Postgres and reports database health.                |
| 7   | Cloud Agent scan persistence | Agent scans locally and Cloud stores derived metadata in Postgres.      |

## Scenario 1: Local Database Startup

## Goal

Run Kode Stream locally with database storage and no external database service.

## Starting State

| #   | Title          | Summary                                            |
|-----|----------------|----------------------------------------------------|
| 1   | Runtime mode   | Local mode is selected or defaulted.               |
| 2   | Storage option | `database` is explicitly selected.                 |
| 3   | Data dir       | `KODE_STREAM_DATA_DIR` resolves to a writable dir. |
| 4   | Database       | SQLite file may exist or may need to be created.   |

## Execution Flow

```text
User starts Kode Stream
    ↓
Backend resolves Local mode and database storage option
    ↓
Backend opens SQLite and runs migrations
    ↓
Repository services use database adapters
    ↓
Browser loads workspaces, board, search, and item views
```

## Expected Result

The local app starts with SQLite-backed app-owned state. Workspace files and Git operations still use registered local
repository paths.

## Scenario 2: Local Datadir Startup

## Goal

Run Kode Stream locally with data-dir storage.

## Starting State

| #   | Title          | Summary                                           |
|-----|----------------|---------------------------------------------------|
| 1   | Runtime mode   | Local mode is selected.                           |
| 2   | Storage option | `datadir` is selected.                            |
| 3   | Data dir       | App-owned YAML/JSONL files may exist or be empty. |

## Execution Flow

```text
User starts Kode Stream
    ↓
Backend resolves Local mode and datadir storage option
    ↓
Repository services use file-backed adapters
    ↓
Browser loads workspaces, board, search, and item views
```

## Expected Result

The local app reads and writes app-owned state under `KODE_STREAM_DATA_DIR`. Database migrations and database health are
not required for this backend.

## Scenario 3: Storage Option Change

## Goal

Allow an admin to choose the Local storage backend for the next app run.

## Starting State

| #   | Title      | Summary                                                   |
|-----|------------|-----------------------------------------------------------|
| 1   | App        | Kode Stream is running in Local mode.                     |
| 2   | Settings   | Storage settings are not locked by environment variables. |
| 3   | Preference | Admin selects `database` or `datadir`.                    |

## Execution Flow

```text
Admin opens Settings → Storage
    ↓
Admin selects storage option
    ↓
Backend writes bootstrap storage settings
    ↓
UI shows restart required
    ↓
Admin restarts Kode Stream
    ↓
Backend composes repositories from the selected option
```

## Expected Result

The selected option applies on the next process start. Runtime writes go only to the active backend.

## Scenario 4: Manual Storage Sync

## Goal

Let an admin explicitly copy Local app-owned state between database and datadir stores.

## Starting State

| #   | Title        | Summary                                     |
|-----|--------------|---------------------------------------------|
| 1   | Runtime      | Kode Stream is running in Local mode.       |
| 2   | Source       | Source store contains app-owned state.      |
| 3   | Target       | Target store may contain replaceable state. |
| 4   | Confirmation | Admin confirms replace-style sync.          |

## Execution Flow

```text
Admin opens Settings → Storage
    ↓
Admin chooses datadir_to_database or database_to_datadir
    ↓
Backend validates source and target stores
    ↓
Backend writes timestamped backup of target state
    ↓
Backend replaces target app-owned state
    ↓
UI shows sync summary and backup path
```

## Expected Result

Workspaces, branch indexes, scan warnings, audit events, navigation state, AI settings, and knowledge index metadata are
copied to the target store. Repository files, credentials, terminal state, and source content are not copied.

## Scenario 5: Branch Re-index On Switch

## Goal

Show board and search data for the selected branch when branch content differs.

## Starting State

| #   | Title        | Summary                                                |
|-----|--------------|--------------------------------------------------------|
| 1   | Workspace    | Workspace has at least two Git branches.               |
| 2   | Branch index | Storage has branch scan metadata for one branch.       |
| 3   | Git state    | User switches to a branch with different item content. |

## Execution Flow

```text
User selects or switches branch
    ↓
Workstream service resolves branch ref and commit
    ↓
Service compares stored branch scan metadata
    ↓
Metadata is missing or stale
    ↓
Scanner reads selected branch content
    ↓
Configured repository replaces rows for workspace and branch
    ↓
Board/search views load current branch items
```

## Expected Result

Items from another branch are not shown for the selected branch.

## Scenario 6: Cloud Postgres Startup

## Goal

Run Cloud mode with durable shared control-plane storage.

## Starting State

| #   | Title      | Summary                                                  |
|-----|------------|----------------------------------------------------------|
| 1   | Runtime    | `KODE_STREAM_MODE=cloud`.                                |
| 2   | Storage    | `KODE_STREAM_STORAGE_OPTION=database`.                   |
| 3   | Connection | `KODE_STREAM_DATABASE_URL` points to reachable Postgres. |

## Execution Flow

```text
Cloud API starts
    ↓
Backend validates database configuration
    ↓
Backend runs or verifies migrations
    ↓
Health endpoint reports database readiness
    ↓
Hosted UI loads metadata through database-backed services
```

## Expected Result

Cloud API is ready only when Postgres is reachable and migrated.

## Scenario 7: Cloud Agent Scan Persistence

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
remain on the user machine.
