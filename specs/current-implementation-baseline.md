# Kode Stream Requirement (Current Implementation Baseline)

This document defines the currently implemented product scope, based on
`plans/platform/current-spec/*`.

## 1. Vision and Scope

Kode Stream is a local Git-native web application for managing planning artifacts
from one or more Git workspaces. It provides Kanban, explorer, and item-level
editing workflows while keeping all source-of-truth data in Git repositories.

The system is single-user and local-first:

- the server binds to loopback only (`127.0.0.1`)
- API and frontend are served from one local process
- no remote hosting behavior is required by default

## 2. Runtime and Composition Requirements

### 2.1 CLI and Server Runtime

- The CLI must support `kode-stream serve [-port]`.
- Default port is `4317`.
- If `-port` is not provided, `KODE_STREAM_PORT` must be used when set.
- Server must expose JSON API under `/api/*` and serve the SPA frontend.
- Unknown top-level frontend routes must resolve to Kanban behavior.

Core runtime endpoints must include:

- `GET /api/health`
- `GET /api/state`
- `GET /api/workspaces`

`GET /api/state` must return a version hash derived from workspace records and
indexed item data.

### 2.2 Frontend Route Baseline

The app shell must provide these routes:

- `/kanban`
- `/explorer`
- `/workspaces`
- `/items/{itemId}`

### 2.3 Service Wiring

Runtime composition must include services for:

- Git operations
- workspace registry
- item indexing
- scanning
- guarded file access and writing
- audit events
- health/reliability checks
- search
- navigation recents
- API route mounting

## 3. Core Domain Concepts

### 3.1 Workspace

A workspace is a registered local Git repository with metadata:

- `id`, `name`, `path`, `baselineBranch`, `lastSelectedBranch`, `sources`
- `createdAt`, `lastScannedAt`

### 3.2 Source

A source is a workspace-relative directory used for item discovery.

### 3.3 Item

An item is a discovered planning unit indexed from source content.

Two item categories must be supported:

- structured items (folder-based, metadata-driven)
- docs-root items (Markdown-root fallback, metadata source `docs`)

### 3.4 Branch Context

Kanban and explorer behavior must respect per-workspace branch context, with
support for working-tree and snapshot reads.

## 4. Workspace Registration and Validation

### 4.1 CRUD

Workspace CRUD endpoints must exist:

- `GET /api/workspaces`
- `POST /api/workspaces`
- `PUT /api/workspaces/{id}`
- `DELETE /api/workspaces/{id}`

Existing workspace import endpoints must exist:

- `POST /api/workspaces/import-preview`
- `POST /api/workspaces/import`

Import preview must strictly parse one bounded current-schema `workspaces.yaml`, return every candidate and its
field-level issues, detect source and registry duplicates, and perform no writes. Confirmed import must reread the
source, match selected candidate keys, atomically register still-valid candidates, and scan each registration without
rolling back registrations whose scan fails. Imported registrations must use destination identity and
`existing_workspace` ownership so deleting one never removes its directory.

### 4.2 Validation Rules

On create/update, the system must validate:

- `name` is provided
- `path` resolves to a Git repository root
- `baselineBranch` exists
- each source path is relative and exists within workspace root

### 4.3 System Path Helpers

The workspace UI must be supported by:

- `POST /api/system/select-directory`
- `POST /api/system/select-file`
- `POST /api/system/open-path`
- `GET /api/system/config-paths` including backend-resolved `registryFile`

## 5. Source Configuration, Discovery, and Indexing

### 5.1 Source Settings

Per-source settings may be defined via `workspace-settings.yaml`.

Requirements:

- support one or more card rules (`pathPattern` + `fields`)
- accept canonical fields (`source`, `item`) and aliases (`scope`, `identifier`)
- support literal and `{variable}` path segments
- return warnings for invalid settings without blocking fallback scanning

Source settings endpoints must exist:

- `GET /api/workspaces/{id}/source-structure?directory={dir}`
- `PUT /api/workspaces/{id}/source-structure?directory={dir}`
- `DELETE /api/workspaces/{id}/source-structure?directory={dir}`

The source-structure read response must include:

- mode: `structured`, `unstructured`, `empty`, or `unknown`
- inferred proposals
- preview rows for mapped item fields

### 5.2 Discovery Order

Scanning must evaluate each source in this order:

1. valid configured rules from settings
2. structured traversal (`{source}/{scope}/{identifier}`)
3. freestyle docs-root fallback

Structured item detection must support:

- folder contains `plan.yaml`, or
- folder name matches identifier-like uppercase pattern (for example `PM-012`)

