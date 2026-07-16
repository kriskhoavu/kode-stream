# Infrastructure Design: Database Storage For Kode Stream

## Overview

PM-033 adds database infrastructure for both runtime modes. Local mode ships with SQLite and requires no external
service. Cloud mode requires Postgres and treats database readiness as a deployment gate.

## Local Mode

| Item             | Requirement                                 |
|------------------|---------------------------------------------|
| Database engine  | SQLite                                      |
| Default location | `<KODE_STREAM_DATA_DIR>/kode-stream.db`     |
| Backup target    | Single SQLite file plus optional SQL export |
| Migration mode   | Embedded migrations at process startup      |
| Operator setup   | None for normal local use                   |

SQLite runs in-process with the Kode Stream server. Local mode should continue to work during install and upgrade of the
single application binary.

## Cloud Mode

| Item            | Requirement                                        |
|-----------------|----------------------------------------------------|
| Database engine | Postgres                                           |
| Connection      | `KODE_STREAM_DATABASE_URL` from deployment secrets |
| Migration mode  | Startup auto-migrate or operator-run migration job |
| Backup target   | Managed Postgres backup or scheduled logical dump  |
| Health gate     | API reports unavailable when DB is unreachable     |

Cloud mode stores users, roles, agents, workspace metadata, branch indexes, audit events, settings, and published
summaries in Postgres. It does not store repository source trees, Git credentials, or command execution state.

## Runtime Configuration

| Setting                      | Local Example          | Cloud Example                  |
|------------------------------|------------------------|--------------------------------|
| `KODE_STREAM_STORAGE_DRIVER` | `sqlite`               | `postgres`                     |
| `KODE_STREAM_SQLITE_PATH`    | `/data/kode-stream.db` | unset                          |
| `KODE_STREAM_DATABASE_URL`   | unset                  | secret-managed Postgres URL    |
| `KODE_STREAM_MIGRATIONS`     | `auto`                 | `auto` or `manual`             |
| `KODE_STREAM_DATA_DIR`       | OS app data dir        | mounted metadata/export volume |

## Container Changes

| Component         | Change                                                                   |
|-------------------|--------------------------------------------------------------------------|
| Cloud image       | Include database driver support and embedded migration files.            |
| Cloud environment | Require Postgres URL, storage driver, public URL, OIDC, and secrets.     |
| Healthcheck       | Verify HTTP health includes database readiness.                          |
| Metadata volume   | Keep exports, import markers, diagnostics, and optional local artifacts. |

## Deployment Topology

Cloud topology: reverse proxy -> Kode Stream Cloud API -> Postgres -> outbound Cloud Agent channels to user machines.

Postgres must be reachable only by the Cloud API deployment and approved operator tooling. Cloud Agents do not connect
directly to Postgres.

## Migration Operations

| Operation         | Local Mode                                               | Cloud Mode                                      |
|-------------------|----------------------------------------------------------|-------------------------------------------------|
| Initial setup     | Create SQLite file and run migrations.                   | Run migrations against Postgres.                |
| App-state import  | Import YAML/JSONL files from data directory.             | Import only operator-provided metadata exports. |
| Backup            | Copy SQLite file while app is stopped or use backup API. | Managed backup or logical dump.                 |
| Restore           | Replace SQLite file or import SQL dump.                  | Restore Postgres snapshot or dump.              |
| Rollback artifact | Original app-state files remain untouched.               | Database snapshot and migration version.        |

## Security Controls

| Control                  | Requirement                                                              |
|--------------------------|--------------------------------------------------------------------------|
| Database credentials     | Stored in deployment secret manager, never in repository files.          |
| Repository paths         | Store redacted labels in Cloud, not executable host paths.               |
| Source content           | Do not persist raw source files or terminal transcripts.                 |
| Cloud Agent scan results | Persist derived item metadata, warnings, and branch scan freshness only. |
| Network access           | Postgres accepts Cloud API traffic, not browser or Cloud Agent traffic.  |
| Backups                  | Encrypt at rest and restrict restore access to operators.                |

## Design Decisions

| Decision                             | Rationale                                                                     |
|--------------------------------------|-------------------------------------------------------------------------------|
| Ship SQLite without external setup   | Local users should keep the current install simplicity.                       |
| Require Postgres for Cloud           | Cloud needs durable state, concurrent writes, and operational backup support. |
| Keep Cloud Agents away from Postgres | Agents should only talk to Cloud API over the PM-032 outbound channel.        |
| Keep metadata volume in Cloud        | Operators still need import/export files, diagnostics, and backup artifacts.  |
| Support manual migration mode        | Operators can control schema changes in stricter production environments.     |
