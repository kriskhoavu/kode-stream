# PM-008: Explorer Productivity Enhancements

## Status

Done

## User Story

As a developer, I want to search, organize, and understand repository files from Explorer so that routine workspace navigation does not require switching to another tool.

## Problem

PM-007 supports safe browsing, preview, Markdown editing, and revert. It does not search unloaded paths, show Git state on tree rows, or create and rename workspace content.

## Scope

### Repository Path Search

- Search file and directory names across one or all registered workspaces.
- Return bounded results without requiring tree expansion.
- Respect `.git`, symlink, hidden, and ignored-file rules.
- Open a result by expanding its ancestor path and updating route selection.
- Cancel stale searches while the query changes.

### Git Decorations

- Batch workspace Git status into a normalized path map.
- Show modified, added, deleted, renamed, untracked, and conflicted states.
- Decorate loaded files and directories without one Git call per row.
- Refresh decorations after save, rename, revert, scan, and manual refresh.

### Guarded Create And Rename

- Create a Markdown file with a required `.md` or `.markdown` extension.
- Create a directory inside a registered workspace.
- Rename a file or directory inside the same workspace.
- Reject empty names, absolute paths, traversal, `.git`, outside symlinks, and existing destinations.
- Do not overwrite content.
- Preserve Git history through normal filesystem rename detection.
- Audit successful and blocked operations.
- Refresh affected directory caches and configured item sources.

## Acceptance Criteria

- [x] Search finds matching unloaded paths in the selected scope.
- [x] Search results include workspace, relative path, type, and match context.
- [x] Search never returns `.git` or outside-symlink content.
- [x] Ignored results follow the Explorer ignored-file preference.
- [x] Selecting a result expands its ancestors and opens the target.
- [x] Loaded tree rows show normalized Git state without per-row Git processes.
- [x] Conflict state is visually distinct and keyboard-accessible.
- [x] Users can create Markdown files and directories from a selected directory.
- [x] Users can rename a selected file or directory after confirmation.
- [x] Create and rename reject invalid, protected, or occupied destinations.
- [x] Failed operations do not leave partial files or stale selected paths.
- [x] Successful operations update route selection, tree caches, audit history, and affected item data.
- [x] Existing PM-007 autosave, stale-hash, diff, revert, and responsive behavior remains unchanged.
- [x] Backend, frontend, and production-build checks pass.

## API Proposal

| Method | Endpoint                               | Purpose                               |
|--------|----------------------------------------|---------------------------------------|
| `GET`  | `/api/workspaces/files/search`         | Search paths across workspace scope   |
| `POST` | `/api/workspaces/{id}/directories`     | Create one guarded directory          |
| `POST` | `/api/workspaces/{id}/files`           | Create one guarded Markdown file      |
| `POST` | `/api/workspaces/{id}/paths/rename`    | Rename one guarded file or directory  |
| `GET`  | `/api/workspaces/{id}/git/path-status` | Return batched normalized path states |

## Safety And Performance Requirements

- Reuse `internal/workspacefiles` path resolution and protected-path rules.
- Use atomic exclusive creation for new files.
- Keep rename inside one resolved workspace root.
- Bound search result count, traversal work, and request time.
- Skip ignored directories before descending when ignored results are disabled.
- Run Git status once per workspace refresh, not once per tree row.
- Keep tree rendering dependency-free until measured row counts justify virtualization.

## Dependencies

- PM-007 Workspace Explorer.
- Existing audit store and targeted workspace refresh behavior.
- Existing Git adapter status parsing.

## Definition Of Done

- API contracts and error mappings have regression tests.
- Path, collision, symlink, ignore, and partial-write safeguards have tests.
- Search and Git decoration helpers have deterministic unit tests.
- Explorer create, rename, search, and keyboard flows have component tests.
- Architecture and PM-008 implementation documents reflect final behavior.
