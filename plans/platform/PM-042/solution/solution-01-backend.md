# Backend Design: PM-042

## Overview

PM-042 adds two read paths on top of the existing session record and Git adapter, and no new entity. `SessionChanges`
computes what the checkout has changed since a session's launch commit. `SessionRecordsAcrossWorkspaces` lists records
for every workspace the caller may read. Both are served by a new `sessions` controller that reuses the AI controller's
workspace authorization. Cancel keeps using the existing embedded-session endpoint.

Nothing is persisted. The session record already carries `observedCommit`; every other value is computed from Git at
request time and bounded the same way existing tree reads are.

## Data Model

### Value: Session Changes (computed, not stored)

| Field        | Type             | Source                                                                                                 |
|--------------|------------------|--------------------------------------------------------------------------------------------------------|
| sessionId    | string           | Record id.                                                                                             |
| launchCommit | string           | `record.ObservedCommit`.                                                                               |
| headCommit   | string           | Current checkout HEAD.                                                                                 |
| headMoved    | int              | `rev-list --count launch..HEAD`; 0 when equal.                                                         |
| files        | []SessionFile    | `diff --name-status launch` plus untracked from status.                                                |
| additions    | int              | `diff --shortstat launch` including working tree.                                                      |
| deletions    | int              | Same.                                                                                                  |
| truncated    | bool             | True when the file list hit the 500-entry cap.                                                         |
| availability | string           | `available`, `unavailable`, `unsupported` with `reason`.                                               |
| reason       | string, optional | `launch_commit_unknown`, `launch_commit_missing`, `workspace_missing`, `content_provider_unsupported`. |

### Value: Session File

| Field   | Type   | Purpose                                        |
|---------|--------|------------------------------------------------|
| path    | string | Repository-relative path.                      |
| status  | string | `A`, `M`, `D`, `R`, `C`, or `?` for untracked. |
| oldPath | string | Set for renames and copies.                    |

### Value: Global Session Row

| Field          | Type   | Source                                                                                       |
|----------------|--------|----------------------------------------------------------------------------------------------|
| record         | view   | `ai.SessionRecordView` including `live`.                                                     |
| workspaceName  | string | Registry.                                                                                    |
| checkoutBranch | string | Current branch of the workspace, best effort.                                                |
| planTitle      | string | Item index lookup by `planRef.itemId`, best effort.                                          |
| changesCount   | int    | Count of `files` when changes are available; else omitted. Computed only for `scope=active`. |

## Git Adapter Additions

| Method          | Behaviour                                                                                        |
|-----------------|--------------------------------------------------------------------------------------------------|
| NameStatusSince | `git diff --name-status -M <commit> --` in the workspace path; refs validated, no options.       |
| ShortStatSince  | `git diff --shortstat <commit> --`.                                                              |
| DiffFileSince   | `git diff <commit> -- <path>` bounded by the existing byte limit; path validated as a tree path. |
| RevListCount    | `git rev-list --count <a>..<b>`.                                                                 |
| CommitExists    | `git cat-file -e <commit>^{commit}`.                                                             |

All are reads and take the context variants the adapter already uses. Refs must be non-empty, must not start with `-`,
and are resolved with `rev-parse --verify` before use.

## Services

| Method                                      | Behaviour                                                                                                                                                                                                                                          |
|---------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ai.Service.SessionChanges`                 | Resolve record and workspace; `unsupported` unless content provider is a Local checkout; `unavailable` when the path is missing or the launch commit is empty or absent; otherwise compute the value above.                                        |
| `ai.Service.SessionFileDiff`                | Same guards, then `DiffFileSince`.                                                                                                                                                                                                                 |
| `ai.Service.SessionRecordsAcrossWorkspaces` | For each registered workspace the caller may read: `SessionRecords(workspace, "")` with live flags; `scope=active` keeps starting, running, and live only; annotate workspace name, checkout branch, plan title. Bounded to 200 rows newest first. |

## API Endpoints

| Method | Endpoint                     | Request       | Response                                                                 |                                           |
|--------|------------------------------|---------------|--------------------------------------------------------------------------|-------------------------------------------|
| GET    | `/api/sessions`              | `scope=active | all`                                                                     | `{ rows: GlobalSessionRow[], truncated }` |
| GET    | `/api/sessions/{id}/changes` | —             | SessionChanges; `404` when the caller may not read the workspace         |                                           |
| GET    | `/api/sessions/{id}/diff`    | `path`        | `{ path, diff, truncated }`; `400 session_request_invalid` on a bad path |                                           |

Existing endpoints reused unchanged: `GET /api/ai/session-records?workspaceId&branch`, `DELETE /api/ai/sessions/{id}`.

Route families: `/api/sessions*` join the `ai` owner and the read capability; cancel stays where it is.

## Design Decisions

| Decision                                                  | Rationale                                                                                 |
|-----------------------------------------------------------|-------------------------------------------------------------------------------------------|
| Compute changes on request, persist nothing               | The record already has the launch commit; storing diffs would duplicate Git and go stale. |
| `unsupported` for non-Local content providers             | Same axis rule as PM-037; no branching on deployment names.                               |
| Global list authorizes per workspace and never 403s       | Mirrors the AI controller's oracle-avoidance: a denied workspace is simply absent.        |
| Changes count only for `scope=active`                     | One Git status per active session is cheap; doing it for history is not.                  |
| Cap file list at 500 and diffs at the existing byte bound | Same limits the tree reader already applies; the UI shows a truncation note.              |
