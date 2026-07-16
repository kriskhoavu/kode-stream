# Pipeline Design: Database Storage For Kode Stream

## Overview

The pipeline verifies SQL storage adapters, migrations, file import, branch re-indexing, and Cloud Postgres readiness.
Local SQLite checks run in every build. Postgres checks run when a test database is available.

## Verification Stages

| Stage                | Command Or Check                                                  | Purpose                                      |
|----------------------|-------------------------------------------------------------------|----------------------------------------------|
| Unit tests           | `rtk go test ./internal/...`                                      | Repository interfaces and service behavior.  |
| SQLite integration   | `rtk go test ./internal/storage/...`                              | SQLite migrations, transactions, import.     |
| Branch re-index      | `rtk go test ./internal/workstream/... ./internal/git/...`        | Stale branch detection and refresh behavior. |
| Frontend contract    | `rtk npm run typecheck`                                           | API type usage remains compatible.           |
| Frontend tests       | `rtk npm test -- --run`                                           | Existing UI flows keep working.              |
| Postgres integration | `KODE_STREAM_DATABASE_URL=... rtk go test ./internal/storage/...` | Postgres dialect and migration behavior.     |
| Cloud smoke          | Start Cloud API with Postgres and verify `/api/health`.           | Deployment readiness.                        |

## Release Gates

| Gate           | Requirement                                                                      |
|----------------|----------------------------------------------------------------------------------|
| Migration gate | SQLite and Postgres schemas reach the expected migration version.                |
| Import gate    | Existing YAML/JSONL fixtures import into SQL without changing source files.      |
| Branch gate    | Branch switch and branch load refresh stale rows, then return board data.        |
| API gate       | Existing route contract tests pass against SQL-backed services.                  |
| Cloud gate     | Cloud mode fails fast without Postgres and passes health with Postgres.          |
| Security gate  | Tests confirm SQL rows do not include SSH keys, token values, or source content. |

## Test Data

| Fixture Type       | Content                                                                  |
|--------------------|--------------------------------------------------------------------------|
| Workspace registry | Multiple workspaces with different sources and selected branches.        |
| Branch index       | Same item identifier with different title/status on different branches.  |
| Scan metadata      | Matching commit, changed commit, changed source hash, changed tree hash. |
| Audit log          | Success, blocked, failed, and workspace-filtered events.                 |
| Navigation state   | Saved filters and recent item entries.                                   |
| Cloud metadata     | Users, agents, Cloud workspaces, and published summaries.                |

## Pipeline Behavior

SQLite is mandatory in normal CI. Postgres is mandatory for Cloud release workflows and optional for local developer
checks when no database is configured.

| Environment       | SQLite Checks   | Postgres Checks | Cloud Smoke |
|-------------------|-----------------|-----------------|-------------|
| Local developer   | yes             | optional        | optional    |
| Pull request      | yes             | optional        | no          |
| Cloud release     | yes             | yes             | yes         |
| Production deploy | migration check | yes             | yes         |

## Design Decisions

| Decision                             | Alternatives Considered          | Rationale                                                             |
|--------------------------------------|----------------------------------|-----------------------------------------------------------------------|
| Run SQLite checks in every build     | Postgres-only SQL checks         | Local mode is the default install path and must stay dependency-free. |
| Gate Cloud releases on Postgres      | Allow file-backed Cloud fallback | PM-033 makes Postgres the Cloud storage contract.                     |
| Use fixture imports for migration QA | Manual migration testing only    | Existing local state needs repeatable compatibility coverage.         |
| Test branch-specific stale detection | Trust service-level scans        | Branch content can differ and stale SQL rows would be user-visible.   |
