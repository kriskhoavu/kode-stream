# Backend Design: PM-002

## Overview

PM-002 adds guarded write operations to the local Plan Manager backend.

The backend remains the only layer that can write to registered repositories. It validates repository IDs, plan IDs, file IDs, and paths before each write. It runs Git through the Git adapter and returns clear status results to the frontend.

## Data Model

### Request And Response Types

| Type                      | Fields                                                                    | Purpose                                |
|---------------------------|---------------------------------------------------------------------------|----------------------------------------|
| `PlanFileUpdateInput`     | `content`, `expectedHash`                                                 | Save one editable file                 |
| `PlanMetadataUpdateInput` | `title`, `status`, `owner`, `tags`, `documents`                           | Update structured plan metadata        |
| `PlanStatusUpdateInput`   | `status`                                                                  | Move a plan across Kanban columns      |
| `PlanCreateInput`         | `planDirectory`, `service`, `ticket`, `title`, `status`, `owner`, `tags`  | Create a structured plan               |
| `RepositorySettings`      | `version`, `cards`, `pathPattern`, field mappings                         | Configure source directory card discovery |
| `GitStatus`               | `branch`, `dirty`, `conflicted`, `changes`, `ahead`, `behind`, `upstream` | Describe current Git state             |
| `GitCommitInput`          | `message`, `paths`                                                        | Commit selected plan paths             |
| `GitOperationResult`      | `ok`, `message`, `status`, `output`                                       | Return a Git operation result          |
| `BranchCreateInput`       | `name`, `startPoint`                                                      | Create a branch                        |
| `BranchSwitchInput`       | `name`, `confirmDirty`                                                    | Switch branch with a dirty-state guard |

### Git Change

| Field    | Type     | Purpose                                     |
|----------|----------|---------------------------------------------|
| `path`   | `string` | Repository-relative changed path            |
| `status` | `string` | Short Git status such as `M`, `A`, `??`     |
| `staged` | `bool`   | Shows whether the change is staged          |
| `planId` | `string` | Matching plan ID when the path is in a plan |

## API Contract

| Method  | Endpoint                              | Request                   | Response             |
|---------|---------------------------------------|---------------------------|----------------------|
| `POST`  | `/api/plans/{id}/files/{fileID}`      | `PlanFileUpdateInput`     | `FileContent`        |
| `PATCH` | `/api/plans/{id}/metadata`            | `PlanMetadataUpdateInput` | `PlanDetail`         |
| `PATCH` | `/api/plans/{id}/status`              | `PlanStatusUpdateInput`   | `PlanSummary`        |
| `POST`  | `/api/repositories/{id}/plans`        | `PlanCreateInput`         | `PlanDetail`         |
| `GET`   | `/api/repositories/{id}/source-settings?directory={dir}` | none       | `SourceSettingsResult` |
| `PUT`   | `/api/repositories/{id}/source-settings?directory={dir}` | `RepositorySettings` | `SourceSettingsResult` |
| `GET`   | `/api/repositories/{id}/git/status`   | none                      | `GitStatus`          |
| `POST`  | `/api/repositories/{id}/git/fetch`    | none                      | `GitOperationResult` |
| `POST`  | `/api/repositories/{id}/git/pull`     | confirmation payload      | `GitOperationResult` |
| `POST`  | `/api/repositories/{id}/git/push`     | confirmation payload      | `GitOperationResult` |
| `POST`  | `/api/repositories/{id}/git/commit`   | `GitCommitInput`          | `GitOperationResult` |
| `POST`  | `/api/repositories/{id}/git/branches` | `BranchCreateInput`       | `GitOperationResult` |
| `POST`  | `/api/repositories/{id}/git/switch`   | `BranchSwitchInput`       | `GitOperationResult` |

## Services

| Service             | Responsibility                                                         |
|---------------------|------------------------------------------------------------------------|
| Safe file writer    | Resolves file IDs and writes content inside the plan root only         |
| Metadata writer     | Updates or creates `plan.yaml` for structured plans                    |
| Source settings     | Reads, validates, writes, and scans `repository-settings.yaml`         |
| Plan creator        | Creates starter folders and documents for new structured plans         |
| Git operation guard | Checks dirty state, conflicts, divergence, and selected path scope     |
| Scan refresher      | Rescans the affected repository after metadata, status, new-plan, or Git content changes |

## Write Guard Rules

- Resolve repository by ID before any write.
- Resolve plan by ID from the index before any plan write.
- Resolve file ID through the existing file tree or document list.
- Reject absolute paths and `..` paths.
- Reject symlink escapes.
- Reject writes outside configured plan directories.
- Reject status and metadata edits for freestyle docs roots unless they are configured source cards.
- Reject branch switch, pull, and push when the repository has conflicts.
- Require explicit confirmation for dirty branch switch, dirty pull, and divergent push.
- Never run `git reset`, `git clean`, `git push --force`, or checkout commands that discard changes.

## Git Adapter Additions

Add methods for:

- `Status(repoPath)`
- `Fetch(repoPath)`
- `Pull(repoPath)`
- `Push(repoPath)`
- `Commit(repoPath, message, paths)`
- `CreateBranch(repoPath, name, startPoint)`
- `SwitchBranch(repoPath, name)`
- `ListBranches(repoPath)` remains from PM-001.

Each method uses a timeout and returns stdout or stderr as a concise message.

## Rescan Behavior

- File save returns updated `FileContent` and hash without a full repository rescan.
- Metadata save rescans the repository.
- Status move rescans the repository.
- New plan creation rescans the repository.
- Commit rescans after success.
- Pull and branch switch rescan after success.
- Fetch and push update Git status but do not require a full rescan unless refs changed in a way that affects branch metadata.

## Design Decisions

| Decision                        | Rationale                                                             |
|---------------------------------|-----------------------------------------------------------------------|
| Keep writes in backend services | The backend can enforce path and Git safety consistently.             |
| Reuse scanner after writes      | The scanner is the source of truth for summaries, docs, and warnings. |
| Use selected paths for commits  | Users should not accidentally commit unrelated repository changes.    |
| Create `plan.yaml` when needed  | Status and metadata edits need a stable structured metadata target.   |
| Return Git status with failures | The UI needs actionable context after failed Git operations.          |
