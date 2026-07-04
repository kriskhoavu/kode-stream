# PM-022: Structured Knowledge Wiki

PM-022 adds a reusable, read-only Knowledge page for structured Markdown Wikis inside registered workspaces. Users can browse domain indexes, read pages, follow links and backlinks, inspect a page graph, refresh the local index, pull remote changes, and explicitly run a configured enrichment command. Wiki pages remain separate from Kanban items and continue to be edited through Workspace Explorer.

## Related Plans

| Ticket                        | Relationship       | Key Context                                                                                 |
|-------------------------------|--------------------|---------------------------------------------------------------------------------------------|
| [PM-005](../PM-005/README.md) | Navigation pattern | Added stable application routes, search navigation, and keyboard-friendly result selection  |
| [PM-006](../PM-006/README.md) | Rendering baseline | Added the secure shared Markdown content viewer                                             |
| [PM-007](../PM-007/README.md) | Explorer baseline  | Added guarded workspace file access, preview, editing, and route-addressable file selection |
| [PM-009](../PM-009/README.md) | Discovery pattern  | Added bounded source traversal and content search                                           |
| [PM-013](../PM-013/README.md) | Branch boundary    | Distinguished working-tree reads from non-checkout branch snapshots                         |
| [PM-015](../PM-015/README.md) | Architecture       | Defined application-service and frontend feature-controller ownership                       |

## Goals

- Detect compatible Wiki roots below configured workspace sources.
- Build a local index from page metadata and links without changing workspace files.
- Browse pages by workspace, Wiki root, and domain hierarchy.
- Read sanitized Markdown and navigate outgoing links and backlinks.
- Visualize page relationships in an interactive graph.
- Rescan the working tree without Git operations.
- Pull through the existing guarded Git workflow and then rebuild the Wiki index.
- Run an optional workspace-configured enrichment command as a separate confirmed action.
- Keep Wiki pages out of the Kanban item index.

## Out Of Scope

- Inline Wiki editing.
- AI chat, embeddings, semantic search, or vector storage.
- Automatic background pull or enrichment.
- Running arbitrary shell command strings.
- Rendering `sourceRef` values as graph nodes.
- Reading Wiki snapshots from branches that are not checked out.
- Replacing Workspace Explorer or its Markdown editor.

## Glossary

| Term              | Meaning                                                                                  | Maps To (code)                     |                 |
|-------------------|------------------------------------------------------------------------------------------|------------------------------------|-----------------|
| Knowledge Page    | Top-level application surface for structured Wiki content                                | `KnowledgePage`                    |                 |
| Wiki Root         | Configured source directory containing `index.md` and compatible Markdown page metadata  | `KnowledgeWiki`                    |                 |
| Wiki Page         | Markdown file with required `slug` and `title` front matter                              | `KnowledgePageSummary`             |                 |
| Domain            | Relative parent directory used to group Wiki pages                                       | `domain`                           |                 |
| Wiki Link         | `[[slug]]` or `[[slug                                                                    | label]]` reference to another page | `KnowledgeLink` |
| Backlink          | Reverse relationship from a target page to a page that links to it                       | `backlinks`                        |                 |
| Knowledge Index   | App-owned persisted metadata, warnings, and graph relationships                          | `knowledge-index.yaml`             |                 |
| Rescan            | Rebuild the Knowledge index from the current working tree without changing Git state     | `RescanKnowledge`                  |                 |
| Sync              | Guarded Git pull followed by a Knowledge rescan                                          | `SyncKnowledge`                    |                 |
| Enrich            | Confirmed execution of a configured executable and arguments followed by a rescan        | `EnrichKnowledge`                  |                 |
| Detection Warning | Non-fatal duplicate slug, missing target, malformed page, or unreadable source condition | `KnowledgeWarning`                 |                 |

## Components

| Layer    | Component                        | Purpose                                                                            |
|----------|----------------------------------|------------------------------------------------------------------------------------|
| Backend  | `internal/knowledge`             | Detect Wiki roots, parse pages, resolve links, create backlinks, and persist index |
| Backend  | `internal/application/knowledge` | Coordinate registry, Git, enrichment, audit, and Knowledge indexing                |
| Backend  | Knowledge API handlers           | Expose Wiki queries and explicit actions                                           |
| Backend  | Workspace registry               | Store optional enrichment executable and argument configuration                    |
| Frontend | `features/knowledge`             | Own queries, selection, route state, actions, and graph adapters                   |
| Frontend | `KnowledgePage`                  | Compose Browse, Read, and Graph views                                              |
| Frontend | Shared `ContentViewer`           | Render selected Markdown through the existing sanitized pipeline                   |
| Frontend | Workspace Explorer route         | Open the selected Wiki file for editing                                            |

## Data Flow

```text
Open /knowledge
  -> frontend loads detected Wikis for registered workspaces
  -> user selects one Wiki and optional page slug
  -> Knowledge service reads its app-owned index
  -> API returns page hierarchy, metadata, warnings, and graph
  -> frontend loads selected Markdown through a guarded Knowledge page endpoint
  -> ContentViewer renders the page and link navigation updates the route

Rescan
  -> validate the registered workspace and detected Wiki root
  -> walk Markdown files below the root with fixed budgets
  -> parse metadata and links, resolve backlinks, and collect warnings
  -> atomically replace only that workspace and Wiki root in the Knowledge index

Sync
  -> use existing guarded Git pull behavior
  -> stop on dirty-tree confirmation or Git failure
  -> rescan all detected Wikis in the workspace after a successful pull

Enrich
  -> require configured executable and argument array
  -> require explicit user confirmation
  -> run without a shell from the workspace root with timeout and output limits
  -> write an audit event
  -> rescan all detected Wikis after successful completion
```

