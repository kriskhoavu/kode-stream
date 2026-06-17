# Plan Manager Architecture

This document describes the current PM-002 architecture.

Plan Manager is a local web app. A Go server exposes a JSON API and serves embedded React assets. The backend scans registered Git repositories, caches plan metadata in YAML files, serves plan data, writes selected Markdown and metadata files, and runs guarded Git operations.

## Goals

- Run locally on the developer machine.
- Keep planning files in Git.
- Show one active repository workspace at a time.
- Support multiple plan directories per repository.
- Support structured plans and freestyle docs.
- Allow explicit, guarded writes to managed repositories.
- Keep app registry and cache outside registered repositories.

## System Context

```text
User browser
  -> http://127.0.0.1:4317
  -> Go local server
  -> JSON API
  -> Repository registry and plan index in user config dir
  -> Registered local Git repositories
```

## Runtime Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│ Browser                                                      │
│                                                              │
│ React app                                                    │
│ - App shell                                                  │
│ - Kanban board                                               │
│ - Repository management                                      │
│ - Plan workspace                                             │
└──────────────────────────────┬───────────────────────────────┘
                               │ HTTP JSON
┌──────────────────────────────▼───────────────────────────────┐
│ Go server on 127.0.0.1                                       │
│                                                              │
│ internal/app                                                 │
│ - serves embedded frontend assets                            │
│ - mounts /api routes                                         │
│                                                              │
│ internal/api                                                 │
│ - validates requests                                         │
│ - coordinates registry, scanner, index, files, Git           │
└───────────────┬───────────────────────────────┬──────────────┘
                │                               │
┌───────────────▼────────────────┐  ┌───────────▼─────────────────┐
│ User config directory          │  │ Registered Git repositories │
│                                │  │                             │
│ repositories.yaml              │  │ plans/                      │
│ plan-index.yaml                │  │ docs/                       │
└────────────────────────────────┘  └─────────────────────────────┘
```

## Backend Components

| Component      | Package                 | Responsibility                                                  |
|----------------|-------------------------|-----------------------------------------------------------------|
| CLI entrypoint | `cmd/plan-manager`      | Parses `serve` command and port flag                            |
| Server         | `internal/app`          | Resolves app paths, wires dependencies, serves API and frontend |
| API            | `internal/api`          | Defines HTTP routes and response handling                       |
| Config         | `internal/config`       | Resolves OS user config path                                    |
| Registry       | `internal/registry`     | Stores registered repositories in `repositories.yaml`           |
| Plan index     | `internal/planindex`    | Stores cached scan results in `plan-index.yaml`                 |
| Scanner        | `internal/scanner`      | Reads plan directories and builds plan metadata                 |
| File access    | `internal/fileaccess`   | Builds file trees and reads or writes files inside plan roots   |
| Plan writer    | `internal/planwriter`   | Writes Markdown, metadata, status changes, and new plans        |
| Git adapter    | `internal/gitadapter`   | Runs Git status and guarded Git commands with timeout           |
| System dialog  | `internal/systemdialog` | Opens native folder picker and reveals local paths              |
| Models         | `internal/models`       | Defines shared backend data structures                          |

## Frontend Components

| Component           | Path                                   | Responsibility                                                    |
|---------------------|----------------------------------------|-------------------------------------------------------------------|
| App shell           | `web/src/App.tsx`                      | Layout, navigation, workspace selector, refresh state             |
| API client          | `web/src/lib/api.ts`                   | Fetch wrapper and typed API calls                                 |
| Shared types        | `web/src/lib/types.ts`                 | Frontend API types                                                |
| Kanban page         | `web/src/pages/KanbanPage.tsx`         | Board, filters, cards, preview drawer                             |
| Repository page     | `web/src/pages/RepositoriesPage.tsx`   | Repository create, edit, delete, scan, reveal                     |
| Plan workspace page | `web/src/pages/PlanWorkspacePage.tsx`  | File tree, preview, Markdown editor, diff, metadata, Git controls |
| Plans page          | `web/src/pages/PlansPage.tsx`          | Searchable list view for active workspace                         |
| Branches page       | `web/src/pages/BranchesPage.tsx`       | Branch summary inferred from indexed plans                        |
| Error boundary      | `web/src/components/ErrorBoundary.tsx` | Catches frontend render failures                                  |
| Styles              | `web/src/styles/app.css`               | Application layout and responsive UI                              |

## Data Flow

### Repository Registration

```text
User creates repository
  -> POST /api/repositories
  -> registry validates name, path, baseline branch, plan directories
  -> Git adapter resolves repository root and validates branch
  -> registry writes repositories.yaml
  -> frontend refreshes repository list
