# Frontend Design: PM-009 Scoped Content Search

## Overview

Add shared content-search state and result rendering. Item details use a fixed item scope. Explorer adds a tree-mode control and passes that mode to both tree composition and content search.

## Types

```typescript
type ExplorerTreeMode = 'sources' | 'all';

interface ContentSearchSelection {
  workspaceId: string;
  itemId?: string;
  path: string;
  fileId?: string;
  lineNumber: number;
  columnStart: number;
  columnEnd: number;
}
```

## Components And Hooks

| Component / Hook          | Responsibility                                                |
|---------------------------|---------------------------------------------------------------|
| `useContentSearch`        | Debounce, stale-request protection, loading, results, errors  |
| `ContentSearchInput`      | Query input, clear action, and search status                  |
| `ContentSearchResults`    | Keyboard navigation, snippets, result counts, truncation      |
| `ItemContentSearch`       | Bind search to one item and open item files                   |
| `ExplorerTreeModeControl` | Switch Configured Sources and All Files                       |
| `ExplorerContentSearch`   | Bind search to tree mode, workspace scope, and ignored mode   |
| `ContentMatchContext`     | Show selected line and match metadata near the file workspace |

## Explorer Tree Modes

### Configured Sources

- This is the default when no saved preference exists.
- Workspace roots render configured source directories as their immediate children.
- Expanding a source uses the existing lazy directory API.
- Root files and unconfigured directories are not shown.
- Use separate cache keys from All Files mode.
- A missing source renders a retryable warning row.

### All Files

- Preserve the current PM-008 tree behavior.
- Workspace roots load the real root directory.
- Keep existing expansion and selection behavior.
- Persist the mode in local storage.
- Add `mode` to the Explorer route query for shareable scope.

## Search Surfaces

### Item Details

- Place content search above the item file tree.
- Do not replace the existing item file navigation.
- Results appear in a bounded panel below the input.
- Opening a result saves pending Markdown before switching files.
- Resolve `fileId` through the existing item file tree.

### Explorer

- Keep PM-008 path search available under a Paths tab.
- Add a Content tab using the same toolbar region.
- Sources mode requests `mode=sources`.
- All Files mode requests `mode=all`.
- Search scope continues to support Current and All workspaces.
- Result selection uses existing ancestor expansion and route selection.

## Result Interaction

- Arrow Up and Arrow Down move active result.
- Enter opens the active result.
- Escape clears results and returns focus to the file tree.
- Result rows show path, line number, snippet, workspace, and item context.
- Highlight the matched substring in the snippet with `<mark>`.
- After opening, show `Line N, columns X-Y` in a stable context strip.
- Raw/source views scroll to the selected line when supported.
- Rendered preview keeps the context strip when direct line mapping is unavailable.

## State And Persistence

| State                    | Owner                   | Persistence                      |
|--------------------------|-------------------------|----------------------------------|
| Explorer tree mode       | Explorer hook           | Route query, then local storage  |
| Search surface tab       | Explorer toolbar        | Session only                     |
| Search query and results | `useContentSearch`      | Component lifetime               |
| Search workspace scope   | Explorer content search | Session only                     |
| Ignored preference       | Existing Explorer hook  | Existing local storage           |
| Selected match context   | File workspace          | Until selection or query changes |

## Responsive And Accessibility

- Keep a stable result height during loading and empty states.
- On mobile, results use the full tree pane width.
- Use `role=search`, `listbox`, `option`, and live result counts.
- Do not rely on highlight color alone; include line and column text.
- Preserve visible focus and roving keyboard behavior.
