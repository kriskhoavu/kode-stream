# PM-033: Database Storage For Kode Stream

PM-033 moves Kode Stream app-owned state into SQL storage. Local mode uses SQLite on the user's machine. Cloud mode uses
Postgres on operator infrastructure. Repository source files, Git credentials, terminal execution, AI sessions, and
verification commands remain outside the database and follow the PM-032 Cloud Agent boundary.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                   |
|-------------------------------|--------------------------|-----------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Local app baseline       | Established workspace registration, scanning, board, and item workspace behavior.             |
| [PM-013](../PM-013/README.md) | Branch context baseline  | Added branch-scoped scans with commit, source mode, and working-tree freshness metadata.      |
| [PM-030](../PM-030/README.md) | API boundary baseline    | Added Gin transport and repository/service seams without changing storage behavior.           |
| [PM-031](../PM-031/README.md) | API route baseline       | Confirms all `/api/*` routes are Gin-owned for storage-backed handlers and middleware.        |
| [PM-032](../PM-032/README.md) | Cloud execution baseline | Defines Cloud as hosted UI/API/control plane with local agent execution and no hosted clones. |

## Goal

Provide one storage boundary that supports Local and Cloud mode:

- Local mode persists app-owned state in SQLite under `KODE_STREAM_DATA_DIR`.
- Cloud mode persists shared control-plane state in Postgres.
- Domain services use repository interfaces and do not depend on YAML files directly.
- Branch-scoped item indexes can be re-indexed when Git branch content changes.

## Non-Goals

- No repository source file storage in SQL.
- No Git credential, SSH key, token, terminal transcript, or prompt storage in SQL.
- No Cloud-hosted clone or Cloud-hosted command execution.
- No frontend redesign.
- No change to public API response shapes unless needed for database health reporting.
- No vector database or semantic search storage.

## Glossary

| Term                 | Meaning                                                                | Code                                     |
|----------------------|------------------------------------------------------------------------|------------------------------------------|
| Storage Driver       | Runtime-selected SQL backend.                                          | `sqlite`, `postgres`                     |
| SQLite Store         | Local single-process database file.                                    | `<data-root>/kode-stream.db`             |
| Postgres Store       | Cloud database for hosted metadata, indexes, users, agents, and audit. | `KODE_STREAM_DATABASE_URL`               |
| App-Owned State      | Metadata Kode Stream owns outside Git repositories.                    | workspaces, indexes, audit, settings     |
| Repository Content   | Source files and Git history in registered workspaces.                 | local workspace or Cloud Agent workspace |
| Branch Index         | Derived item read model for one workspace branch.                      | `workspace_id`, `branch`, `item_id`      |
| Branch Scan Metadata | Freshness record for one workspace branch scan.                        | commit and source hashes                 |
| Re-index             | Scan branch content and replace derived SQL rows for that branch.      | `ReplaceWorkspaceBranch` behavior        |
| SQL Migration        | Versioned schema change run at startup or deploy time.                 | embedded migrations                      |

## Runtime Configuration

| Setting                      | Local Default                | Cloud Requirement       | Purpose                                   |
|------------------------------|------------------------------|-------------------------|-------------------------------------------|
| `KODE_STREAM_STORAGE_DRIVER` | `sqlite`                     | `postgres`              | Selects SQL storage backend.              |
| `KODE_STREAM_SQLITE_PATH`    | `<data-root>/kode-stream.db` | unused                  | Overrides Local SQLite file path.         |
| `KODE_STREAM_DATABASE_URL`   | unused                       | required                | Connects Cloud API to Postgres.           |
| `KODE_STREAM_DATA_DIR`       | OS config dir                | mounted metadata volume | Holds SQLite, imports, exports, and logs. |
| `KODE_STREAM_MIGRATIONS`     | `auto`                       | `auto` or operator-run  | Controls startup migration behavior.      |

## Component Flow

Local mode: Browser -> Kode Stream local API -> repository services -> SQLite store -> registered local workspace.

Cloud mode: Browser -> Kode Stream Cloud API -> Postgres store for metadata and indexes -> Cloud Agent channel -> user
machine repository for scans, Git, terminal, AI CLI, and verification.

| Component             | Local Mode                  | Cloud Mode                             |
|-----------------------|-----------------------------|----------------------------------------|
| API process           | User machine                | Cloud/VPS                              |
| SQL database          | SQLite file on user machine | Postgres service                       |
| Workspace repository  | User-selected local path    | User-selected path through Cloud Agent |
| Branch scan execution | Local API process           | Cloud Agent on user machine            |
| Index persistence     | SQLite                      | Postgres                               |
| Command execution     | User machine                | User machine through Cloud Agent       |

## Branch Re-indexing

Indexes are derived data. SQL storage must preserve branch scope so one branch cannot leak cards into another branch.

Branch load uses stored rows only when the stored scan metadata matches the selected branch:

1. Resolve selected branch, ref, and commit.
2. Calculate source configuration hash.
3. Calculate working-tree hash for editable working-tree scans.
4. Read branch scan metadata from SQL.
5. Use cached branch index only when commit and hashes match.
6. Re-index the selected branch when metadata is missing or stale.
7. Replace SQL item rows, warnings, and branch scan metadata for that workspace and branch atomically.

## Design Decisions

| Decision                                     | Rationale                                                                                     |
|----------------------------------------------|-----------------------------------------------------------------------------------------------|
| Use SQLite for Local mode                    | Keeps local setup dependency-free while adding transactions, migrations, and one-file backup. |
| Use Postgres for Cloud mode                  | Supports multi-user control-plane state, concurrent writes, and managed backups.              |
| Keep one repository interface                | Domain services should not branch on file, SQLite, or Postgres storage details.               |
| Keep repository content outside SQL          | Git remains the source of truth for planning files and branch content.                        |
| Store branch indexes by workspace and branch | Branch-specific content can differ and must be queried independently.                         |
| Re-index stale branch rows                   | Board and search views must reflect the selected branch content.                              |
| Import app-owned file state into SQL         | Existing local users keep workspaces, indexes, audit, settings, and navigation.               |
| Leave PM-032 Cloud Agent boundary unchanged  | Database storage must not move execution or repository access into Cloud hosts.               |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Infrastructure Design](design/design-02-infrastructure.md)
- [Pipeline Design](design/design-03-pipeline.md)
- [Implementation Plan](implementation-plan.md)