### 5.3 Metadata Extraction

Scanner/index behavior must support:

- metadata from `plan.yaml` (`identifier`, `scope`, `title`, `status`, `owner`, `tags`)
- title from README heading fallback
- description from first README paragraph fallback
- inferred document list from Markdown paths and roles
- `plan.yaml` document override merge by path

Status normalization must support:

- `unsorted`, `draft`, `in_progress`, `review`, `done`

### 5.4 Docs-Root Fallback

If structured layout does not apply and source root contains Markdown files:

- one docs item must be created for that source root
- item status must be `unsorted`
- metadata source must be `docs`

### 5.5 Item Index Storage

Index state must be persisted in `item-index.yaml` and support:

- replace by workspace scan
- replace by workspace+branch scan
- query by workspace, branch, status, and text
- direct item detail lookup by ID
- branch scan metadata persistence

Deleting a workspace must remove its workspace record and indexed items.

### 5.6 Branch-Aware Scanning

Scanning must support:

- working-tree mode (filesystem reads)
- snapshot mode (Git tree object reads)
- stable item IDs for workspace + branch + item path identity

## 6. Kanban Board and Item Lifecycle

### 6.1 Board Scope and Branch Loading

Route `/kanban` must operate on one active workspace.

Branch loading endpoint:

- `POST /api/workspaces/{id}/kanban/branch`

Returned branch context must include:

- selected branch
- resolved ref/commit
- current checkout branch
- source mode: `working_tree` or `snapshot`
- editability flag
- branch-scoped item list
- scan warnings

Supporting endpoints:

- `GET /api/workspaces/{id}/git/status`
- `GET /api/workspaces/{id}/git/branches`

Board branch UX must support local branch listing with checkout indicator,
branch-switch refresh without implicit checkout, and explicit manual refresh.

### 6.2 Filters and Saved Views

Kanban filtering must support:

- source
- scope
- status
- author
- free-text query

Saved filter APIs must support create/list/delete:

- `GET /api/saved-filters`
- `POST /api/saved-filters`
- `DELETE /api/saved-filters/{id}`

Saved filter payload must support route, optional workspace scope, and serialized
filter/query state.

### 6.3 Columns and Card Rules

Board columns must follow status order:

- `unsorted`, `draft`, `in_progress`, `review`, `done`

Card behavior requirements:

- docs-root cards are visible but not draggable
- structured cards support status changes
- preview drawer opens via click/keyboard
- title opens `/items/{itemId}`

### 6.4 Status Updates

Status update endpoint:

- `PATCH /api/items/{id}/status`

Interaction and safety requirements:

- support drag/drop and menu-based status update
- optimistic move while request is pending
- per-card pending lock against duplicate updates
- rollback on failure
- block dropping into `unsorted`
- block docs-root item mutation

### 6.5 Snapshot Materialization

When an item originates from snapshot mode and a write action is attempted,
the UI must request explicit confirmation.

Confirmed requests must include `materializeConfirmed: true` so backend can
materialize into checkout branch before applying write.

### 6.6 Item Creation

Kanban must support new structured item creation:

- `POST /api/items`

Creation flow must allow source, identifier, title, and initial status selection.

## 7. Item Workspace Requirements

### 7.1 Route and Initial Loading

Route `/items/{itemId}` must load:

- `GET /api/items/{id}`
- `GET /api/items/{id}/files`
- `GET /api/items/{id}/diff`
- `GET /api/workspaces/{workspaceId}/git/status`

Selected file content must load via:

- `GET /api/items/{id}/files/{fileID}`

### 7.2 File Workspace UI Model

The item workspace must provide:

- left panel: file tree and in-item content search
- center panel: preview/raw/diff tabs
- right panel: item metadata or Git controls

File tree must support recursive navigation, keyboard/pointer selection, and file
state indicators from Git and unsaved edits.

### 7.3 In-Item Content Search

Endpoint:

- `GET /api/items/{id}/content-search?q=...`

Behavior:

- debounced query
- minimum length 2
- results include line/column/snippet
- selecting result opens file at matched coordinates

### 7.4 Editing and Autosave

File editing must support shared editor session behavior:

- debounced autosave (default 900ms)
- save state: pending, saving, saved, error
- save-before-navigation when unsaved changes exist

Save endpoint:

- `POST /api/items/{id}/files/{fileID}` with `content`, `expectedHash`

Backend write guards must enforce:

- path is within item root and configured sources
- stale hash blocks blind overwrite
- unsupported/binary content is rejected

Revert endpoint:

- `POST /api/items/{id}/files/{fileID}/revert`

