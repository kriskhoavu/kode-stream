# PM-033: Configurable App-State Storage For Kode Stream

PM-033 lets operators choose the app-owned state backend at startup. Local mode supports `database` and `datadir`.
Cloud mode uses `database` with Postgres. Repository source files, Git credentials, terminal execution, AI sessions,
and verification commands remain outside app-state storage and follow the PM-032 Cloud Agent boundary.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                   |
|-------------------------------|--------------------------|-----------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Local app baseline       | Established workspace registration, scanning, board, and item workspace behavior.             |
| [PM-013](../PM-013/README.md) | Branch context baseline  | Added branch-scoped scans with commit, source mode, and working-tree freshness metadata.      |
| [PM-030](../PM-030/README.md) | API boundary baseline    | Added Gin transport and repository/service seams without changing storage behavior.           |
| [PM-031](../PM-031/README.md) | API route baseline       | Confirms all `/api/*` routes are Gin-owned for storage-backed handlers and middleware.        |
| [PM-032](../PM-032/README.md) | Cloud execution baseline | Defines Cloud as hosted UI/API/control plane with local agent execution and no hosted clones. |

## Goal

Provide one storage boundary with an operator-selected backend:

- Local `datadir` mode is the default and persists app-owned state in YAML and JSONL files under `KODE_STREAM_DATA_DIR`.
- Local `database` mode persists app-owned state in SQLite under `KODE_STREAM_DATA_DIR`.
- Cloud mode persists shared control-plane state in Postgres.
- Domain services use repository interfaces and do not branch on the selected backend.
- Branch-scoped item indexes can be re-indexed when Git branch content changes.
- Settings exposes the active storage option and local-only manual sync between `database` and `datadir`.

## Non-Goals

- No repository source file storage in app-state storage.
- No Git credential, SSH key, token, terminal transcript, or prompt storage in app-state storage.
- No Cloud-hosted clone or Cloud-hosted command execution.
- No frontend redesign.
- No automatic dual-write between `database` and `datadir`.
- No Cloud `datadir` backend.
- No change to public API response shapes unless needed for storage status and sync operations.
- No vector database or semantic search storage.

## Glossary

| Term                 | Meaning                                                              | Code                                      |
|----------------------|----------------------------------------------------------------------|-------------------------------------------|
| Storage Option       | Admin-facing backend choice for app-owned state.                     | `database`, `datadir`                     |
| Storage Driver       | Internal repository implementation selected from the storage option. | `file`, `sqlite`, `postgres`              |
| Database Store       | Database app-state store. Local uses SQLite, Cloud uses Postgres.    | SQLite file or `KODE_STREAM_DATABASE_URL` |
| Data-dir Store       | File-backed app-state store under the configured data directory.     | YAML and JSONL files                      |
| App-Owned State      | Metadata Kode Stream owns outside Git repositories.                  | workspaces, indexes, audit, settings      |
| Repository Content   | Source files and Git history in registered workspaces.               | local workspace or Cloud Agent workspace  |
| Branch Index         | Derived item read model for one workspace branch.                    | workspace, branch, item ID                |
| Branch Scan Metadata | Freshness record for one workspace branch scan.                      | commit and source hashes                  |
| Re-index             | Scan branch content and replace derived rows for that branch.        | `ReplaceWorkspaceBranch` behavior         |
| Manual Sync          | Explicit local copy between `database` and `datadir` stores.         | Settings storage sync action              |

## Runtime Configuration

| Setting                      | Local Default                | Cloud Requirement       | Purpose                                            |
|------------------------------|------------------------------|-------------------------|----------------------------------------------------|
| `KODE_STREAM_STORAGE_OPTION` | `datadir`                    | `database`              | Selects `database` or `datadir` app-state storage. |
| `KODE_STREAM_STORAGE_DRIVER` | derived from storage option  | `postgres`              | Optional low-level compatibility override.         |
| `KODE_STREAM_SQLITE_PATH`    | `<data-root>/kode-stream.db` | unused                  | Overrides Local SQLite file path.                  |
| `KODE_STREAM_DATABASE_URL`   | unused                       | required                | Connects Cloud API to Postgres.                    |
| `KODE_STREAM_DATA_DIR`       | OS config dir                | mounted metadata volume | Holds app-state files, SQLite, backups, and logs.  |
| `KODE_STREAM_MIGRATIONS`     | `auto`                       | `auto` or operator-run  | Controls database migration behavior.              |

When both storage option and low-level storage driver are configured, the low-level driver wins and Settings reports the
effective option as environment-locked.

## Component Flow

Local database option: Browser -> Kode Stream local API -> repository services -> SQLite store -> registered local
workspace.

Local datadir option: Browser -> Kode Stream local API -> repository services -> YAML/JSONL store -> registered local
workspace.

Cloud mode: Browser -> Kode Stream Cloud API -> Postgres store for metadata and indexes -> Cloud Agent channel -> user
machine repository for scans, Git, terminal, AI CLI, and verification.

| Component             | Local `database`            | Local `datadir`    | Cloud `database`                       |
|-----------------------|-----------------------------|--------------------|----------------------------------------|
| API process           | User machine                | User machine       | Cloud/VPS                              |
| App-state store       | SQLite file on user machine | YAML/JSONL files   | Postgres service                       |
| Workspace repository  | User-selected local path    | User-selected path | User-selected path through Cloud Agent |
| Branch scan execution | Local API process           | Local API process  | Cloud Agent on user machine            |
| Index persistence     | SQLite                      | `item-index.yaml`  | Postgres                               |
| Command execution     | User machine                | User machine       | User machine through Cloud Agent       |

## Branch Re-indexing

Indexes are derived data. Every storage backend must preserve branch scope so one branch cannot leak cards into another
branch.

Branch load uses stored rows only when the stored scan metadata matches the selected branch:

1. Resolve selected branch, ref, and commit.
2. Calculate source configuration hash.
3. Calculate working-tree hash for editable working-tree scans.
4. Read branch scan metadata from the configured storage backend.
5. Use cached branch index only when commit and hashes match.
6. Re-index the selected branch when metadata is missing or stale.
7. Replace item rows, warnings, and branch scan metadata for that workspace and branch.

## Manual Storage Sync

Settings provides local-only manual sync in both directions:

- `datadir -> database` copies current YAML/JSONL app-owned state into SQLite.
- `database -> datadir` exports current SQLite app-owned state into YAML/JSONL files.
- Each sync creates a timestamped backup under `KODE_STREAM_DATA_DIR/backups/storage-sync/`.
- Sync is explicit and replace-style; runtime writes go only to the selected backend.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Infrastructure Design](design/design-02-infrastructure.md)
- [Pipeline Design](design/design-03-pipeline.md)
- [Implementation Plan](implementation-plan.md)
