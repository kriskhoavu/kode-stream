# Database Storage

Kode Stream stores app-owned state in SQL. Local mode defaults to SQLite. Cloud mode requires Postgres. Git repositories
remain the source of truth for repository content; SQL stores workspace metadata, derived item indexes, scan metadata,
audit events, navigation state, and settings.

## Local SQLite

| Item           | Value                                                                         |
|----------------|-------------------------------------------------------------------------------|
| Driver         | `KODE_STREAM_STORAGE_DRIVER=sqlite`                                           |
| Default file   | `<KODE_STREAM_DATA_DIR>/kode-stream.db`                                       |
| Override file  | `KODE_STREAM_SQLITE_PATH=/path/to/kode-stream.db`                             |
| Migration mode | `KODE_STREAM_MIGRATIONS=auto` by default                                      |
| Legacy import  | Existing YAML and JSONL files under `KODE_STREAM_DATA_DIR` are imported once. |

Back up SQLite by stopping Kode Stream and copying `kode-stream.db`. For a portable export, use `sqlite3
kode-stream.db .dump > kode-stream.sql`. Restore by stopping Kode Stream, replacing the SQLite file or importing the
dump into a new file, then starting the app and checking `/api/health`.

Legacy source files such as `workspaces.yaml`, `item-index.yaml`, `audit-log.jsonl`, `saved-filters.yaml`,
`recent-items.yaml`, and `ai-settings.yaml` are left untouched after import. Import completion is recorded in SQL.

## Cloud Postgres

Cloud mode must set:

| Variable                              | Purpose                                                                |
|---------------------------------------|------------------------------------------------------------------------|
| `KODE_STREAM_STORAGE_DRIVER=postgres` | Selects Postgres repositories.                                         |
| `KODE_STREAM_DATABASE_URL`            | Secret-managed Postgres connection URL.                                |
| `KODE_STREAM_MIGRATIONS`              | `auto` for startup migrations or `manual` for operator-run migrations. |

Provision Postgres with encrypted storage, regular backups, and network access limited to the Kode Stream API and
approved operator tooling. Cloud Agents never connect to Postgres; they connect only to the Cloud API over HTTPS and
WebSocket.

For manual migrations, run the Kode Stream binary once with `KODE_STREAM_MIGRATIONS=auto` against the target database
from an operator job, then deploy the API with `KODE_STREAM_MIGRATIONS=manual`. `/api/health` reports database
connectivity and `migrationVersion`.

Back up Postgres with the managed provider snapshot feature or `pg_dump`. Restore from a snapshot or logical dump,
then verify `/api/health`, workspace listing, branch load, and audit event reads.