```

### Scan

```text
User clicks Scan
  -> POST /api/repositories/{id}/scan
  -> API loads repository config
  -> scanner reads each configured plan directory
  -> scanner parses plan.yaml or fallback README/folder metadata
  -> scanner reads Git author and update time
  -> plan index replaces that repository's cached plans
  -> registry updates lastScannedAt
  -> frontend reloads plans
```

### Plan Detail

```text
User opens plan
  -> GET /api/plans/{id}
  -> GET /api/plans/{id}/files
  -> GET /api/plans/{id}/files/{fileID}
  -> GET /api/plans/{id}/diff
  -> workspace renders file tree, preview, raw file, info, and diff
```

### Plan Editing

```text
User edits Markdown or metadata
  -> frontend tracks dirty state
  -> user clicks Save
  -> API validates repository, plan, file ID, and path scope
  -> fileaccess or planwriter writes the repository file
  -> scanner rescans the affected repository
  -> plan index and /api/state version update
  -> frontend refreshes current data and other tabs show stale-content notice
```

### Git Operations

```text
User opens Git panel
  -> GET /api/repositories/{id}/git/status
  -> frontend shows branch, ahead/behind, dirty files, conflicts, and path selection
  -> user commits selected paths or runs fetch/pull/push/branch operation
  -> API validates branch names, commit message, path scope, and dirty-state guards
  -> Git adapter runs the command with timeout
  -> operations that change content rescan the affected repository
```

### Stale Content Detection

```text
Frontend polls /api/state
  -> backend hashes repositories and plan summaries
  -> version changes after registry or index changes
  -> another open tab shows refresh popup
  -> user refreshes app data in place
```

## Storage Design

Plan Manager does not use a database server. It uses YAML files in the OS user config directory.

```text
<user-config-dir>/plan-manager/
  repositories.yaml
  plan-index.yaml
```

### repositories.yaml

Stores registered repository configuration.

| Field             | Type       | Purpose                                       |
|-------------------|------------|-----------------------------------------------|
| `id`              | `string`   | Stable app ID derived from name and root path |
| `name`            | `string`   | Display name                                  |
| `path`            | `string`   | Absolute Git repository root                  |
| `baselineBranch`  | `string`   | Baseline branch validated at registration     |
| `planDirectories` | `string[]` | Relative roots such as `plans` or `docs`      |
| `createdAt`       | `string`   | Creation timestamp                            |
| `lastScannedAt`   | `string`   | Last successful scan timestamp                |

Example:

```yaml
- id: discovery-9409b56c
  name: discovery
  path: /workspace/discovery
  baselineBranch: master
  planDirectories:
    - plans
    - docs
  createdAt: 2026-06-16T18:21:48Z
  lastScannedAt: 2026-06-17T09:18:05Z
