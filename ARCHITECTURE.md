# Kode Stream Architecture

Kode Stream uses a Go backend and React frontend to provide workflow views over Git-backed planning content. This
document focuses on system boundaries, package responsibilities, data ownership, and integration flow.

## Architecture Principles

- Keep source content in Git and app state outside managed repositories.
- Make repository writes explicit, scoped, and guarded.
- Use cached indexes for fast board, search, explorer, and graph workflows.
- Keep transport concerns at the API boundary.
- Treat workspace content as untrusted input.
- Keep integrations optional and isolated from core workspace workflows.

## System Overview

```text
Browser
  -> Kode Stream server
  -> React app and JSON API
  -> Domain services
  -> App-owned state
  -> Registered Git workspaces
  -> Optional integrations
```

```text
┌──────────────────────────────────────────────────────────────┐
│ React app                                                    │
│ Workstream | Explorer | Item workspace | Wiki graph | Tools  │
└──────────────────────────────┬───────────────────────────────┘
                               │ JSON API
┌──────────────────────────────▼───────────────────────────────┐
│ Go server                                                    │
│ internal/server       Composition and embedded frontend      │
│ internal/server/api   HTTP transport and API contracts       │
│ internal/*            Domain services and adapters           │
└───────────────┬───────────────────────────────┬──────────────┘
                │                               │
┌───────────────▼────────────────┐  ┌───────────▼───────────────┐
│ App-owned state                │  │ Registered Git workspaces │
│ Registry, indexes, audit, UI   │  │ User source files         │
│ state, integration settings    │  │ Git history               │
└────────────────────────────────┘  └───────────────────────────┘
```

## Backend Layers

| Layer             | Package               | Role                                                         |
|-------------------|-----------------------|--------------------------------------------------------------|
| CLI               | `cmd/kode-stream`     | Starts the server and runs diagnostics                       |
| Composition       | `internal/server`     | Wires services, resolves paths, serves embedded frontend     |
| API transport     | `internal/server/api` | Owns routes, middleware, request parsing, and JSON responses |
| Shared contracts  | `internal/common`     | Defines errors, HTTP helpers, and compatibility DTOs         |
| Domain services   | `internal/*`          | Own workflows, policies, repositories, and integration logic |
| File capabilities | `internal/filesystem` | Provides guarded path validation, bounded reads, and writes  |

Gin is limited to `internal/server/api`. Domain packages receive standard contexts, model types, and primitive
parameters instead of framework-specific request objects.

## Domain Areas

| Domain       | Package                 | Responsibility                                                    |
|--------------|-------------------------|-------------------------------------------------------------------|
| Workspace    | `internal/workspace`    | Registration, import, scanning, source settings, files, health    |
| Workstream   | `internal/workstream`   | Board snapshots, branch-scoped views, filters, active workspace   |
| Item         | `internal/item`         | Item detail, Markdown writes, metadata, status, creation, refresh |
| Search       | `internal/search`       | Indexed item search, content search, and workspace path search    |
| Knowledge    | `internal/knowledge`    | LLM Wiki indexing, graph, sync, reads, and enrichment             |
| Git          | `internal/git`          | Status, fetch, pull, push, commit, branch create, branch switch   |
| Jira         | `internal/jira`         | Issue lookup, REST access, caching, and guarded attachments       |
| AI           | `internal/ai`           | Provider detection, settings, external launch, embedded sessions  |
| Verification | `internal/verification` | Bounded verification jobs, status, checkpoints, and artifacts     |
| System       | `internal/system`       | Config paths, native dialogs, path reveal, health, diagnostics    |
| Audit        | `internal/audit`        | Local operation event append and query                            |
| Navigation   | `internal/navigation`   | Saved filters and recent items                                    |

## Frontend Areas

| Area            | Path                                   | Role                                                     |
|-----------------|----------------------------------------|----------------------------------------------------------|
| App shell       | `web/src/App.tsx`                      | Layout and top-level navigation                          |
| App state       | `web/src/app/useAppState.ts`           | Workspace, route, refresh, theme, and stale state        |
| API layer       | `web/src/shared/api`                   | Fetch wrapper and endpoint methods                       |
| Workstream      | `web/src/pages/WorkstreamPage.tsx`     | Board, cards, filters, intake, and preview drawer        |
| Workspaces      | `web/src/pages/WorkspacesPage.tsx`     | Workspace setup, import, edit, delete, scan, reveal      |
| Item workspace  | `web/src/pages/ItemWorkspacePage.tsx`  | Files, preview, editor, diff, metadata, Jira, Git tools  |
| Explorer        | `web/src/pages/WorkstreamExplorer.tsx` | Workspace tree, file editor, content search, inspector   |
| Feature modules | `web/src/features/*`                   | Search, reliability, content rendering, editor, explorer |
| Shared modules  | `web/src/shared/*`                     | Reusable API, domain, and UI support code                |

