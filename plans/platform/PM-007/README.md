# PM-007: Workspace Explorer

PM-007 adds a global filesystem explorer for every registered workspace. Kanban remains scoped to one active workspace. Explorer shows each repository as a root, loads its real directories and files lazily, previews supported content with PM-006, and edits Markdown with the same autosave and stale-write behavior as item details.

## Related Plans

| Ticket                        | Relationship    | Key Context                                                                                 |
|-------------------------------|-----------------|---------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature  | Created workspace registration, item file trees, and item detail routes                     |
| [PM-002](../PM-002/README.md) | Parent feature  | Added guarded Markdown editing, autosave, diff, revert, and Git workflows                   |
| [PM-003](../PM-003/README.md) | Foundation      | Established application services, path guards, feature modules, and shared frontend helpers |
| [PM-004](../PM-004/README.md) | Related feature | Added workspace health, warnings, activity, and safety signals                              |
| [PM-005](../PM-005/README.md) | Related feature | Added all-workspace search, recent items, and keyboard navigation                           |
| [PM-006](../PM-006/README.md) | Parent feature  | Added secure Markdown, HTML, JSON, YAML, code, and text rendering                           |

### What Existing Plans Established

- **Workspace**: a registered local Git repository.
- **Active Workspace**: the workspace currently shown by Kanban, Items, and Branches.
- **Item**: an indexed planning folder or configured docs card.
- **Content Viewer**: the secure renderer for supported text formats.
- **Raw Mode**: the existing Markdown editing surface.
- **Write Guard**: path, source, symlink, stale-hash, and Git safety checks.
- **Item Workspace**: the existing Preview, Raw, Diff, metadata, and Git workflow.

## Goals

- Show every registered workspace as a top-level tree root.
- Browse the real repository directory structure, not only indexed items.
- Load one directory at a time to remain responsive in large repositories.
- Enrich known item directories with status, identifier, branch, owner, and warning context.
- Preview supported files through PM-006.
- Edit Markdown with Preview, Raw, Diff, autosave, stale-hash protection, and revert.
- Support keyboard navigation, search, path breadcrumbs, and persisted expansion.
- Preserve current Kanban and item detail behavior.

## Out Of Scope

- Replacing or removing Kanban.
- Exposing `.git` internals.
- Editing binary files or non-Markdown text formats.
- Moving, renaming, creating, or deleting arbitrary files and directories.
- Running scripts or executable files.
- Editing item metadata or status from Explorer.
- Running commit, fetch, pull, push, or branch mutations from Explorer.
- Following symlinks outside a registered workspace.

## Glossary

| Term                 | Meaning                                                            | Code Target                 |
|----------------------|--------------------------------------------------------------------|-----------------------------|
| Workspace Explorer   | Global route containing all registered workspace roots             | `WorkspaceExplorerPage`     |
| Workspace Tree Entry | One real directory or file below a registered workspace            | `WorkspaceTreeEntry`        |
| Directory Listing    | One lazy page of immediate children for a directory                | `WorkspaceDirectoryListing` |
| Workspace File       | A file addressed by workspace ID and root-relative path            | `WorkspaceFileContent`      |
| Item Decoration      | Indexed item metadata attached to a matching directory path        | `ExplorerItemDecoration`    |
| Visible Tree         | Flattened expanded workspace rows rendered by the explorer         | `VisibleExplorerRow[]`      |
| Selection            | Current workspace, directory, or file path                         | `ExplorerSelection`         |
| Editor Session       | Shared preview, raw, diff, autosave, stale-write, and revert state | `useFileEditorSession`      |
| Ignored Entry        | A path excluded by Git ignore rules                                | `ignored`                   |
| Expansion State      | Persisted set of open workspace and directory node IDs             | `expandedNodeIds`           |

## Information Architecture

```text
Workspace Explorer
├── Kode Stream
│   ├── cmd/
│   ├── docs/
│   ├── internal/
│   ├── plans/
│   │   └── platform/
│   │       └── PM-007 Workspace Explorer/
│   ├── web/
│   ├── go.mod
│   └── package.json
└── Discovery
    ├── api/
    ├── customer/
    ├── docs/
    └── plans/
```

Known item directories remain real directory rows. The UI adds item status and context without replacing the filesystem hierarchy.

## Components

| Layer    | Component                             | Purpose                                                                |
|----------|---------------------------------------|------------------------------------------------------------------------|
| Backend  | `internal/workspacefiles`             | Guard workspace paths and list, read, write, diff, and revert files    |
| Backend  | `internal/application/workspacefiles` | Coordinate registry, file access, Git, audit, and refresh behavior     |
| Backend  | Workspace file API routes             | Expose lazy directory and guarded file operations                      |
| Frontend | `features/workspace-explorer`         | Own global tree, expansion, lazy directory cache, selection, and panes |
| Frontend | `features/file-editor`                | Share Preview, Raw, Diff, autosave, stale-write, and revert state      |
| Frontend | `WorkspaceExplorerPage`               | Compose tree, file workspace, and inspector                            |
| Frontend | `ExplorerTree`                        | Render real workspace directories and enriched item rows               |
| Frontend | `WorkspaceFileEditor`                 | Render selected file with the shared viewer and editor session         |

## Data Flow