### 7.5 Metadata Editing

Metadata endpoint:

- `PATCH /api/items/{id}/metadata`

Rules:

- docs-root items are metadata read-only
- structured items support title/scope/identifier/status/owner/tags updates
- successful metadata writes must refresh workspace scan/index state

### 7.6 Diff and Git Controls

Diff endpoint:

- `GET /api/items/{id}/diff`

UI requirements:

- parsed review mode and raw diff mode
- single-file revert from selected file context

Workspace Git actions in item workspace must support:

- `POST /api/workspaces/{id}/git/fetch`
- `POST /api/workspaces/{id}/git/pull`
- `POST /api/workspaces/{id}/git/push`
- `POST /api/workspaces/{id}/git/commit`
- `POST /api/workspaces/{id}/git/branches`

Git panel must show branch/ahead-behind/dirty-conflict state, changed path
selection for commit, commit validation, and branch creation with optional
checkout.

## 8. Global Explorer Requirements

### 8.1 Scope and Route State

Route `/explorer` must display all registered workspaces and persist selection in
query state (`workspaceId`, `path`, `mode`).

### 8.2 Tree Modes and Directory Loading

Explorer must support:

- `sources` mode (default)
- `all` mode (full repository tree)

Mode must be persisted locally and reflected in route state.

Directory loading endpoint:

- `GET /api/workspaces/{id}/tree?path={dir}&includeIgnored={bool}`

Directory loading constraints:

- lazy one-level child loading
- hide `.git` and protected paths
- block symlink escapes outside workspace root
- hide ignored files unless explicitly requested

Expansion/selection/ignored-visibility state must persist locally.

### 8.3 Explorer Branch Selection

Per-workspace branch controls must use:

- `GET /api/workspaces/{id}/git/branches`
- `POST /api/workspaces/{id}/git/switch`

Branch switching must refresh only the affected workspace explorer/search/cache
and Git path state.

### 8.4 Unified Explorer Search

Path search endpoint:

- `GET /api/workspaces/files/search?q=...&workspaceId=...&includeIgnored=...`

Content search endpoint:

- `GET /api/workspaces/files/content-search?q=...&mode=...&workspaceId=...`

Requirements:

- debounced search
- keyboard navigation with arrows, Enter, Escape
- result select expands ancestors and opens target
- content results include line/column context

### 8.5 Workspace File Operations

Workspace file APIs must support guarded open/save/diff/revert:

- `GET /api/workspaces/{id}/files?path=...`
- `PUT /api/workspaces/{id}/files`
- `GET /api/workspaces/{id}/files/diff?path=...`
- `POST /api/workspaces/{id}/files/revert`

Writes must use expected-hash conflict protection.

### 8.6 Path Mutations

Explorer must support:

- `POST /api/workspaces/{id}/files` (Markdown create)
- `POST /api/workspaces/{id}/directories` (directory create)
- `POST /api/workspaces/{id}/paths/rename` (rename)

Mutation guardrails:

- reject invalid names, absolute/traversal paths, `.git`, protected paths
- reject symlink mutation or out-of-root destination
- reject destination collisions
- allow Markdown create only for `.md` or `.markdown`
- return invalidated directories for targeted frontend refresh

### 8.7 Git Path Decorations and Inspector

Path status endpoint:

- `GET /api/workspaces/{id}/git/path-status`

Explorer must render status for modified, added, deleted, renamed, untracked, and
conflicted paths, including ancestor aggregation.

Inspector panel must show workspace branch/health/change count, selected file
metadata, and linked item metadata when path maps to indexed item context.

## 9. Global Search, Recents, and Navigation

### 9.1 Command Palette Search

Global search is opened by `Ctrl/Cmd+K` and must support current-workspace and
all-workspaces scopes.

Search endpoint:

- `GET /api/search?q=...&workspaceId=...&types=...&limit=...`

Results must include route, subtitle/context, and ranking score.

### 9.2 Recent Items

Recent item APIs:

- `GET /api/recent-items?limit=...`
- `POST /api/recent-items`

Selecting a search result must record recents.

## 10. Reliability and State Coordination

### 10.1 Workspace Health and Audit

Health endpoint:

- `GET /api/workspaces/{id}/health`

Health checks must include path availability, source availability, Git root/
conflict status, baseline branch presence, read/write permission, and indexed
item availability.

Audit endpoint:

- `GET /api/audit-events?workspaceId=...&limit=...`

Audit events must store operation, status (`success`, `blocked`, `failed`),
message, paths, duration, and error metadata.

Frontend reliability surfaces must refresh health/activity on initial mount,
explicit refresh, and `kode-stream:reliability-changed` events after write/Git
actions.