Frontend shared modules do not import page modules. Pages compose feature and shared modules.

## Data Ownership

Kode Stream separates app-owned state from repository-owned content.

| Owner      | Examples                                        | Write policy                                           |
|------------|-------------------------------------------------|--------------------------------------------------------|
| App state  | Workspace registry, indexes, audit, UI settings | Written by Kode Stream outside registered repositories |
| Repository | Markdown, metadata, wiki pages, Git history     | Written only through explicit user actions             |

Indexes are derived data. A scan can rebuild them from registered workspace content.

## Item Discovery

The scanner turns configured repository sources into item cards and knowledge entries. It evaluates each source in this
order:

1. Source mappings from `workspace-settings.yaml`.
2. Structured item folders.
3. Freestyle Markdown docs.

Metadata precedence is:

1. `plan.yaml`
2. Source mapping fields and README heading
3. README heading and inferred status
4. Folder names and fallback defaults

Supported workflow statuses are `draft`, `in_progress`, `review`, `done`, and `unsorted`.

## Core Flows

### Scan And Index

```text
Register or scan workspace
  -> validate workspace configuration
  -> read configured sources
  -> apply source mappings
  -> parse metadata, README headings, Markdown documents, and Git metadata
  -> update derived indexes
  -> report warnings without blocking valid items
```

### Write

```text
Edit Markdown or metadata
  -> validate workspace, item, file identity, path scope, and content hash
  -> write selected file or metadata
  -> rescan affected workspace data
  -> refresh open views
```

### Integration Flow

```text
Jira
  -> resolve workspace settings and token
  -> fetch matching issue context
  -> proxy allowed attachments through guarded access

Terminal and AI sessions
  -> detect supported local providers
  -> validate workspace and optional selected item
  -> launch external terminal or embedded session
  -> record audit outcome without prompt content

Verification
  -> start bounded job
  -> collect checkpoints, status, logs, and artifacts
  -> expose results through the item and verification APIs

LLM Wiki & Graph
  -> index structured Markdown knowledge
  -> build graph relationships
  -> expose reads, sync, enrichment, and graph views
```

## API Structure

All API routes are local and grouped by capability.

| Family       | Scope                                                                       |
|--------------|-----------------------------------------------------------------------------|
| Health/audit | Health checks and recent operation events                                   |
| Navigation   | Saved filters and recent items                                              |
| System       | Config paths, native pickers, path reveal, diagnostics                      |
| State/search | App state, item search, content search, AI settings, capabilities           |
| Workspace    | Workspace CRUD, import, scan, runtime, source settings, files, tree, search |
| Item         | Item detail, files, writes, metadata, status, diff, Jira, verification      |
| Knowledge    | Wiki reads, graph, rescan, sync, enrichment                                 |
| Verification | Job creation, checkpoint ingest, status, artifacts, reruns                  |
| Git          | Status, activity, branches, fetch, pull, push, commit, branch operations    |
| Streaming    | Workspace creation events and embedded session channels                     |

## Error Handling

Domain errors use stable codes and map to HTTP statuses at the API boundary.

| Code           | HTTP status                 |
|----------------|-----------------------------|
| `not_found`    | `404 Not Found`             |
| `validation`   | `400 Bad Request`           |
| `conflict`     | `409 Conflict`              |
| `unauthorized` | `401 Unauthorized`          |
| `forbidden`    | `403 Forbidden`             |
| `unavailable`  | `503 Service Unavailable`   |
| `infra`        | `500 Internal Server Error` |

The frontend receives a stable JSON envelope with `error` and optional `code` and `recoveryHint` fields.

## Safety

The safety model is enforced across filesystem, rendering, Git, and integration boundaries:

- Scope file access to registered workspace sources.
- Reject path traversal, `.git` access, and symlink escapes.
- Use expected content hashes for Markdown writes.
- Sanitize or escape workspace content in previews.
- Stage only selected paths for Git commits.
- Keep credentials, prompts, and terminal content out of managed repositories.