```text
User opens /explorer
  -> existing workspace list becomes top-level roots
  -> existing item index provides optional directory decorations
  -> user expands a directory
  -> frontend requests only that directory's immediate children
  -> backend validates workspace root, path, symlinks, ignore rules, and .git exclusion
  -> user selects a file
  -> frontend loads classified file content
  -> PM-006 renders Preview
  -> Markdown Raw mode uses shared autosave and expected hash checks
  -> Diff and Revert use guarded workspace-relative Git operations
```

## Workspace File API

| Method | Endpoint                                      | Purpose                           |
|--------|-----------------------------------------------|-----------------------------------|
| `GET`  | `/api/workspaces/{id}/tree?path={path}`       | List immediate directory children |
| `GET`  | `/api/workspaces/{id}/files?path={path}`      | Read classified text content      |
| `PUT`  | `/api/workspaces/{id}/files?path={path}`      | Save Markdown with expected hash  |
| `GET`  | `/api/workspaces/{id}/files/diff?path={path}` | Read Git diff for one file        |
| `POST` | `/api/workspaces/{id}/files/revert`           | Revert one validated file path    |

The API is additive. Existing item file routes remain unchanged.

## Browse Policy

- Treat the registered Git root as the hard boundary.
- Never return `.git` or descendants.
- Never follow a symlink outside the workspace root.
- Hide Git-ignored entries by default.
- Allow an explicit Show ignored files toggle.
- Include hidden files except `.git` unless product testing shows excessive noise.
- Return immediate children only. Never recursively walk a workspace for one expansion.
- Return enough entry metadata to show directory, file kind, size, ignored state, and child availability.

## Edit Policy

- Use PM-006 for Markdown, HTML, JSON, YAML, code, and plain-text previews.
- Keep HTML, JSON, YAML, code, and plain text read-only.
- Allow Raw editing only when `FileContent.editable` is true.
- Match current detail autosave timing and status labels.
- Require `expectedHash` on saves.
- Reject stale content and show the existing recovery hint.
- Revert only the selected validated workspace-relative file.
- Refresh diff and Git state after successful save or revert.
- Record workspace file writes and reverts in the audit log.

## Row Design

| Node      | Primary Signal                 | Secondary Signal                                  | Actions                    |
|-----------|--------------------------------|---------------------------------------------------|----------------------------|
| Workspace | Repository name and health dot | Branch, path, change count, and root totals       | Open Kanban, collapse      |
| Directory | Folder name                    | Item status/context when the path matches an item | Collapse, copy path        |
| File      | File name and kind icon        | Size, Git state, and read-only/editable state     | Preview, copy path, reveal |

Folders stay visually quiet. Known item folders receive a small status rail and metadata line. Files use stable icons and compact Git markers. Hover actions use icons with tooltips.

## State And Persistence

- Store selected workspace and relative path in route query parameters.
- Persist expanded directory IDs for the global explorer.
- Cache directory listings by workspace ID, path, and ignored-file mode.
- Clear directory and file caches when `refreshKey` changes.
- Persist pane widths, collapsed inspector state, and ignored-file preference locally.
- Keep editor content in memory until autosave settles.
- Do not write Explorer preferences into registered workspaces.

## Performance Strategy

- Fetch immediate directory children only.
- Avoid recursive filesystem scans during initial loading.
- Batch Git-ignore checks for one directory listing.
- Load file content, diff, Git status, and health only when needed.
- Cache successful directory listings and selected files.
- Flatten only expanded nodes with memoized helpers.
- Keep the memoized visible row list dependency-free. Revisit virtualization above 1,000 visible rows or a 16 ms frame budget.
- Cancel or ignore stale requests after rapid selection changes.

## Design Decisions

| Decision                                         | Alternatives Considered                     | Rationale                                                                     |
|--------------------------------------------------|---------------------------------------------|-------------------------------------------------------------------------------|
| Make Explorer global and Kanban workspace-scoped | Make both views follow the active workspace | Explorer supports cross-repository browsing while Kanban manages one workflow |
| Show real workspace directories                  | Show only indexed item hierarchy            | The requested workflow includes all repository files and folders              |
| Load directories lazily                          | Return full recursive trees                 | Large repositories and generated folders must not block initial rendering     |
| Reuse PM-006                                     | Add another preview implementation          | Security and format behavior stay consistent                                  |
| Share editor session behavior                    | Copy item-detail autosave logic             | Details and Explorer must not drift                                           |
| Keep Markdown as the only editable format        | Edit all text formats                       | This matches current detail mode and existing write validation                |
| Hide ignored entries by default                  | Show every generated dependency directory   | Repository browsing stays useful and bounded                                  |
| Enrich real directories with item metadata       | Replace folders with virtual item nodes     | Filesystem truth remains clear while planning context stays visible           |

## Acceptance Criteria

- Explorer always shows every registered workspace as a top-level root.
- Expanding a workspace or directory loads its real immediate children.
- `.git` is never visible or addressable.
- Git-ignored entries are hidden by default and available through an explicit toggle.
- Symlinks cannot escape a registered workspace.
- Indexed item directories show status and useful planning context.
- Selecting a supported file renders it through PM-006.
- Markdown supports Preview, Raw, Diff, autosave, stale-write recovery, and revert like item details.
- Non-Markdown formats remain read-only.
- Keyboard users can navigate, expand, collapse, select, and open rows.
- Expansion, selection, pane widths, and ignored-file preference restore predictably.
- Opening Kanban from a workspace explicitly makes that workspace active.
- Existing Kanban, item detail, search, and Git workflows remain unchanged.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