## Detection Contract

A configured workspace source is a Wiki root when:

- the source contains `index.md`; and
- at least one Markdown file below it contains non-empty `slug` and `title` front matter.

Detection is limited to `WorkspaceConfig.sources`. Nested source roots are evaluated independently but the same physical root is returned only once. `.git`, ignored files, path traversal, and symlinks escaping the workspace remain excluded. `index.md` is evidence of the format, but its generated tables are not parsed as the application index.

## Metadata Contract

| Field         | Required | Behavior                                                        |
|---------------|----------|-----------------------------------------------------------------|
| `slug`        | Yes      | Stable page identity within one Wiki root                       |
| `title`       | Yes      | Display name and search text                                    |
| `pageType`    | No       | Filter and metadata badge                                       |
| `roles`       | No       | Comma-separated string or YAML list normalized to a string list |
| `topics`      | No       | Comma-separated string or YAML list normalized to a string list |
| `summary`     | No       | Reader and browse description                                   |
| `sourceRef`   | No       | One string or multiline source-reference metadata               |
| `sourceCount` | No       | Non-negative source count; invalid values create a warning      |

The first valid page for a slug is indexed deterministically by normalized relative path. Later duplicates remain visible as warnings and do not replace the first page.

## Safety Boundaries

- Resolve every path below the registered workspace and configured source root.
- Reject traversal and symlink escape before reading content.
- Never expose `.git` content.
- Treat Markdown and front matter as untrusted input.
- Invoke enrichment with `exec.CommandContext(executable, args...)`; never invoke a shell.
- Set a fixed execution timeout and combined-output byte limit.
- Do not place credentials or environment values in command output or audit records.
- Reuse existing Git dirty-tree confirmation instead of bypassing it.
- Write only the app-owned Knowledge index during scans; enrichment owns any workspace changes it creates.

## Design Decisions

| Decision                                | Alternatives Considered                 | Rationale                                                                           |
|-----------------------------------------|-----------------------------------------|-------------------------------------------------------------------------------------|
| Add a dedicated Knowledge page          | Extend Workspace Explorer               | Explorer is file-oriented; Knowledge is metadata- and relationship-oriented         |
| Use “Knowledge” as the navigation label | Wiki, LLM Wiki                          | The content is useful independently of its generation mechanism                     |
| Auto-detect index plus front matter     | Front matter only, explicit roots only  | Avoids matching arbitrary docs while remaining reusable                             |
| Persist a separate Knowledge index      | Add Wiki pages to the item index        | Wiki pages must not become Kanban cards or inherit item lifecycle fields            |
| Keep v1 read-only                       | Add an inline editor                    | Existing Explorer already owns safe editing and diff behavior                       |
| Resolve Wiki and Markdown links         | Resolve only generated index links      | Page content is the relationship source of truth                                    |
| Keep Enrich separate from Sync          | Run enrichment automatically after pull | External content generation must be visible, configured, and explicitly confirmed   |
| Configure executable and argument array | Store a shell command string            | Structured process execution avoids shell injection and quoting ambiguity           |
| Build graph data on the backend         | Parse links in the browser              | One parser supplies browse, backlinks, warnings, and graph consistently             |
| Add an interactive graph package        | Implement pan and zoom manually         | Accessibility and interaction behavior should use a maintained graph implementation |

## Acceptance Criteria

- Discovery's configured `docs` source is detected as a Wiki without project-specific code.
- Pages with valid `slug` and `title` metadata appear in a domain hierarchy.
- Selected Markdown renders through the existing secure viewer.
- Wiki links navigate to indexed pages and backlinks identify referring pages.
- Duplicate slugs, broken links, and malformed pages show non-blocking warnings.
- Graph nodes and edges match the resolved page-link index and support selection, pan, and zoom.
- “Open in Explorer” deep-links to the selected workspace-relative file.
- Rescan does not invoke Git or modify workspace content.
- Sync uses guarded Git pull and rescans only after pull succeeds.
- Enrich is disabled until configured, requires confirmation, is shell-free and bounded, creates an audit event, and rescans after success.
- Existing Kanban, Explorer, item index, Git, search, and release behavior remain unchanged.

## Implementation Status

Implemented on `feature/PM-022-structured-knowledge-wiki` in nine phase commits.

- The real Discovery `docs` source indexed 50 pages, 50 graph nodes, 131 directed edges, and 5 non-blocking content warnings on 2026-07-04.
- Live API verification covered Wiki listing, `offer-overview` detail, graph response, Rescan, and configured shell-free Enrich using `/bin/echo` against temporary Plan Manager data.
- Sync confirmation/failure preservation, process start failure, non-zero exit, timeout, output truncation, and audit behavior are covered by disposable fixtures.
- Backend verification: 256 tests passed across 34 Go packages.
- Frontend verification: 170 tests passed across 43 test files; TypeScript and the production build passed.
- The lazy Knowledge graph bundle is 185.64 kB (60.26 kB gzip); initial Knowledge Browse is 14.94 kB (5.18 kB gzip).
- The in-app browser was unavailable during final verification, so desktop/mobile visual inspection could not be performed. Responsive CSS, navigation, keyboard behavior, reader, graph, warnings, actions, settings, and Explorer routing are covered by focused interaction tests and the production build.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
