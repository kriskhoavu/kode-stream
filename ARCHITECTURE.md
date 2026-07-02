# Plan Manager Architecture

This document describes the current architecture through PM-011.

Plan Manager is a local web app. A Go server exposes a JSON API and serves embedded React assets. The backend scans registered Git workspaces, caches item metadata in YAML files, serves item data, writes selected Markdown and metadata files, and runs guarded Git operations.

## Goals

- Run locally on the developer machine.
- Keep planning files in Git.
- Keep Kanban scoped to one active workspace while Explorer spans every registered workspace.
- Support multiple sources per workspace.
- Support structured items, configured document sources, and freestyle docs.
- Allow explicit, guarded writes to managed workspaces.
- Keep app registry and cache outside registered workspaces.

## System Context

```text
User browser
  -> http://127.0.0.1:4317
  -> Go local server
  -> JSON API
  -> Workspace registry and item index in user config dir
  -> Registered local Git workspaces (from local path or remote clone)
```

## Runtime Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│ Browser                                                      │
│                                                              │
│ React app                                                    │
│ - App shell                                                  │
│ - Kanban board                                               │
│ - Workspace management                                       │
│ - Item workspace                                             │
│ - Global workspace explorer                                  │
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
│ - validates HTTP requests                                    │
│ - delegates use cases to application services                │
│                                                              │
│ internal/application                                         │
│ - coordinates workspace, item, and Git workflows             │
└───────────────┬───────────────────────────────┬──────────────┘
                │                               │
