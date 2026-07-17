# Storage

Kode Stream stores app-owned state separately from repository-owned source files. Repository files and Git history remain
the source of truth for planning content. App-owned state covers workspace registration, derived item indexes, branch
scan metadata, scan warnings, audit events, navigation state, and AI settings.

## Current Storage

The current implementation uses SQL-backed storage:

| Runtime mode | Storage driver | Location or service                     | Purpose                                  |
|--------------|----------------|-----------------------------------------|------------------------------------------|
| Local        | SQLite         | `<KODE_STREAM_DATA_DIR>/kode-stream.db` | Dependency-free local durable app state. |
| Cloud        | Postgres       | `KODE_STREAM_DATABASE_URL`              | Shared hosted control-plane state.       |

Local mode defaults to SQLite when no storage driver is set. Cloud mode requires Postgres and fails startup when
Postgres is missing, unreachable, or not migrated.

| Variable                     | Local behavior                        | Cloud behavior                               |
|------------------------------|---------------------------------------|----------------------------------------------|
| `KODE_STREAM_STORAGE_DRIVER` | Defaults to `sqlite`                  | Must be `postgres`                           |
| `KODE_STREAM_SQLITE_PATH`    | Optional SQLite file override         | Unused                                       |
| `KODE_STREAM_DATABASE_URL`   | Unused                                | Required secret-managed Postgres URL         |
| `KODE_STREAM_MIGRATIONS`     | `auto` by default, `manual` supported | `auto` or operator-managed `manual`          |
| `KODE_STREAM_DATA_DIR`       | Holds SQLite file and legacy imports  | Holds optional imports, exports, diagnostics |

`/api/health` reports database connectivity and `migrationVersion` when SQL storage is configured.

## Deprecated Data-Dir Storage

Deprecated: older Kode Stream versions used individual files under `KODE_STREAM_DATA_DIR` as the primary store.

| Legacy file          | Previous responsibility                     | Current role                  |
|----------------------|---------------------------------------------|-------------------------------|
| `workspaces.yaml`    | Workspace registry                          | One-time SQLite import source |
| `item-index.yaml`    | Derived item index and branch scan metadata | One-time SQLite import source |
| `audit-log.jsonl`    | Append-only audit log                       | One-time SQLite import source |
| `saved-filters.yaml` | Saved filters                               | One-time SQLite import source |
| `recent-items.yaml`  | Recent items                                | One-time SQLite import source |
| `ai-settings.yaml`   | AI provider and terminal launch settings    | One-time SQLite import source |

The deprecated files are still supported for migration. On first SQLite startup, Kode Stream imports them into
`kode-stream.db`, records the completed import in SQL, and leaves the original files untouched. After import, runtime
reads and writes use SQLite by default. Changes are not written back to the YAML or JSONL files.

The deprecated data-dir storage model is not supported as Cloud storage. Cloud mode uses Postgres for durable shared
state while `KODE_STREAM_DATA_DIR` remains useful for optional imports, exports, diagnostics, and rollback artifacts.

## Persistence Ownership (data-dir based)

The old data-dir implementation is domain-owned and spread across packages:

| Package                       | Legacy responsibility                                      |
|-------------------------------|------------------------------------------------------------|
| `internal/workspace/registry` | Workspace file persistence for `workspaces.yaml`.          |
| `internal/item/index`         | Item index file persistence for `item-index.yaml`.         |
| `internal/audit`              | Audit JSONL persistence for `audit-log.jsonl`.             |
| `internal/navigation`         | Saved filter and recent item persistence.                  |
| `internal/ai`                 | AI settings persistence for `ai-settings.yaml`.            |
| `internal/system`             | Path resolution for `KODE_STREAM_DATA_DIR` and file paths. |

The new SQL implementation is centralized mostly in `internal/storage`, where it provides SQL-backed implementations
for those same repository interfaces.

That means the architecture is currently mixed:

| Storage model    | Persistence ownership                                                                |
|------------------|--------------------------------------------------------------------------------------|
| Old file storage | Persistence logic lives inside each domain package.                                  |
| New DB storage   | Persistence logic lives in `internal/storage`, while domain packages use interfaces. |

This is intentional for PM-033 migration because it let the app keep existing domain interfaces and swap repository
implementations at startup. The downside is that persistence ownership is inconsistent: file-backed repositories remain
domain-local, while SQL-backed repositories are centralized.

