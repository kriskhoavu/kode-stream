# Infrastructure Design: Configurable App-State Storage For Kode Stream

## Overview

PM-033 adds operator-selectable app-state storage. Local mode can run with a SQLite database or data-dir files. Cloud
mode runs with Postgres and treats database readiness as a deployment gate.

## Local Mode

| Item                   | Database Option                         | Datadir Option                   |
|------------------------|-----------------------------------------|----------------------------------|
| App-state store        | SQLite                                  | YAML/JSONL files                 |
| Default location       | `<KODE_STREAM_DATA_DIR>/kode-stream.db` | `KODE_STREAM_DATA_DIR` files     |
| Backup target          | SQLite file or SQL export               | Data-dir files                   |
| Migration requirement  | Embedded migrations at process startup  | File schema compatibility checks |
| Operator setup         | None for normal local use               | None for normal local use        |
| Settings switch timing | Restart required                        | Restart required                 |

Local storage option changes are read at process startup. Runtime writes go only to the selected backend.

## Cloud Mode

| Item            | Requirement                                          |
|-----------------|------------------------------------------------------|
| Storage option  | `database`                                           |
| Database engine | Postgres                                             |
| Connection      | `KODE_STREAM_DATABASE_URL` from deployment secrets   |
| Migration mode  | Startup auto-migrate or operator-run migration job   |
| Backup target   | Managed Postgres backup or scheduled logical dump    |
| Health gate     | API reports unavailable when database is unreachable |

Cloud mode stores users, roles, agents, workspace metadata, branch indexes, audit events, settings, and published
summaries in Postgres. It does not store repository source trees, Git credentials, or command execution state.

## Runtime Configuration

| Setting                      | Local Example              | Cloud Example                  |
|------------------------------|----------------------------|--------------------------------|
| `KODE_STREAM_STORAGE_OPTION` | `database` or `datadir`    | `database`                     |
| `KODE_STREAM_STORAGE_DRIVER` | unset or compatibility use | `postgres`                     |
| `KODE_STREAM_SQLITE_PATH`    | `/data/kode-stream.db`     | unset                          |
| `KODE_STREAM_DATABASE_URL`   | unset                      | secret-managed Postgres URL    |
| `KODE_STREAM_MIGRATIONS`     | `auto`                     | `auto` or `manual`             |
| `KODE_STREAM_DATA_DIR`       | OS app data dir            | mounted metadata/export volume |

## Container Changes

| Component         | Change                                                                    |
|-------------------|---------------------------------------------------------------------------|
| Cloud image       | Include database driver support and embedded migration files.             |
| Cloud environment | Require database storage option, Postgres URL, public URL, OIDC, secrets. |
| Healthcheck       | Verify HTTP health includes database readiness.                           |
| Metadata volume   | Keep exports, diagnostics, and optional local artifacts.                  |

## Deployment Topology

Cloud topology: reverse proxy -> Kode Stream Cloud API -> Postgres -> outbound Cloud Agent channels to user machines.

Postgres must be reachable only by the Cloud API deployment and approved operator tooling. Cloud Agents do not connect
directly to Postgres.

## Migration Operations

| Operation     | Local Database                                | Local Datadir                                 | Cloud Database                     |
|---------------|-----------------------------------------------|-----------------------------------------------|------------------------------------|
| Initial setup | Create SQLite file and run migrations.        | Create data-dir files.                        | Run migrations against Postgres.   |
| Manual sync   | Import from or export to data-dir files.      | Import from or export to SQLite.              | Not available.                     |
| Backup        | Copy SQLite file or use SQL export.           | Copy data-dir files.                          | Managed backup or logical dump.    |
| Restore       | Replace SQLite file or import SQL dump.       | Restore data-dir files.                       | Restore Postgres snapshot or dump. |
| Sync backup   | Timestamped backup before local sync replace. | Timestamped backup before local sync replace. | Database snapshot.                 |

## Security Controls

| Control                  | Requirement                                                              |
|--------------------------|--------------------------------------------------------------------------|
| Database credentials     | Stored in deployment secret manager, never in repository files.          |
| Repository paths         | Store redacted labels in Cloud, not executable host paths.               |
| Source content           | Do not persist raw source files or terminal transcripts.                 |
| Cloud Agent scan results | Persist derived item metadata, warnings, and branch scan freshness only. |
| Network access           | Postgres accepts Cloud API traffic, not browser or Cloud Agent traffic.  |
| Backups                  | Encrypt at rest and restrict restore access to operators.                |