┌───────────────▼────────────────┐  ┌───────────▼───────────────┐
│ User config directory          │  │ Registered Git workspaces │
│                                │  │                           │
│ workspaces.yaml                │  │ plans/                    │
│ item-index.yaml                │  │ docs/                     │
│ audit-log.jsonl                │  │                           │
│ saved-filters.yaml             │  │                           │
│ recent-items.yaml              │  │                           │
└────────────────────────────────┘  └───────────────────────────┘
```

## Backend Components

| Component              | Package                               | Responsibility                                                         |
|------------------------|---------------------------------------|------------------------------------------------------------------------|
| CLI entrypoint         | `cmd/plan-manager`                    | Parses `serve` command and port flag                                   |
| Server                 | `internal/app`                        | Resolves app paths, wires dependencies, serves API and frontend        |
| HTTP API               | `internal/api`                        | Defines routes, decodes requests, encodes responses                    |
| Application services   | `internal/application/*`              | Coordinates workspace, item, and Git use cases                         |
| App errors             | `internal/application/apperrors`      | Shared not-found sentinels across services and HTTP                    |
| Config                 | `internal/config`                     | Resolves OS user config path                                           |
| Registry               | `internal/registry`                   | Stores registered workspaces in `workspaces.yaml`                      |
| Item index             | `internal/itemindex`                  | Stores cached item scan results in `item-index.yaml`                   |
| Audit store            | `internal/audit`                      | Appends and reads local operation events in JSONL                      |
| Navigation store       | `internal/navigation`                 | Stores saved filters and recent items in YAML                          |
| Health service         | `internal/application/health`         | Runs read-only workspace, source, Git, and index checks                |
| Safety service         | `internal/application/safety`         | Centralizes write path and Git preflight decisions                     |
| Search service         | `internal/application/search`         | Ranks indexed item matches without scanning workspaces                 |
| Scanner                | `internal/scanner`                    | Reads sources and builds item metadata                                 |
| Scanner metadata       | `internal/scanner/metadata_*`         | Parses `plan.yaml` and document metadata                               |
| Scanner settings       | `internal/scanner/source_*`           | Matches source item settings to item folders                           |
| File access            | `internal/fileaccess`                 | Builds file trees, classifies bounded text reads, and writes Markdown  |
| Workspace file access  | `internal/workspacefiles`             | Lists, searches, creates, renames, and guards workspace paths          |
| Workspace file service | `internal/application/workspacefiles` | Coordinates file actions, Git state, audit, and targeted refresh       |
| Content search service | `internal/application/contentsearch`  | Resolves item, source, and full-workspace search scopes                |
| Item writer            | `internal/itemwriter`                 | Writes Markdown, metadata, status changes, and new items               |
| Path guard             | `internal/security/pathguard`         | Shared safe-join and configured-source path validation                 |
| Git adapter            | `internal/gitadapter`                 | Runs Git status and guarded Git commands with timeout                  |
| System dialog          | `internal/systemdialog`               | Opens native folder picker and reveals local paths                     |
| AI settings            | `internal/aisettings`                 | Stores provider and terminal launch templates outside workspaces       |
| AI session service     | `internal/application/aisession`      | Detects tools, validates items, creates context, and launches sessions |
| Models                 | `internal/models`                     | Defines stable API DTOs and shared data structures                     |

## Frontend Components

| Component                 | Path                                      | Responsibility                                                        |
|---------------------------|-------------------------------------------|-----------------------------------------------------------------------|
| App shell                 | `web/src/App.tsx`                         | Layout and navigation composition                                     |
| App state                 | `web/src/app/useAppState.ts`              | Workspace, theme, refresh, route, and stale-content state             |
| Router helpers            | `web/src/app/router.ts`                   | Browser path parsing and path generation                              |
| API facade                | `web/src/lib/api.ts`                      | Compatibility export for existing feature imports                     |
| Shared API implementation | `web/src/shared/api`                      | Fetch wrapper, endpoint methods, and response normalization           |
| Shared domain helpers     | `web/src/shared/domain`                   | Reusable diff parsing and domain helpers                              |
| Feature helpers           | `web/src/features/*`                      | Kanban filtering and workspace source settings helper logic           |
| Shared types              | `web/src/lib/types.ts`                    | Frontend API types                                                    |
| Reliability hooks         | `web/src/features/reliability`            | Workspace health and activity loading and refresh                     |
| Search hooks              | `web/src/features/search`                 | Debounced search, quick switcher, and keyboard navigation             |
| Content search            | `web/src/features/content-search`         | Targeted content query state, results, highlighting, and line context |
| Content viewer            | `web/src/features/content-viewer`         | Secure Markdown, HTML, JSON, YAML, code, and text rendering           |
| File editor session       | `web/src/features/file-editor`            | Shared Markdown autosave, stale-write, and settled-save state         |
| Workspace explorer        | `web/src/features/workspace-explorer`     | Lazy tree, path search, Git markers, mutations, and keyboard state    |
| Search dialog             | `web/src/components/SearchDialog.tsx`     | Global search, grouped results, and recent items                      |
| Kanban page               | `web/src/pages/KanbanPage.tsx`            | Board, cards, and preview drawer composition                          |
| Workspace page            | `web/src/pages/WorkspacesPage.tsx`        | Workspace create, edit, delete, scan, reveal                          |
| Item workspace page       | `web/src/pages/ItemWorkspacePage.tsx`     | File tree, preview, Markdown editor, diff, metadata, Git controls     |
| Explorer page             | `web/src/pages/WorkspaceExplorerPage.tsx` | Global filesystem tree, file editor, and context inspector            |
| Error boundary            | `web/src/components/ErrorBoundary.tsx`    | Catches frontend render failures                                      |
| Styles                    | `web/src/styles`                          | Global styles plus app-shell stylesheet                               |

## Dependency Rules

- `internal/api` owns HTTP details and delegates workflows to `internal/application`.
- `internal/application` packages coordinate use cases and may depend on registry, index, scanner, file access, writer, Git, and models.
- Infrastructure packages must not import `internal/api`.
- Shared path checks belong in `internal/security/pathguard`.
- Frontend pages may use feature and shared modules.
- `web/src/shared/*` must not import page modules.
- Search reads the item index and must not trigger workspace scans.
- Audit, saved filters, and recents stay in the user config directory.
- `web/src/lib/api.ts` remains a compatibility facade over `web/src/shared/api`.
- Workspace file content is untrusted. Rich renderers must sanitize output or render escaped React text.
- Standalone HTML must stay in an iframe sandbox without script or same-origin permissions.

## PM-003 Refactoring Notes

- Item write refresh now scans once and reuses the scan data to return the updated item.
- Scanner branch matching lists branches once per workspace scan instead of once per item identifier.
- Scanner source settings matching and metadata parsing are split into focused files behind the same `Scanner.Scan` facade.
- Frontend route and app state behavior moved out of `App.tsx`.
- Kanban filtering, workspace source settings helpers, and Git diff parsing moved into feature or shared modules.
- App shell CSS is split into `web/src/styles/app-shell.css` and imported by `app.css`.

## Data Flow

### Workspace Registration

```text
User creates workspace
  -> POST /api/workspaces
  -> registry validates name, baseline branch, local root path, and sources
  -> Git adapter resolves workspace root and validates branch
  -> registry writes workspaces.yaml
  -> frontend refreshes workspace list
```

### Scan

```text
User clicks Scan
  -> POST /api/workspaces/{id}/scan
  -> API loads workspace config
  -> scanner reads each configured source
  -> scanner reads workspace-settings.yaml when present
  -> scanner parses plan.yaml, configured source rules, or fallback README/folder metadata
  -> scanner reads Git author and update time
  -> item index replaces that workspace's cached items
  -> registry updates lastScannedAt
  -> frontend reloads items
```

### Item Detail

```text
User opens item
  -> GET /api/items/{id}
  -> GET /api/items/{id}/files
  -> GET /api/items/{id}/files/{fileID}
  -> GET /api/items/{id}/diff
  -> file access classifies text and applies response limits
  -> shared content viewer lazy-loads the matching safe renderer
  -> workspace renders file tree, preview, raw file, info, and diff
```

### Rich Content Preview

```text
Selected workspace file
  -> backend path and symlink guards
  -> binary detection, file kind, language, size, and bounded text response
  -> ContentViewer selects rendered, tree, or source mode
  -> Markdown uses sanitized GFM, KaTeX, and code highlighting
  -> HTML uses DOM sanitization, CSP, and an empty iframe sandbox
  -> JSON and YAML render as escaped bounded React trees
  -> source uses escaped syntax highlighting with copy and wrap controls
```

### Source Items Settings

```text
User opens a source's Source Items settings
  -> GET /api/workspaces/{id}/source-structure?directory={dir}
  -> API reads <dir>/<dir>/workspace-settings.yaml or returns defaults
  -> user saves a path pattern and field mapping
  -> PUT /api/workspaces/{id}/source-structure?directory={dir}
  -> API validates the pattern, writes workspace-settings.yaml, rescans the workspace
  -> configured source cards appear on the Kanban board
```

### Item Editing

```text
User edits Markdown or metadata
  -> frontend tracks dirty state
  -> Markdown autosaves after a short debounce, metadata saves explicitly
  -> API validates workspace, item, file ID, and path scope
  -> fileaccess or planwriter writes the workspace file
  -> scanner rescans the affected workspace
  -> item index and /api/state version update
  -> frontend refreshes current data and other tabs show stale-content notice
```

### External AI Session Launch

```text
User opens an item and selects Open AI session
  -> frontend loads detected providers, terminals, and item eligibility
  -> user selects workspace-only or selected-card context
  -> backend validates the indexed item and registered workspace
  -> selected-card passes the workspace-relative card path; workspace-only passes no prompt
  -> terminal adapter starts the selected AI CLI at the workspace root
  -> audit records identifiers and outcome without prompt content
```

Workspace-only starts at the workspace root without card context so the user can reference files and directories manually. Selected-card context works for any editable working-tree item and passes its workspace-relative path directly to the AI with a neutral instruction to read relevant documents and wait for the user's request. No persistent context resource is created. External tools retain their own authentication, approval, and sandbox behavior.

### Workspace Explorer

```text
User opens /explorer
  -> frontend renders every registered workspace as a root
	-> Configured Sources mode composes registered source roots by default
	-> All Files mode preserves full-workspace browsing
  -> expanding one row requests GET /api/workspaces/{id}/tree
  -> workspacefiles rejects traversal, .git, and outside symlinks
  -> Git ignore checks run once for the immediate directory
  -> selecting a file requests bounded classified content and its diff
  -> Markdown uses the shared editor session and required content hash
  -> successful saves and reverts append audit events
  -> configured-source changes trigger a targeted workspace refresh
```

### Explorer Productivity

```text
User searches unloaded paths
  -> bounded workspace traversal skips .git, ignored paths, and outside symlinks
  -> selecting a result expands ancestors and updates route selection

User creates or renames a path
  -> workspacefiles validates names, source, parent, and destination
  -> exclusive create or no-overwrite rename changes the workspace
  -> audit records success or blocked operation
  -> configured sources and affected directory caches refresh

Explorer loads Git path state
  -> one workspace Git status call returns normalized changes
  -> frontend aggregates child state to directory rows

User searches file contents
  -> item search resolves one guarded item directory
  -> Explorer search resolves configured sources or full roots from the active mode
  -> one shared budget bounds files, bytes, file size, results, and query length
  -> scanner skips .git, ignored paths, binary content, and outside symlinks
  -> selecting a result opens the file with line and column context
```

### Git Operations

```text
User opens Git panel
  -> GET /api/workspaces/{id}/git/status
  -> frontend shows branch, ahead/behind, dirty files, conflicts, and path selection
  -> user commits selected paths or runs fetch/pull/push/branch operation
  -> API validates branch names, commit message, path scope, and dirty-state guards
  -> Git adapter runs the command with timeout
  -> operations that change content rescan the affected workspace
```

### Stale Content Detection

```text
Frontend polls /api/state
  -> backend hashes workspaces and item summaries
  -> version changes after registry or index changes
  -> another open tab shows refresh popup
  -> user refreshes app data in place
```

## Storage Design

Plan Manager does not use a database server. It uses YAML files in the OS user config directory.

```text
<user-config-dir>/plan-manager/
  workspaces.yaml
  item-index.yaml
  audit-log.jsonl
  saved-filters.yaml
  recent-items.yaml
```

### workspaces.yaml

Stores registered workspace configuration. The filename and API fields use workspace naming.

| Field            | Type       | Purpose                                       |
|------------------|------------|-----------------------------------------------|
| `id`             | `string`   | Stable app ID derived from name and root path |
| `name`           | `string`   | Display name                                  |
| `path`           | `string`   | Absolute Git workspace root                   |
| `baselineBranch` | `string`   | Baseline branch validated at registration     |
| `sources`        | `string[]` | Configured sources such as `plans` or `docs`  |
| `createdAt`      | `string`   | Creation timestamp                            |
| `lastScannedAt`  | `string`   | Last successful scan timestamp                |

Example:

```yaml
- id: discovery-9409b56c
  name: discovery
  path: /workspace/discovery
  baselineBranch: master
  sources:
    - plans
    - docs
  createdAt: 2026-06-16T18:21:48Z
  lastScannedAt: 2026-06-17T09:18:05Z
```

### item-index.yaml

Stores cached item details, scan warnings, and scan timestamps. The filename and API fields use item naming.

| Field      | Type            | Purpose                                     |
|------------|-----------------|---------------------------------------------|
| `plans`    | `ItemDetail[]`  | Cached item details and document metadata   |
| `warnings` | `ScanWarning[]` | Non-fatal scan warnings                     |
| `scans`    | `object`        | Workspace ID to last scan timestamp mapping |

The item index is derived data. It can be rebuilt by scanning workspaces again.

### workspace-settings.yaml

Each configured source can optionally contain a workspace-owned settings file:

```text
<workspace>/<source>/workspace-settings.yaml
```

The expected source layout is:

```text
<workspace>/
└── <configured-source>/          # e.g. plans/
    ├── workspace-settings.yaml   # optional source items
    └── <folder>/
        └── <item>/
            ├── plan.yaml         # preferred plan metadata
            └── README.md
```

The canonical plan metadata format is:

```yaml
plan:
  status: draft
  tags: [backend, frontend]
```

The scanner infers `source` and `item` from the directory path, title from the first `README.md` heading, and documents recursively from Markdown files. It also infers document roles, tracks, labels, IDs, and display order from conventional paths such as `scenario/`, `design/`, and `implementation-plan.md`. Therefore, `plan.yaml` normally contains only workflow metadata that cannot be derived from the source tree: `status`, optional `owner`, and optional `tags`. Set `title` only when it intentionally differs from the README heading. Optional `documents` entries are sparse overrides merged by path onto discovered Markdown files; use them only when a role, track, or label cannot be inferred correctly.

This file lets a non-standard docs tree behave like a structured item source. The scanner currently supports segment-based path patterns where each segment is literal text or a `{variable}`. Generic product language uses `source` and `item`; legacy `repository-settings.yaml`, `service`, and `ticket` are read for migration compatibility.

Example:

```yaml
version: 1
cards:
  - pathPattern: "{folder}/feature/{item}"
    fields:
      source: docs
      item: "{item}"
      title: readme_heading
      status: draft
      tags: [docs]
```

If the file is missing or invalid, the scanner keeps the fallback behavior for that source root. If a configured card later receives a metadata edit, the metadata writer creates `plan.yaml` in that card directory, and `plan.yaml` becomes the source of truth on later scans.

## Item Data Model

### WorkspaceConfig

`WorkspaceConfig` is the compatibility API type for a registered workspace.

| Field            | Type       | Description                    |
|------------------|------------|--------------------------------|
| `id`             | `string`   | Workspace ID                   |
| `name`           | `string`   | Display name                   |
| `path`           | `string`   | Absolute Git workspace root    |
| `baselineBranch` | `string`   | Baseline branch                |
| `sources`        | `string[]` | Configured sources             |
| `createdAt`      | `string`   | Creation timestamp             |
| `lastScannedAt`  | `string`   | Last successful scan timestamp |

### ItemSummary

`ItemSummary` is the compatibility API type for an item card.

| Field            | Type       | Description                                              |
|------------------|------------|----------------------------------------------------------|
| `id`             | `string`   | Stable item ID                                           |
| `workspaceId`    | `string`   | Owning workspace                                         |
| `workspaceName`  | `string`   | Workspace display name                                   |
| `branch`         | `string`   | Current or identifier-matched branch                     |
| `scope`          | `string`   | Compatibility key for scope                              |
| `identifier`     | `string`   | Compatibility key for identifier                         |
| `title`          | `string`   | Display title                                            |
| `status`         | `string`   | `unsorted`, `draft`, `in_progress`, `review`, `done`     |
| `owner`          | `string`   | Metadata owner                                           |
| `author`         | `string`   | Last Git author or owner fallback                        |
| `tags`           | `string[]` | Item tags                                                |
| `updatedAt`      | `string`   | Last Git update or filesystem time                       |
| `description`    | `string`   | First README paragraph                                   |
| `metadataSource` | `string`   | `plan.yaml`, `workspace-settings`, `fallback`, or `docs` |
| `itemPath`       | `string`   | Workspace-relative item path                             |

### ItemDetail

`ItemDetail` is the compatibility API type for item detail and extends `ItemSummary`.

| Field       | Type             | Description                                                           |
|-------------|------------------|-----------------------------------------------------------------------|
| `documents` | `ItemDocument[]` | Explicit documents or Markdown files inferred from the plan directory |
| `metadata`  | `object`         | Parsed item metadata                                                  |
| `warnings`  | `ScanWarning[]`  | Item-level warnings                                                   |
| `counts`    | `object`         | Workspace counts such as file count                                   |

## Item Discovery Rules

Discovery runs per configured source. The scanner checks the modes in this order:

1. `workspace-settings.yaml` if present and valid.
2. Structured item discovery.
3. Freestyle docs fallback.

Structured item roots use:

```text
{source}/{folder}/{item}/
```

A folder is treated as a structured item when:

- It contains `plan.yaml`, or
- Its item folder matches an uppercase identifier pattern such as `DI-170`.

Freestyle docs roots are supported when:

- The configured root contains Markdown files, and
- It does not contain structured item children.

Plain freestyle docs roots are assigned the `unsorted` status so the Kanban board separates unstructured sources from normal workflow columns. Once a source root has a valid `workspace-settings.yaml`, matched cards use the configured status or `plan.yaml`.

Metadata precedence:

1. `plan.yaml`.
2. `workspace-settings.yaml` fields and README heading.
3. README heading and inferred status.
4. Folder names and fallback defaults.

Status normalization maps common values into:

- `draft`
- `in_progress`
- `review`
- `done`
- `unsorted`

## API Endpoints

All endpoints are local and served from `http://127.0.0.1:{port}`.

| Method   | Endpoint                                                | Description                                      |
|----------|---------------------------------------------------------|--------------------------------------------------|
| `GET`    | `/api/health`                                           | Health check                                     |
| `GET`    | `/api/state`                                            | App state version, workspace count, item count   |
| `GET`    | `/api/audit-events`                                     | Recent local operation events                    |
| `GET`    | `/api/search`                                           | Ranked indexed item search                       |
| `GET`    | `/api/saved-filters`                                    | List saved Kanban filter views                   |
| `POST`   | `/api/saved-filters`                                    | Create or update a saved filter                  |
| `DELETE` | `/api/saved-filters/{id}`                               | Delete a saved filter                            |
| `GET`    | `/api/recent-items`                                     | List recently opened items                       |
| `POST`   | `/api/recent-items`                                     | Record an opened item                            |
| `GET`    | `/api/workspaces`                                       | List registered workspaces                       |
| `POST`   | `/api/workspaces`                                       | Create workspace registration                    |
| `PUT`    | `/api/workspaces/{id}`                                  | Update workspace registration                    |
| `DELETE` | `/api/workspaces/{id}`                                  | Delete workspace registration and cached items   |
| `POST`   | `/api/workspaces/{id}/scan`                             | Scan one workspace                               |
| `GET`    | `/api/workspaces/{id}/health`                           | Read workspace health checks                     |
| `GET`    | `/api/workspaces/{id}/source-structure?directory={dir}` | Read source item settings                        |
| `PUT`    | `/api/workspaces/{id}/source-structure?directory={dir}` | Save source item settings and rescan             |
| `GET`    | `/api/items`                                            | List cached item summaries                       |
| `GET`    | `/api/items/{id}`                                       | Get item detail                                  |
| `GET`    | `/api/items/{id}/files`                                 | Get safe file tree for an item                   |
| `GET`    | `/api/items/{id}/files/{fileID}`                        | Read one item file                               |
| `POST`   | `/api/items/{id}/files/{fileID}`                        | Save one Markdown file                           |
| `GET`    | `/api/items/{id}/diff`                                  | Get read-only Git diff for the item path         |
| `PATCH`  | `/api/items/{id}/metadata`                              | Save structured item metadata                    |
| `PATCH`  | `/api/items/{id}/status`                                | Move a structured item to another status         |
| `POST`   | `/api/items`                                            | Create a structured item                         |
| `GET`    | `/api/workspaces/{id}/git/status`                       | Read branch, ahead/behind, and change state      |
| `POST`   | `/api/workspaces/{id}/git/fetch`                        | Fetch remotes                                    |
| `POST`   | `/api/workspaces/{id}/git/pull`                         | Pull with dirty-state guard                      |
| `POST`   | `/api/workspaces/{id}/git/push`                         | Push current branch                              |
| `POST`   | `/api/workspaces/{id}/git/commit`                       | Commit selected item paths                       |
| `POST`   | `/api/workspaces/{id}/git/branches`                     | Create a branch                                  |
| `POST`   | `/api/workspaces/{id}/git/switch`                       | Switch branch with dirty-state guard             |
| `POST`   | `/api/system/select-directory`                          | Open native directory picker                     |
| `POST`   | `/api/system/open-path`                                 | Reveal a local path in the platform file manager |

### Query Parameters

`GET /api/items` supports:

| Parameter     | Description                                                    |
|---------------|----------------------------------------------------------------|
| `workspaceId` | Filter by workspace ID                                         |
| `branch`      | Filter by branch                                               |
| `status`      | Filter by normalized status                                    |
| `q`           | Search title, identifier, scope, description, author, and tags |

## API Payloads

### WorkspaceInput

```json
{
  "name": "discovery",
  "registrationMode": "local_path",
  "path": "/workspace/discovery",
  "baselineBranch": "master",
  "sources": ["plans", "docs"]
}
```

Remote-clone example:

```json
{
  "name": "discovery",
  "registrationMode": "remote_clone",
  "remoteUrl": "git@bitbucket.org:team/discovery.git",
  "cloneRoot": "/workspace",
  "baselineBranch": "master",
  "sources": ["plans", "docs"]
}
```

### ScanResult

```json
{
  "workspaceId": "discovery-9409b56c",
  "scannedAt": "2026-06-17T09:18:05Z",
  "itemCount": 42,
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
- Autosave Markdown edits after a short debounce.
- Restrict reads and writes to configured sources.
- Resolve file IDs through the safe file tree or plan document list.
- Reject path traversal and symlink escapes.
- Use expected content hashes for Markdown saves.
- Keep plain freestyle docs roots Markdown-only and place them in `Unsorted`.
- Stage and commit only selected paths inside configured sources.
- Block pull and branch switch on dirty working trees unless confirmed.
- Do not store Git credentials.
- Store app config and cache outside registered workspaces.
- Validate workspace roots through Git.
- Validate baseline branches at registration.
- Restrict file reads to configured sources.
- Reject invalid file paths and symlink escapes.
- Use short timeouts for Git commands.
- Do not store credentials.

PM-002 will add guarded write operations. See [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md).

## Performance Model

- Board and list views read cached item summaries.
- File content loads only after a plan file is opened.
- Scans rebuild derived metadata for one workspace.
- Large workspace support depends on keeping Markdown content out of board-level queries.
- The target scale from PM-001 is 100 workspaces, 10,000 plans, and 100,000 files.

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