## Why SQL Replaced Data-Dir Files

| Area                | Deprecated data-dir files                         | Current SQL storage                                       |
|---------------------|---------------------------------------------------|-----------------------------------------------------------|
| Local setup         | Simple files, no database service                 | SQLite keeps the same no-service local setup              |
| Cloud support       | No safe multi-process/shared-state story          | Postgres supports hosted shared state and backups         |
| Branch indexes      | YAML replacement of derived rows                  | Transactional branch row, warning, and metadata updates   |
| Migration tracking  | Implicit file shape                               | Explicit schema migration version and import status       |
| Health checks       | File existence and read/write errors only         | Database connectivity and migration readiness             |
| Backup/restore      | Copy several files and preserve ordering manually | Copy one SQLite file locally or use Postgres backup tools |
| Operator confidence | Harder to verify partial writes or stale indexes  | SQL constraints, transactions, and release gates          |

The main benefit is preserving local simplicity while adding a durable Cloud-ready storage contract. SQLite keeps local
installations dependency-free, and Postgres gives Cloud deployments concurrent write safety, managed backups, readiness
checks, and explicit migrations. PM-033 does not claim a general local performance advantage over the deprecated
data-dir files; for Local mode the database primarily improves consistency, observability, migration tracking, and
alignment with the Cloud storage contract.

## Performance Comparison

The table below is an expected performance profile, not a benchmark result. Actual latency depends on workspace size,
item count, filesystem speed, SQLite pragmas, Postgres network distance, and query shape.

| Operation                     | Data-dir files                                                           | Local SQLite                                                      | Cloud Postgres                                                  |
|-------------------------------|--------------------------------------------------------------------------|-------------------------------------------------------------------|-----------------------------------------------------------------|
| Cold startup with small state | Often fastest because the app reads a few local files directly.          | Similar, with extra database open and migration checks.           | Slower than local storage because startup includes network I/O. |
| Full branch re-index write    | Rewrites YAML-derived index files and can be cheap for small indexes.    | Similar or better for larger indexes when rows change in batches. | Depends on network and transaction size; wins on shared writes. |
| Filtered board/search reads   | Must load and scan in-memory file-backed data unless separately indexed. | Can use SQL predicates and indexes for branch/status queries.     | Can use SQL predicates and indexes, with network latency added. |
| Single-record metadata update | May require rewriting a whole YAML file, depending on the domain store.  | Updates one row in a transaction.                                 | Updates one row in a transaction over the network.              |
| Concurrent writers            | File locks and partial-write handling are difficult across processes.    | Better for one local process, limited for many writers.           | Best option here; designed for shared hosted control-plane use. |
| Crash during write            | Requires domain-specific temp-file and recovery discipline.              | Transaction rollback prevents partial SQL state.                  | Transaction rollback prevents partial SQL state.                |
| Backup performance            | Copies several files and must preserve ordering across related files.    | Copies or dumps one local database file.                          | Uses managed snapshot or logical dump tooling.                  |

For small Local installations, the deprecated data-dir model can be as fast as, or faster than, SQLite on simple reads
because it avoids SQL overhead. SQLite becomes more attractive when the app needs indexed branch/status queries,
single-record updates, transactional replacement of branch rows, and explicit migration state. Postgres is not a local
latency optimization; it is the Cloud storage backend for shared state, concurrent writes, managed backups, and health
gates.

## Local SQLite Operations

Back up SQLite by stopping Kode Stream and copying `kode-stream.db`. For a portable export, use:

```bash
sqlite3 kode-stream.db .dump > kode-stream.sql
```

Restore by stopping Kode Stream, replacing the SQLite file or importing the dump into a new file, then starting the app
and checking `/api/health`.

## Cloud Postgres Operations

Provision Postgres with encrypted storage, regular backups, and network access limited to the Kode Stream API and
approved operator tooling. Cloud Agents never connect to Postgres; they connect only to the Cloud API over HTTPS and
WebSocket.

For manual migrations, run the Kode Stream binary once with `KODE_STREAM_MIGRATIONS=auto` against the target database
from an operator job, then deploy the API with `KODE_STREAM_MIGRATIONS=manual`.

Back up Postgres with the managed provider snapshot feature or `pg_dump`. Restore from a snapshot or logical dump, then
verify `/api/health`, workspace listing, branch load, and audit event reads.
