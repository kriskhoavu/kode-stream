# Frontend Design: Feature Module Refactor

## Overview

The frontend should keep the same rendered UI and workflows while moving state and helpers out of large page files. Pages should become feature composition points. Hooks should own side effects. Shared modules should own reusable pure helpers and UI controls.

## Current Technical Debt

| Module                   | Debt                                                                                 | Refactor Direction                                                             |
|--------------------------|--------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|
| `web/src/App.tsx`        | Routing, theme, workspace state, stale polling, storage, and shell rendering combine | Move route helpers and app state to `web/src/app` hooks                        |
| `KanbanPage.tsx`         | Board loading, filters, drawer state, editor behavior, Git behavior, and helpers mix | Split board, filters, cards, drawer, hooks, and pure helpers                   |
| `ItemWorkspacePage.tsx`  | File loading, autosave, metadata, Git, diff parsing, resizing, and rendering combine | Split editor, panels, Git hook, metadata hook, diff helpers, file tree helpers |
| `WorkspacesPage.tsx`     | Workspace forms, source chips, source settings editor, directory picker, helpers mix | Split workspace form, source field, source settings feature, and helpers       |
| `web/src/styles/app.css` | Global stylesheet owns all layout and feature styles                                 | Move styles by feature after components are split; keep class names first      |
| `web/src/lib/api.ts`     | Endpoint groups and normalization share one object                                   | Split by resource under `shared/api` while exporting the same `api` facade     |

## Target Modules

| Module                                                | Responsibility                                                 |
|-------------------------------------------------------|----------------------------------------------------------------|
| `web/src/app/router.ts`                               | `routeFromLocation`, path generation, navigation helpers       |
| `web/src/app/useAppState.ts`                          | Workspaces, active workspace, refresh keys, stale state, theme |
| `web/src/shared/api/client.ts`                        | Fetch wrapper and error handling                               |
| `web/src/shared/api/workspaces.ts`                    | Workspace and source structure endpoints                       |
| `web/src/shared/api/items.ts`                         | Item, file, metadata, status, and diff endpoints               |
| `web/src/shared/api/git.ts`                           | Git endpoints and response normalization                       |
| `web/src/shared/domain/status.ts`                     | Status labels and order                                        |
| `web/src/shared/domain/files.ts`                      | File tree traversal and selected file helpers                  |
| `web/src/shared/domain/diff.ts`                       | Git diff parsing and diff helpers                              |
| `web/src/features/kanban/useKanban.ts`                | Board data loading, filters, move actions, drawer state        |
| `web/src/features/item-workspace/useItemWorkspace.ts` | Item detail, files, autosave, metadata, diff, Git state        |
| `web/src/features/workspaces/useWorkspaces.ts`        | Workspace form and source settings state                       |

## Component Split

| Current File            | Extracted Components                                                                       |
|-------------------------|--------------------------------------------------------------------------------------------|
| `KanbanPage.tsx`        | `KanbanToolbar`, `FacetMenu`, `SelectedFilters`, `KanbanLane`, `ItemCard`, `PreviewDrawer` |
| `ItemWorkspacePage.tsx` | `WorkspaceToolbar`, `FileTree`, `MarkdownPanel`, `DiffPanel`, `MetadataPanel`, `GitPanel`  |
| `WorkspacesPage.tsx`    | `WorkspaceForm`, `SourcesField`, `WorkspaceCard`, `SourceStructureDialog`                  |
| `App.tsx`               | `LeftNav`, `TopBar`, `WorkspaceSwitcher`, `StaleNotice`                                    |

## State Management

| State Area       | Target Owner                 | Notes                                                               |
|------------------|------------------------------|---------------------------------------------------------------------|
| Active workspace | `useAppState`                | Continue persisting `activeWorkspaceId` in local storage            |
| Route            | `router.ts` plus app shell   | Keep browser history paths unchanged                                |
| Theme            | `useAppState` or `useTheme`  | Keep `document.documentElement.dataset.theme` behavior              |
| Stale state      | `useAppState`                | Keep `/api/state` polling and storage broadcast behavior            |
| Kanban filters   | `useKanbanFilters`           | Pure filter helpers stay separately testable                        |
| Editor autosave  | `useAutosaveFile`            | Keep debounce timing and stale hash behavior unchanged              |
| Metadata editing | `useItemMetadata`            | Keep explicit save and dirty navigation guard                       |
| Git operations   | `useGitPanel`                | Keep confirmations and selected path behavior unchanged             |
| Source settings  | `useSourceStructureSettings` | Keep compatibility field inference and validation display unchanged |

## Frontend Test Additions

| Area       | Tests                                                                       |
|------------|-----------------------------------------------------------------------------|
| Router     | Path to route parsing and route to path generation                          |
| App state  | Active workspace fallback, stale state detection, storage broadcast         |
| Kanban     | Filter helpers, source labels, status move command behavior                 |
| Workspace  | Autosave state transitions, dirty metadata guard, file state map            |
| Diff       | `parseGitDiff` for add, delete, rename, and multi-file diffs                |
| Workspaces | Source parsing, source settings compatibility inference, dropped path parse |

## Visual Stability Rules

- Keep existing class names until the component split is complete.
- Move CSS without changing selector meaning.
- Do not change layout dimensions, labels, icons, colors, or breakpoints during PM-003 refactors.
- Use screenshots only for verification, not for redesign.
- Prefer pure helper extraction before JSX movement.

