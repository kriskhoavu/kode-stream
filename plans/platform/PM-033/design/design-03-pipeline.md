# Pipeline Design: Configurable App-State Storage For Kode Stream

## Overview

The pipeline verifies storage-option resolution, local database and datadir repositories, manual sync, branch
re-indexing, and Cloud Postgres readiness. SQLite and datadir checks run in every build. Postgres checks run when a test
database is available.

## Verification Stages

| Stage                | Command Or Check                                                  | Purpose                                       |
|----------------------|-------------------------------------------------------------------|-----------------------------------------------|
| Unit tests           | `rtk go test ./internal/...`                                      | Repository interfaces and service behavior.   |
| Storage integration  | `rtk go test ./internal/storage/...`                              | Option resolution, SQLite, datadir, and sync. |
| Branch re-index      | `rtk go test ./internal/workstream/... ./internal/git/...`        | Stale branch detection and refresh behavior.  |
| Frontend contract    | `rtk npm run typecheck`                                           | Storage settings API type usage.              |
| Frontend tests       | `rtk npm test -- --run`                                           | Settings and existing UI flows.               |
| Postgres integration | `KODE_STREAM_DATABASE_URL=... rtk go test ./internal/storage/...` | Postgres dialect and migration behavior.      |
| Cloud smoke          | Start Cloud API with Postgres and verify `/api/health`.           | Deployment readiness.                         |

## Release Gates

| Gate           | Requirement                                                                          |
|----------------|--------------------------------------------------------------------------------------|
| Option gate    | Local accepts `database` and `datadir`; Cloud rejects `datadir`.                     |
| Migration gate | SQLite and Postgres schemas reach the expected migration version.                    |
| Sync gate      | Manual sync copies app-owned state in both directions and writes target backups.     |
| Branch gate    | Branch switch and branch load refresh stale rows, then return board data.            |
| API gate       | Existing route contract tests pass against the configured repositories.              |
| Cloud gate     | Cloud mode fails fast without Postgres and passes health with Postgres.              |
| Security gate  | Tests confirm app-state storage excludes SSH keys, token values, and source content. |

## Test Data

| Fixture Type       | Content                                                                  |
|--------------------|--------------------------------------------------------------------------|
| Workspace registry | Multiple workspaces with different sources and selected branches.        |
| Branch index       | Same item identifier with different title/status on different branches.  |
| Scan metadata      | Matching commit, changed commit, changed source hash, changed tree hash. |
| Audit log          | Success, blocked, failed, and workspace-filtered events.                 |
| Navigation state   | Saved filters and recent item entries.                                   |
| Storage sync       | Database and datadir state with replaceable target backups.              |
| Cloud metadata     | Users, agents, Cloud workspaces, and published summaries.                |

## Pipeline Behavior

SQLite and datadir checks are mandatory in normal CI. Postgres is mandatory for Cloud release workflows and optional for
local developer checks when no database is configured.

| Environment       | Local Storage Checks | Postgres Checks | Cloud Smoke |
|-------------------|----------------------|-----------------|-------------|
| Local developer   | yes                  | optional        | optional    |
| Pull request      | yes                  | optional        | no          |
| Cloud release     | yes                  | yes             | yes         |
| Production deploy | migration check      | yes             | yes         |