```

### plan-index.yaml

Stores cached plan details, scan warnings, and scan timestamps.

| Field      | Type            | Purpose                                      |
|------------|-----------------|----------------------------------------------|
| `plans`    | `PlanDetail[]`  | Cached plan details and document metadata    |
| `warnings` | `ScanWarning[]` | Non-fatal scan warnings                      |
| `scans`    | `object`        | Repository ID to last scan timestamp mapping |

The plan index is derived data. It can be rebuilt by scanning repositories again.

## Plan Data Model

### RepositoryConfig

| Field             | Type       | Description                    |
|-------------------|------------|--------------------------------|
| `id`              | `string`   | Repository ID                  |
| `name`            | `string`   | Display name                   |
| `path`            | `string`   | Absolute repository root       |
| `baselineBranch`  | `string`   | Baseline branch                |
| `planDirectories` | `string[]` | Configured scan roots          |
| `createdAt`       | `string`   | Creation timestamp             |
| `lastScannedAt`   | `string`   | Last successful scan timestamp |

### PlanSummary

| Field            | Type       | Description                                       |
|------------------|------------|---------------------------------------------------|
| `id`             | `string`   | Stable plan ID                                    |
| `repositoryId`   | `string`   | Owning repository                                 |
| `repositoryName` | `string`   | Repository display name                           |
| `branch`         | `string`   | Current or ticket-matched branch                  |
| `service`        | `string`   | Service or docs root label                        |
| `ticket`         | `string`   | Ticket or docs item key                           |
| `title`          | `string`   | Display title                                     |
| `status`         | `string`   | `ideas`, `draft`, `in_progress`, `review`, `done` |
| `owner`          | `string`   | Metadata owner                                    |
| `author`         | `string`   | Last Git author or owner fallback                 |
| `tags`           | `string[]` | Plan tags                                         |
| `updatedAt`      | `string`   | Last Git update or filesystem time                |
| `description`    | `string`   | First README paragraph                            |
| `metadataSource` | `string`   | `plan.yaml`, `fallback`, or `docs`                |
| `planRoot`       | `string`   | Repository-relative plan root                     |

### PlanDetail

`PlanDetail` extends `PlanSummary`.

| Field       | Type             | Description                                      |
|-------------|------------------|--------------------------------------------------|
| `documents` | `PlanDocument[]` | Documents from `plan.yaml` or fallback discovery |
| `metadata`  | `object`         | Parsed plan metadata                             |
| `warnings`  | `ScanWarning[]`  | Plan-level warnings                              |
| `counts`    | `object`         | Workspace counts such as file count              |

## Plan Discovery Rules

Structured plan roots use:

```text
{planDirectory}/{service}/{ticket}/
```

A folder is treated as a structured plan when:

- It contains `plan.yaml`, or
- Its ticket folder matches an uppercase ticket pattern such as `DI-170`.

Freestyle docs roots are supported when:

- The configured root contains Markdown files, and
- It does not contain structured plan children.

Metadata precedence:

1. `plan.yaml`.
2. README heading and inferred status.
3. Folder names and fallback defaults.

Status normalization maps common values into:

- `ideas`
- `draft`
- `in_progress`
- `review`
- `done`

## API Endpoints

All endpoints are local and served from `http://127.0.0.1:{port}`.

| Method   | Endpoint                              | Description                                      |
|----------|---------------------------------------|--------------------------------------------------|
| `GET`    | `/api/health`                         | Health check                                     |
| `GET`    | `/api/state`                          | App state version, repository count, plan count  |
| `GET`    | `/api/repositories`                   | List registered repositories                     |
| `POST`   | `/api/repositories`                   | Create repository registration                   |
| `PUT`    | `/api/repositories/{id}`              | Update repository registration                   |
| `DELETE` | `/api/repositories/{id}`              | Delete repository registration and cached plans  |
| `POST`   | `/api/repositories/{id}/scan`         | Scan one repository                              |
| `GET`    | `/api/plans`                          | List cached plan summaries                       |
| `GET`    | `/api/plans/{id}`                     | Get plan detail                                  |
| `GET`    | `/api/plans/{id}/files`               | Get safe file tree for a plan                    |
| `GET`    | `/api/plans/{id}/files/{fileID}`      | Read one plan file                               |
| `POST`   | `/api/plans/{id}/files/{fileID}`      | Save one Markdown file                           |
| `GET`    | `/api/plans/{id}/diff`                | Get read-only Git diff for the plan root         |
| `PATCH`  | `/api/plans/{id}/metadata`            | Save structured plan metadata                    |
| `PATCH`  | `/api/plans/{id}/status`              | Move a structured plan to another status         |
| `POST`   | `/api/plans`                          | Create a structured plan                         |
| `GET`    | `/api/repositories/{id}/git/status`   | Read branch, ahead/behind, and change state      |
| `POST`   | `/api/repositories/{id}/git/fetch`    | Fetch remotes                                    |
| `POST`   | `/api/repositories/{id}/git/pull`     | Pull with dirty-state guard                      |
| `POST`   | `/api/repositories/{id}/git/push`     | Push current branch                              |
| `POST`   | `/api/repositories/{id}/git/commit`   | Commit selected plan paths                       |
| `POST`   | `/api/repositories/{id}/git/branches` | Create a branch                                  |
| `POST`   | `/api/repositories/{id}/git/switch`   | Switch branch with dirty-state guard             |
| `POST`   | `/api/system/select-directory`        | Open native directory picker                     |
| `POST`   | `/api/system/open-path`               | Reveal a local path in the platform file manager |

