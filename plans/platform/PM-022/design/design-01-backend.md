# Backend Design: Structured Knowledge Wiki

## Overview

The backend adds a Knowledge-specific parser, persisted index, application service, and HTTP resource. It reuses the workspace registry, path guards, Git service, audit store, and classified Markdown content model. No database or workspace-owned metadata file is introduced.

## Package Ownership

| Package                          | Responsibility                                                                  |
|----------------------------------|---------------------------------------------------------------------------------|
| `internal/knowledge`             | Detection, parsing, normalization, link resolution, graph creation, persistence |
| `internal/application/knowledge` | Workspace resolution, queries, rescan, Sync, Enrich, and audit coordination     |
| `internal/api`                   | Request decoding, response mapping, errors, and route registration              |
| `internal/registry`              | Persist optional workspace Knowledge configuration                              |
| `internal/config`                | Resolve `knowledge-index.yaml` in the app data directory                        |

## Data Model

### Workspace Configuration

```go
type KnowledgeSettings struct {
    Enabled          *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
    EnrichExecutable string   `json:"enrichExecutable,omitempty" yaml:"enrichExecutable,omitempty"`
    EnrichArgs       []string `json:"enrichArgs,omitempty" yaml:"enrichArgs,omitempty"`
}
```

`nil` or omitted `enabled` means enabled for backward-compatible automatic detection. `false` disables detection and actions for that workspace. The executable may be absolute or resolved through the server process PATH. Arguments are passed literally; variable expansion and shell syntax are not supported.

### Wiki And Page Models

| Model                   | Required Fields                                                                                   |
|-------------------------|---------------------------------------------------------------------------------------------------|
| `KnowledgeWiki`         | workspace ID, root, display name, page count, warning count, indexed timestamp                    |
| `KnowledgePageSummary`  | slug, title, relative path, domain, optional metadata, outgoing links, backlinks                  |
| `KnowledgePageDetail`   | summary fields plus classified Markdown content                                                   |
| `KnowledgeLink`         | source slug, raw target, optional label, resolved target slug, resolution state                   |
| `KnowledgeWarning`      | workspace ID, Wiki root, optional path and slug, warning code, message                            |
| `KnowledgeGraph`        | nodes, deduplicated directed edges, total counts, truncation state                                |
| `KnowledgeActionResult` | operation, Wiki summaries, warnings, bounded operation log, truncation flag, completion timestamp |

Stable warning codes include `invalid_front_matter`, `missing_identity`, `duplicate_slug`, `unresolved_link`, `invalid_metadata`, `file_too_large`, and `unsafe_path`.

## Persisted Index

Store `knowledge-index.yaml` under the resolved Kode Stream data directory. The file contains a schema version and entries keyed by workspace ID plus normalized Wiki root. Writes use a temporary file, file sync, and atomic rename. A failed scan does not replace the last valid entry.

The index stores metadata and relationships, not full Markdown content. Page content is read on selection through guarded access so local edits are visible after rescan and the app-owned file remains bounded.

## Detection And Parsing

1. Read only registered `WorkspaceConfig.sources` in the working tree.
2. Normalize and validate every source path against the workspace root.
3. Require a regular `index.md` and at least one valid page with `slug` and `title`.
4. Walk Markdown files with limits for files visited, bytes per file, total bytes, pages, links, and elapsed time.
5. Parse YAML front matter with `gopkg.in/yaml.v3`.
6. Normalize string or sequence forms of `roles`, `topics`, and `sourceRef`.
7. Extract `[[slug]]`, `[[slug|label]]`, and relative `.md` links while ignoring fenced and inline code through Markdown AST traversal.
8. Sort pages by normalized path before choosing the first duplicate slug.
9. Resolve relative Markdown paths and slugs inside the same Wiki root.
10. Build backlinks and deduplicated graph edges.

Default budgets should be constants covered by tests and selected to comfortably index the current Discovery Wiki. Budget exhaustion returns partial valid results plus warnings, except timeout or root-access failure, which preserves the prior index.

## API Contract

