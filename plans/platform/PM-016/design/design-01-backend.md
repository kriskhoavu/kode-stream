# Backend Design: Remote Workspace Registration

## Overview

The backend should support workspace creation from either an existing local Git path or a remote Git URL cloned onto the local machine. Existing scan, item, branch, and file APIs should continue using the workspace's local path.

## API Contract

Keep the same endpoint and expand request shape:

| Method | Endpoint          | Change |
|--------|-------------------|--------|
| POST   | `/api/workspaces` | Extend request body with registration mode and remote clone fields |
| PUT    | `/api/workspaces/{id}` | Keep current update behavior; remote metadata is read-only in first version |

### Proposed Request Fields

```json
{
  "name": "Platform Workspace",
  "baselineBranch": "main",
  "sources": ["plans", "docs"],
  "registrationMode": "local_path",
  "path": "/Users/me/repos/kode-stream",
  "remoteUrl": "git@bitbucket.org:team/repo.git",
  "cloneRoot": "/Users/me/workspaces"
}
```

- `registrationMode`: `local_path` or `remote_clone` (default `local_path`).
- `path`: required for `local_path`, ignored for `remote_clone` input validation.
- `remoteUrl` + `cloneRoot`: required for `remote_clone`.

## Service And Registry Design

| Component | Change |
|-----------|--------|
| `models.WorkspaceInput` | Add `registrationMode`, `remoteUrl`, `cloneRoot` |
| `models.WorkspaceConfig` | Add optional remote metadata (`registrationMode`, `remoteUrl`, `clonePathManaged`) |
| `workspace.Service.Create` | Resolve remote clone path and call clone service before registry create |
| `registry.validate` | Validate by mode; keep current source and baseline checks |
| `gitadapter.GitAdapter` | Add bounded clone helper (`Clone(remoteURL, destination)`) |

## Clone Workflow

```text
Create workspace request (remote_clone)
  -> validate URL and clone root
  -> derive destination path from repo slug
  -> ensure destination is safe and not already registered
  -> git clone (local machine credentials)
  -> resolve workspace root, baseline branch, and sources
  -> persist workspace config
```

## Safety Rules

- Reject clone roots outside normal local filesystem constraints.
- Reject clone destination if already registered or non-empty unexpectedly.
- Keep duplicate detection by canonical local path.
- If clone succeeds but validation fails, return explicit error and keep folder for manual inspection (no registry write).

## Tests

Add/update tests in:

- `internal/application/workspace/service_test.go`
- `internal/registry/registry_test.go`
- `internal/api/api_test.go`
- `internal/gitadapter/git_test.go`

Cover:

- local mode unchanged
- remote clone success
- invalid remote URL
- duplicate local path after clone
- baseline/sources validation failures
