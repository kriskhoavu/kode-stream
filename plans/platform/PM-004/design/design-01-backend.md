# Backend Design: Reliability, Safety, And Observability

## Overview

PM-004 adds backend support for audit events, workspace health checks, operation results, and better stale-file feedback. It builds on `internal/application`, `internal/security/pathguard`, `internal/fileaccess`, and `internal/gitadapter`.

## Data Model

### Entity: AuditEvent

| Field         | Type        | Purpose                                            |
|---------------|-------------|----------------------------------------------------|
| `id`          | `string`    | Stable event ID                                    |
| `time`        | `time.Time` | Event time in UTC                                  |
| `workspaceId` | `string`    | Workspace that the event belongs to                |
| `itemId`      | `string`    | Optional item ID                                   |
| `operation`   | `string`    | `scan`, `save_file`, `save_metadata`, `git_commit` |
| `status`      | `string`    | `success`, `blocked`, or `failed`                  |
| `message`     | `string`    | Short result message                               |
| `paths`       | `[]string`  | Related workspace-relative paths                   |
| `durationMs`  | `int64`     | Operation duration                                 |
| `error`       | `string`    | Error text for failed events                       |

### Entity: WorkspaceHealth

| Field         | Type            | Purpose                      |
|---------------|-----------------|------------------------------|
| `workspaceId` | `string`        | Workspace ID                 |
| `checkedAt`   | `time.Time`     | Check time                   |
| `checks`      | `HealthCheck[]` | Ordered checks               |
| `summary`     | `string`        | `ok`, `warning`, or `failed` |

### Entity: HealthCheck

| Field          | Type     | Purpose                      |
|----------------|----------|------------------------------|
| `name`         | `string` | Check key                    |
| `status`       | `string` | `ok`, `warning`, or `failed` |
| `message`      | `string` | User-facing explanation      |
| `recoveryHint` | `string` | Suggested next step          |

## Storage

```text
<user-config-dir>/kode-stream/
  audit-log.jsonl
  workspaces.yaml
  item-index.yaml
```

The audit log is append-only. A read API returns recent events. A later ticket can add pruning.

## API Contract

| Method | Endpoint                         | Request | Response                      |
|--------|----------------------------------|---------|-------------------------------|
| GET    | `/api/audit-events`              | query   | `AuditEvent[]`                |
| GET    | `/api/workspaces/{id}/health`    | none    | `WorkspaceHealth`             |
| POST   | existing write and Git endpoints | current | current plus hints where safe |

## Services

| Service           | Responsibility                                    |
|-------------------|---------------------------------------------------|
| `audit.Service`   | Append and read local audit events                |
| `health.Service`  | Run workspace path, source, Git, and index checks |
| `safety.Service`  | Centralize preflight checks for risky operations  |
| Existing services | Call audit and safety services around operations  |

## Design Decisions

| Decision                         | Rationale                                                |
|----------------------------------|----------------------------------------------------------|
| Keep audit service independent   | Application services can log without coupling to storage |
| Add health as read-only endpoint | Users can diagnose problems without changing workspaces  |
| Keep current errors stable       | Existing frontend behavior should not regress            |
| Add hints incrementally          | Each operation can improve feedback without route churn  |