### 10.2 Stale Content Awareness

Frontend must poll `GET /api/state` every 30 seconds while visible.

When state version changes, the app must show stale notice and allow in-place
refresh.

Cross-tab signaling must use local-storage key `itemManagerContentVersion`.

## 11. Secure Content Rendering and Search Budgets

### 11.1 Content Viewer Modes

Viewer must classify and render files as:

- Markdown rendered/source
- HTML rendered/source
- JSON/YAML structured/source
- code/text source

### 11.2 Security Requirements

- Markdown rendering must sanitize HTML and use controlled plugins.
- HTML preview must sanitize via DOMPurify, strip resource-loading attributes,
  apply strict CSP, and render in sandboxed iframe.
- JSON/YAML structured mode must enforce depth/node/alias limits.
- Source code rendering must escape content and bound syntax highlighting.

Backend must reject binary/unsupported text for viewer flows.

For large files, rich preview must pause with source fallback; large text payloads
may be truncated.

### 11.3 Shared Content Search Limits

Limits for item and explorer content-search endpoints:

- min query length: 2
- max query length: 200
- max results: 100
- max files scanned: 10,000
- max bytes read: 64 MiB
- max file size: 2 MiB
- max snippet length: 240 characters

## 12. Persistence Requirements

### 12.1 Local App Data

App-managed files must be stored under:

`<user-config-dir>/kode-stream/`

Required files:

- `workspaces.yaml`
- `item-index.yaml`
- `audit-log.jsonl`
- `saved-filters.yaml`
- `recent-items.yaml`
- `ai-settings.yaml`

## 13. AI Sessions

### 13.1 Settings and Detection

Kode Stream must detect supported local AI providers and terminal applications without starting them. Machine-specific settings are stored in `ai-settings.yaml` with mode `0600` and expose:

- `GET /api/ai/capabilities`
- `GET /api/ai/settings`
- `PUT /api/ai/settings`

Launch templates accept only approved placeholders and must execute through argument arrays or validated native-terminal wrappers.

### 13.2 Item Launch

Item AI APIs:

- `GET /api/items/{id}/ai-session-eligibility`
- `POST /api/items/{id}/ai-sessions`

Context mode must be `workspace_only` or `card_context`. Workspace-only opens at the registered workspace root without provider prompt arguments and may be used from snapshot items. Card context requires an editable working-tree item but does not require `plan.yaml`; it passes the workspace-relative card path directly to the AI with a neutral instruction to read relevant documents and wait for the user's request. Neither mode creates a persistent context resource.

Provider authentication, approvals, and sandbox behavior remain owned by the provider CLI. Audit events must not record prompts, command arguments, or manifest contents.

### 13.3 Embedded Sessions

The launch dialog keeps external mode as the default and may start a provider in a managed embedded PTY. Embedded sessions expose create, status, cancel, and loopback WebSocket channel APIs. Session IDs and grants are cryptographically random; grants are short-lived, scoped to one session, retained only in browser memory, and compared in constant time.

The session manager limits concurrent sessions, terminal dimensions, reconnect output, and disconnect grace. It terminates the process group after cancellation, lease expiry, startup failure, or server shutdown. Request logs and audit payloads must not include grant query values, provider input, or terminal output. Embedded PTYs are supported on macOS, Linux, FreeBSD, DragonFly, and OpenBSD; external launch remains available elsewhere.

## 14. Read-Only Jira Integration

Each workspace may define one optional Jira Cloud or Jira Server/Data Center connection. Persisted configuration contains deployment type, base URL, project key, optional Cloud account email, and token environment-variable name. Token values must never be persisted or returned.

Required APIs:

- `POST /api/workspaces/{id}/jira/test`
- `GET /api/items/{id}/jira`
- `POST /api/items/{id}/jira/refresh`
- `GET /api/items/{id}/jira/attachments/{attachmentId}`

Item identifiers must match the configured project key exactly before Jira access. Successful and not-found reads may be cached in memory for five minutes; refresh bypasses the cache. Expected Jira states must distinguish not configured, invalid identifier, project mismatch, not found, authentication failure, forbidden access, and unavailable service.

Descriptions must render as safe normalized text. Attachments require explicit user action and backend checks for issue ownership, origin, response size, filename, content type, timeout, and safe response headers.

## 15. Baseline Safety Rules

- Server must not expose remote network binding by default.
- Registry/index/audit data must be outside managed workspace paths.
- Git operations must run through timeout-bounded adapter behavior.
- Error responses may include recovery hints for blocked or stale actions.
