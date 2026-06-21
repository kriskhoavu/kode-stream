# PM-009: Scoped Content Search

PM-009 adds text-content search to item details and Workspace Explorer. Item details search only inside the selected item directory. Explorer defaults to configured workspace sources such as `plans` and `docs`, keeps an explicit All files mode, and searches only the directories included by the active mode.

## Related Plans

| Ticket                        | Relationship    | Key Context                                                                     |
|-------------------------------|-----------------|---------------------------------------------------------------------------------|
| [PM-007](../PM-007/README.md) | Parent feature  | Added the global workspace tree, file preview, editor, and guarded file access  |
| [PM-008](../PM-008/README.md) | Parent feature  | Added unloaded path-name search, ancestor expansion, Git markers, and mutations |
| [PM-006](../PM-006/README.md) | Related feature | Defines supported text formats, binary checks, and bounded file reads           |
| [PM-005](../PM-005/README.md) | Related feature | Added global item search and debounced search interaction patterns              |

### What Parent Plans Established

- **Workspace Explorer**: the global `/explorer` route across registered workspaces.
- **Workspace Source**: a configured root such as `plans` or `docs` in `WorkspaceConfig.sources`.
- **Workspace Tree Entry**: one real file or directory below a registered workspace.
- **Item Workspace**: the details page opened from Kanban or other item links.
- **Path Search**: PM-008 name/path search, which stays separate from content search.
- **Content Viewer**: the PM-006 renderer for supported text formats.
- **Ignored Entry**: a Git-ignored path excluded unless Show ignored is enabled.

## Goals

- Search file contents recursively inside the selected Kanban item directory.
- Default Explorer tree mode to configured workspace sources.
- Keep the current full workspace tree through an explicit All files mode.
- Search Explorer content only inside the directories included by the active tree mode.
- Search one selected workspace or all registered workspaces.
- Show file path, line number, matching snippet, and workspace or item context.
- Open a result in the existing viewer or editor.
- Preserve path safety, ignored-file behavior, and bounded local performance.

## Out Of Scope

- Replacing PM-008 path-name search.
- Regular-expression search.
- Replace-all or content mutation from search results.
- Searching binary or unsupported content.
- Searching `.git` or outside-workspace symlinks.
- Building a persistent search index or database.
- Searching Git history, deleted files, or remote branches.
- Fuzzy semantic search.

## Glossary

| Term                    | Meaning                                                                  | Code Target                    |
|-------------------------|--------------------------------------------------------------------------|--------------------------------|
| Content Search          | Case-insensitive literal text search inside supported text files         | `WorkspaceContentSearch`       |
| Content Search Result   | One matching line and snippet inside one workspace-relative file         | `WorkspaceContentSearchResult` |
| Item Search Scope       | The selected item's guarded directory                                    | `item`                         |
| Explorer Tree Mode      | Configured Sources or All Files                                          | `ExplorerTreeMode`             |
| Configured Sources Mode | Tree and search limited to `WorkspaceConfig.sources`                     | `sources`                      |
| All Files Mode          | Tree and search cover the full guarded workspace root                    | `all`                          |
| Search Root             | One validated directory included in a content-search request             | `WorkspaceContentSearchRoot`   |
| Match Snippet           | Bounded text around a match with line and column metadata                | `snippet`                      |
| Search Budget           | Limits for files visited, bytes read, matches returned, and elapsed time | `WorkspaceContentSearchBudget` |

## Scope Rules

| Surface             | Default Scope                          | Optional Scope                          |
|---------------------|----------------------------------------|-----------------------------------------|
| Kanban item details | Selected item directory                | None                                    |
| Explorer            | Configured sources in all workspaces   | One workspace within configured sources |
| Explorer All files  | Full root of all registered workspaces | One full workspace root                 |

- Configured Sources mode uses `WorkspaceConfig.sources` exactly.
- A workspace with no configured sources shows an empty source tree and an All files action.
- All Files mode preserves PM-008 full-tree behavior.
- Ignored paths stay excluded unless Show ignored is enabled.
- Hidden non-ignored paths remain searchable in All Files mode.

## Data Flow

```text
Item details content search
  -> resolve item and registered workspace
  -> use the guarded item directory as the only search root
  -> scan supported text files within the search budget
  -> return line matches and snippets
  -> selecting a result opens the item file

Explorer content search
  -> read tree mode: sources or all
  -> resolve roots for every selected workspace
  -> scan only those roots within one shared request budget
  -> selecting a result expands ancestors and opens the file
```

## Components

| Layer    | Component                                   | Purpose                                                       |
|----------|---------------------------------------------|---------------------------------------------------------------|
| Backend  | `internal/workspacefiles/content_search.go` | Scan guarded roots and return bounded literal matches         |
| Backend  | `internal/application/contentsearch`        | Resolve item, workspace, tree mode, roots, and shared budgets |
| Backend  | Content-search API routes                   | Expose item-scoped and Explorer-scoped content search         |
| Frontend | `features/content-search`                   | Own query, debounce, scope, stale-request, and result state   |
| Frontend | `ItemContentSearch`                         | Search and open matches inside one item directory             |
| Frontend | `ExplorerContentSearch`                     | Search matches inside the active Explorer tree mode           |
| Frontend | `ExplorerTreeModeControl`                   | Switch between Configured Sources and All Files               |

## Design Decisions

| Decision                                  | Alternatives Considered           | Rationale                                                         |
|-------------------------------------------|-----------------------------------|-------------------------------------------------------------------|
| Default Explorer to configured sources    | Keep full workspace as default    | Planning work normally lives in known roots and needs less noise  |
| Keep an explicit All Files mode           | Remove full workspace browsing    | Preserves PM-008 behavior and supports repository-wide inspection |
| Separate path and content search          | Merge both into one result list   | Users can choose between navigation and text matches              |
| Use bounded in-process scanning           | Shell out to `rg` or add an index | Keeps the binary self-contained and reuses Go path/content guards |
| Search literal text first                 | Support regex immediately         | Literal search is predictable and easier to bound safely          |
| Return line-oriented snippets             | Return complete matching files    | Results stay small and useful without exposing large file content |
| Share one budget across multi-root search | Reset limits for every root       | One request has predictable total work                            |

## Documents

- [Ticket](ticket.md)
- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
