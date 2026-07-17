# Storage

Kode Stream stores app-owned state separately from repository-owned source files. Repository files and Git history remain
the source of truth for planning content. App-owned state covers workspace registration, derived item indexes, branch
scan metadata, scan warnings, audit events, navigation state, and AI settings.

## Supported Options

Storage is selected at startup with `KODE_STREAM_STORAGE_OPTION`.

| Runtime mode | Storage option | Internal driver | Location or service                     | Purpose                                  |
|--------------|----------------|-----------------|-----------------------------------------|------------------------------------------|
| Local        | `database`     | `sqlite`        | `<KODE_STREAM_DATA_DIR>/kode-stream.db` | Dependency-free local durable app state. |
| Local        | `datadir`      | `file`          | YAML and JSONL files under data dir     | Inspectable file-backed app state.       |
| Cloud        | `database`     | `postgres`      | `KODE_STREAM_DATABASE_URL`              | Shared hosted control-plane state.       |

Local mode defaults to `database`. Cloud mode only supports `database` and fails startup when `datadir` is selected or
Postgres is missing, unreachable, or not migrated.

| Variable                     | Local behavior                             | Cloud behavior                                |
|------------------------------|--------------------------------------------|-----------------------------------------------|
| `KODE_STREAM_STORAGE_OPTION` | `database` or `datadir`, default database  | Must be `database`                            |
| `KODE_STREAM_STORAGE_DRIVER` | Optional low-level override                | `postgres`                                    |
| `KODE_STREAM_SQLITE_PATH`    | Optional SQLite file override              | Unused                                        |
| `KODE_STREAM_DATABASE_URL`   | Unused                                     | Required secret-managed Postgres URL          |
| `KODE_STREAM_MIGRATIONS`     | `auto` by default, `manual` supported      | `auto` or operator-managed `manual`           |
| `KODE_STREAM_DATA_DIR`       | Holds app-state files, SQLite, and backups | Holds optional diagnostics and rollback files |

When `KODE_STREAM_STORAGE_DRIVER` is set, it is treated as an environment-locked compatibility override. Settings shows
the effective option but does not let the user change it until the override is removed.

## Runtime Behavior

Runtime writes go only to the selected backend. Kode Stream does not dual-write between SQLite and YAML/JSONL files.

| Area                 | Local `database` | Local `datadir`      | Cloud `database` |
|----------------------|------------------|----------------------|------------------|
| Workspace registry   | SQLite           | `workspaces.yaml`    | Postgres         |
| Item index           | SQLite           | `item-index.yaml`    | Postgres         |
| Audit events         | SQLite           | `audit-log.jsonl`    | Postgres         |
| Saved filters        | SQLite           | `saved-filters.yaml` | Postgres         |
| Recent items         | SQLite           | `recent-items.yaml`  | Postgres         |
| AI settings          | SQLite           | `ai-settings.yaml`   | Postgres         |
| Branch scan metadata | SQLite           | `item-index.yaml`    | Postgres         |
| Repository source    | Git workspace    | Git workspace        | User Cloud Agent |

`/api/health` reports database connectivity and `migrationVersion` when SQL storage is configured. Settings calls
`/api/storage/status` to show the effective option, environment lock state, data directory, database path, and database
health.

## Manual Sync

Settings provides local-only manual sync in both directions:

| Direction             | Source     | Target     |
|-----------------------|------------|------------|
| `datadir_to_database` | YAML/JSONL | SQLite     |
| `database_to_datadir` | SQLite     | YAML/JSONL |

Each sync is explicit and confirmed by the user. Before replacing the target, Kode Stream writes a timestamped backup
under:

```text
<KODE_STREAM_DATA_DIR>/backups/storage-sync/
```

Sync is replace-style. After a sync, restart with the desired `KODE_STREAM_STORAGE_OPTION` so runtime writes continue in
the intended backend.

## Performance Comparison

The table below is an expected performance profile, not a benchmark result. Actual latency depends on workspace size,
item count, filesystem speed, SQLite pragmas, Postgres network distance, and query shape.

| Operation                     | Local `datadir`                                                         | Local SQLite                                                     | Cloud Postgres                                                  |
|-------------------------------|-------------------------------------------------------------------------|------------------------------------------------------------------|-----------------------------------------------------------------|
| Cold startup with small state | Often fastest because the app reads a few local files directly.         | Similar, with extra database open and migration checks.          | Slower than local storage because startup includes network I/O. |
| Full branch re-index write    | Rewrites YAML-derived index files and can be cheap for small indexes.   | Similar or better for larger indexes when rows change in batches | Depends on network and transaction size; wins on shared writes. |
| Filtered board/search reads   | Loads and scans file-backed data unless separately indexed.             | Uses SQL predicates and indexes for branch/status queries.       | Uses SQL predicates and indexes, with network latency added.    |
| Single-record metadata update | May require rewriting a whole YAML file, depending on the domain store. | Updates one row in a transaction.                                | Updates one row in a transaction over the network.              |
| Concurrent writers            | Best treated as one local process writing files.                        | Better for one local process, limited for many writers.          | Designed for shared hosted control-plane use.                   |
| Crash during write            | Relies on domain-specific temp-file and recovery discipline.            | Transaction rollback prevents partial SQL state.                 | Transaction rollback prevents partial SQL state.                |
| Backup performance            | Copies several files and must preserve related files together.          | Copies or dumps one local database file.                         | Uses managed snapshot or logical dump tooling.                  |

For small Local installations, `datadir` can be as fast as, or faster than, SQLite on simple reads because it avoids SQL
overhead. SQLite becomes more attractive when the app needs indexed branch/status queries, single-record updates,
transactional replacement of branch rows, and explicit migration state. Postgres is not a local latency optimization; it
is the Cloud storage backend for shared state, concurrent writes, managed backups, and health gates.

## Local Operations

Run local database mode:

```bash
KODE_STREAM_STORAGE_OPTION=database ./run.sh restart
```

Run local data-dir mode:

```bash
KODE_STREAM_STORAGE_OPTION=datadir ./run.sh restart
```

Smoke both local options:

```bash
./run.sh smoke-storage
```

Back up SQLite by stopping Kode Stream and copying `kode-stream.db`. For a portable export, use:

```bash
sqlite3 kode-stream.db .dump > kode-stream.sql
```

Back up data-dir mode by stopping Kode Stream and copying the YAML/JSONL app-state files together.

Restore by stopping Kode Stream, replacing the target files or database, starting with the matching
`KODE_STREAM_STORAGE_OPTION`, and checking `/api/health` plus workspace listing.

## Cloud Postgres Operations

Cloud uses `KODE_STREAM_STORAGE_OPTION=database`, `KODE_STREAM_STORAGE_DRIVER=postgres`, and
`KODE_STREAM_DATABASE_URL`.

Provision Postgres with encrypted storage, regular backups, and network access limited to the Kode Stream API and
approved operator tooling. Cloud Agents never connect to Postgres; they connect only to the Cloud API over HTTPS and
WebSocket.

For manual migrations, run the Kode Stream binary once with `KODE_STREAM_MIGRATIONS=auto` against the target database
from an operator job, then deploy the API with `KODE_STREAM_MIGRATIONS=manual`.

Back up Postgres with the managed provider snapshot feature or `pg_dump`. Restore from a snapshot or logical dump, then
verify `/api/health`, workspace listing, branch load, and audit event reads.
