# Frontend Design: Render And Feature Architecture Review

## Overview

The frontend should keep the current visual behavior while reducing main-thread render cost, splitting page orchestration into feature controllers, and making future views easier to extend. PM-015 focuses on Kanban, Item Workspace, Workspace Explorer, Workspace management, content viewer, and file editor flows.

## Current Implementation Review

| Component                                 | Current Shape                                                            | Risk                                                                      |
|-------------------------------------------|--------------------------------------------------------------------------|---------------------------------------------------------------------------|
| `web/src/pages/KanbanPage.tsx`            | Board data loading, branch state, filters, drag/drop, dialogs, drawer UI | Large state surface makes render and behavior changes expensive           |
| `web/src/pages/ItemWorkspacePage.tsx`     | File tree, editor, metadata, Git, diff, search, layout in one component  | Hidden panels still share heavy page state and many effects               |
| `web/src/pages/WorkspaceExplorerPage.tsx` | Tree, search, editor, mutation dialogs, branch switch, inspector         | UI and controller logic are mixed                                         |
| `useWorkspaceExplorer`                    | Cache, expansion, Git state, decorations, selection, mode, and refresh   | One hook owns too many reasons to re-render                               |
| `ContentViewer` and renderers             | Lazy modules exist; Markdown and source rendering are still main-thread  | Large files can pause interactions or typing                              |
| `useFileEditorSession`                    | Reusable autosave hook exists                                            | Needs stronger stale/save lifecycle handling for multiple editor surfaces |

## Target Patterns

| Pattern             | Frontend Use                                                                                          |
|---------------------|-------------------------------------------------------------------------------------------------------|
| Controller Hook     | `useKanbanController`, `useItemWorkspaceController`, `useExplorerController` own effects and commands |
| Adapter             | Content renderers expose a common async renderer contract                                             |
| Strategy            | Viewer mode and file-size thresholds choose rich, source, or paused rendering                         |
| View Model Selector | Derived rows, grouped cards, Git state maps, and filters are memoized pure helpers                    |
| Compound Components | Page regions become testable layout pieces without owning data fetching                               |

## Render Mechanism

```text
FileContent
  -> classify kind and size
  -> choose renderer adapter
  -> defer heavy parse/highlight work
  -> cache bounded result by file hash/content hash
  -> render toolbar, loading, error, or preview state
```

### Renderer Requirements

| Renderer        | Requirement                                                                       |
|-----------------|-----------------------------------------------------------------------------------|
| Markdown        | Cache by content hash, cancel stale render results, preserve sanitization         |
| Source Code     | Highlight by chunk or visible range for large files; avoid per-line full rerender |
| Structured Data | Parse once, render bounded tree nodes, keep expand/collapse state local           |
| HTML            | Keep sanitized iframe behavior and sandbox restrictions                           |

## Feature Decomposition

### Kanban

| Module                                | Responsibility                                       |
|---------------------------------------|------------------------------------------------------|
| `features/kanban/useKanbanBoard.ts`   | Branch load, refresh, scan, item list, pending moves |
| `features/kanban/useKanbanFilters.ts` | Filters, saved filters, facet view models            |
| `features/kanban/KanbanBoard.tsx`     | Board layout and lanes                               |
| `features/kanban/KanbanDrawer.tsx`    | Preview/raw/diff drawer                              |
| `features/kanban/NewItemDialog.tsx`   | New item workflow                                    |

### Item Workspace

| Module                                            | Responsibility                        |
|---------------------------------------------------|---------------------------------------|
| `features/item-workspace/useItemWorkspace.ts`     | Item, files, selected file, search    |
| `features/item-workspace/useItemGitPanel.ts`      | Git status, commit, branch, pull/push |
| `features/item-workspace/MetadataPanel.tsx`       | Metadata draft and save UI            |
| `features/item-workspace/FileWorkspaceLayout.tsx` | Panels, resize state, tabs            |

### Workspace Explorer

| Module                                                | Responsibility                         |
|-------------------------------------------------------|----------------------------------------|
| `features/workspace-explorer/useExplorerTree.ts`      | Expansion, directory cache, rows       |
| `features/workspace-explorer/useExplorerSelection.ts` | Selection, route sync, expand-to-path  |
| `features/workspace-explorer/useExplorerGitState.ts`  | Path Git state and branch refresh      |
| `features/workspace-explorer/ExplorerTree.tsx`        | Tree rendering and keyboard navigation |
| `features/workspace-explorer/ExplorerEditor.tsx`      | Preview/raw/diff editor surface        |

## State Management

No new global state library is required. Prefer local controller hooks plus pure selectors. Keep `useAppState` as the app-wide owner for workspace list, active workspace, route, theme, and stale content.

| State                         | Owner                                          |
|-------------------------------|------------------------------------------------|
| App route/theme/workspaces    | `web/src/app/useAppState.ts`                   |
| Kanban branch/items/filters   | `features/kanban` controller hooks             |
| Explorer tree cache/selection | `features/workspace-explorer` controller hooks |
| Editor autosave session       | `features/file-editor/useFileEditorSession.ts` |
| Content preview mode/cache    | `features/content-viewer` renderer adapters    |

## Design Decisions

| Decision                                       | Rationale                                                                       |
|------------------------------------------------|---------------------------------------------------------------------------------|
| Preserve class names during extraction         | Avoid accidental visual regressions while files move                            |
| Keep renderer modules lazy-loaded              | Current code-splitting is useful and should be extended, not removed            |
| Defer heavy preview work before virtualization | Main-thread pauses are the immediate issue; virtualization can follow if needed |
| Use pure selectors for view models             | Enables focused tests and reduces page rerender cost                            |
| Keep autosave shared                           | Item Workspace and Explorer already use the same edit session pattern           |

## Verification Strategy

| Area              | Verification Command                                                                                                                           |
|-------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| TypeScript        | `rtk npm run typecheck`                                                                                                                        |
| Frontend tests    | `rtk npm test -- --run`                                                                                                                        |
| Build             | `rtk npm run build`                                                                                                                            |
| Focused renderers | `rtk npm test -- --run web/src/features/content-viewer web/src/features/file-editor`                                                           |
| Focused pages     | `rtk npm test -- --run web/src/pages/KanbanPage.test.tsx web/src/pages/ItemWorkspacePage.test.ts web/src/pages/WorkspaceExplorerPage.test.tsx` |