| Method | Endpoint                                                 | Purpose                                           |
|--------|----------------------------------------------------------|---------------------------------------------------|
| `GET`  | `/api/knowledge/wikis?workspaceId={id}`                  | List detected/indexed Wikis                       |
| `GET`  | `/api/knowledge/wikis/{workspaceId}/{root}/pages`        | Return hierarchy, summaries, links, and warnings  |
| `GET`  | `/api/knowledge/wikis/{workspaceId}/{root}/pages/{slug}` | Return guarded Markdown content and page metadata |
| `GET`  | `/api/knowledge/wikis/{workspaceId}/{root}/graph`        | Return bounded graph nodes and edges              |
| `POST` | `/api/knowledge/wikis/{workspaceId}/{root}/rescan`       | Rebuild one Wiki from the working tree            |
| `POST` | `/api/knowledge/workspaces/{workspaceId}/sync`           | Guarded pull followed by detection and rescan     |
| `POST` | `/api/knowledge/workspaces/{workspaceId}/enrich`         | Confirmed process execution followed by rescan    |

Path parameters use URL escaping and are resolved as opaque values. Query responses use empty arrays rather than `null`.

### Action Requests

```json
{
  "confirm": true
}
```

Sync forwards `confirm` into the existing `GitOperationInput`. Enrich rejects requests unless `confirm` is true. Enrich configuration is read from the persisted workspace; clients cannot submit an executable or arguments in the action request.

## Sync Coordination

- Resolve the workspace through the registry.
- Call the existing application Git pull service to preserve dirty-tree and error behavior.
- Do not rescan when pull fails or confirmation is required.
- After success, rerun detection for all configured sources and atomically replace all Knowledge entries for the workspace.
- Remove entries for roots that no longer qualify only after the full post-pull detection succeeds.
- Return Git operation output together with Wiki summaries and warnings.

## Enrichment Execution

- Require enabled Knowledge settings and a non-empty executable.
- Show configuration through workspace APIs, but never expose environment values.
- Start with `exec.CommandContext` and set `Cmd.Dir` to the registered workspace root.
- Pass the configured argument array unchanged.
- Inherit only the server's existing environment; do not accept request-supplied environment variables.
- Capture bounded combined stdout and stderr.
- Enforce a fixed timeout and terminate the process group on Unix so child processes do not remain running.
- Treat non-zero exit, timeout, and start failure as failed actions and do not rescan.
- Record audit operation `knowledge_enrich` with workspace ID, status, duration, and sanitized summary.
- Do not attempt to revert files after failure.

## Graph Contract

Graph node IDs are page slugs within the selected Wiki. Nodes include title, domain, page type, roles, topics, path, and inbound/outbound counts. Directed edges use `sourceSlug -> targetSlug` and are deduplicated. Self-links remain visible. Unresolved links are excluded from edges and represented by warnings.

The endpoint applies node and edge response budgets. If truncated, it returns total counts and `truncated: true`; it selects nodes deterministically by normalized domain, title, and slug.

## Error Mapping

| Condition                         | Status | Behavior                                                       |
|-----------------------------------|--------|----------------------------------------------------------------|
| Workspace, Wiki, or page missing  | 404    | Stable JSON error and recovery hint                            |
| Unsafe root or page path          | 400    | Reject before filesystem access                                |
| Enrich not configured             | 409    | Link user to workspace Knowledge settings                      |
| Confirmation required             | 409    | Reuse Git confirmation shape or Enrich-specific confirmation   |
| Pull or process failure           | 422    | Return bounded operation result and preserve prior index       |
| Index read or persistence failure | 500    | Return recovery hint without exposing absolute sensitive paths |

## Verification

- Unit-test front matter normalization and link extraction.
- Unit-test deterministic duplicate handling, backlinks, and graph edge creation.
- Test traversal, symlink escape, `.git`, ignored path, file size, and aggregate budgets.
- Test atomic persistence and preservation after failed scans.
- Test API response shapes and status mapping.
- Test Sync for clean, dirty-confirmed, dirty-unconfirmed, pull failure, and post-pull scan failure.
- Test Enrich configuration, literal arguments, missing executable, non-zero exit, timeout, output truncation, audit records, and post-success rescan.