### Query Parameters

`GET /api/plans` supports:

| Parameter      | Description                                                  |
|----------------|--------------------------------------------------------------|
| `repositoryId` | Filter by repository ID                                      |
| `branch`       | Filter by branch                                             |
| `status`       | Filter by normalized status                                  |
| `q`            | Search title, ticket, service, description, author, and tags |

## API Payloads

### RepositoryInput

```json
{
  "name": "discovery",
  "path": "/workspace/discovery",
  "baselineBranch": "master",
  "planDirectories": ["plans", "docs"]
}
```

### ScanResult

```json
{
  "repositoryId": "discovery-9409b56c",
  "scannedAt": "2026-06-17T09:18:05Z",
  "planCount": 42,
  "warnings": []
}
```

### FileContent

```json
{
  "id": "README_md",
  "path": "README.md",
  "content": "# Example",
  "language": "markdown",
  "hash": "7c9f..."
}
```

## Security And Safety

PM-002 safety rules:

- Bind only to `127.0.0.1`.
- Do not expose authentication or remote access.
- Do not auto-save.
- Restrict reads and writes to configured plan directories.
- Resolve file IDs through the safe file tree or plan document list.
- Reject path traversal and symlink escapes.
- Use expected content hashes for Markdown saves.
- Keep freestyle docs roots Markdown-only.
- Stage and commit only selected paths inside configured plan directories.
- Block pull and branch switch on dirty working trees unless confirmed.
- Do not store Git credentials.
- Store app config and cache outside registered repositories.
- Validate repository roots through Git.
- Validate baseline branches at registration.
- Restrict file reads to configured plan directories.
- Reject invalid file paths and symlink escapes.
- Use short timeouts for Git commands.
- Do not store credentials.

PM-002 will add guarded write operations. See [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md).

## Performance Model

- Board and list views read cached plan summaries.
- File content loads only after a plan file is opened.
- Scans rebuild derived metadata for one repository.
- Large repository support depends on keeping Markdown content out of board-level queries.
- The target scale from PM-001 is 100 repositories, 10,000 plans, and 100,000 files.

## Build And Packaging

Production build flow:

```text
npm run build
  -> writes frontend assets to internal/app/frontend

go build -o ./bin/plan-manager ./cmd/plan-manager
  -> embeds internal/app/frontend
  -> produces one local binary
```

Runtime flow:

```text
./bin/plan-manager serve -port 4317
  -> resolves config paths
  -> opens or creates registry and index files
  -> serves API and embedded frontend
```

## PM-002 Extension Points

The PM-002 plan adds:

- Safe file writer.
- Metadata writer.
- New plan creator.
- Git status and operation APIs.
- Markdown editor UI.
- Metadata editor UI.
- Git operation controls.

The design keeps PM-001 read APIs stable and adds write APIs behind backend guards.
