---
slug: workspace-workstream-explorer
title: Workstream, Explorer, And Search
pageType: CONCEPT
topics: workstream, explorer, search, navigation, content viewer, drag and drop
summary: Workstream is the primary planning surface, with repository exploration, scoped search, preview, and editing embedded in an item's workspace.
sourceRef: |
  PM-005 (plan retired) | Search and navigation
  PM-006 (plan retired) | Shared content viewer
  PM-007 (plan retired) | Workspace Explorer
  PM-008 (plan retired) | Explorer productivity
  PM-009 (plan retired) | Scoped content search
  PM-010 (plan retired) | Explorer branch selection
  PM-011 (plan retired) | Primary navigation consolidation
  PM-012 (plan retired) | Workstream card movement
  PM-028 (plan retired) | Unified Workstream and Explorer
lastVerified: 2026-09-12
---

# Workstream, Explorer, And Search

## One Planning Surface
<!-- chunkId: workspace-workstream-explorer-one-surface -->
<!-- keywords: board, status column, item workspace, drag -->

[live] Workstream is the main collection view. A structured item opens into Item Workspace. Evidence: `web/src/app/router.ts:9`, `web/src/pages/ItemWorkspacePage.tsx:594`.

Cards group by workflow status. Dragging a card between columns does the same as the explicit status control.

## Explorer Is Context, Not A Destination
<!-- chunkId: workspace-workstream-explorer-context -->
<!-- keywords: embedded explorer, sources mode, all files, lazy tree -->

[live] The repository explorer is embedded in Item Workspace, with a configured-sources mode and an all-files mode. Evidence: `web/src/pages/WorkstreamExplorer.tsx:29`.

The tree loads directories lazily and offers Git state, path search, scoped content search, preview and guarded editing. It never changes the app's route context.

## Search Has Three Scopes
<!-- chunkId: workspace-workstream-explorer-search-scopes -->
<!-- keywords: quick switcher, path search, content search, ranking -->

[live] Search separates item navigation, path search and content search. Evidence: `internal/search/search_service.go:34`, `internal/search/content_search_service.go:47`.

- Global search drives the quick switcher and ranks structured items.
- Path search finds repository entries.
- Content search covers one item, configured sources, one workspace or all workspaces, as the calling surface decides.

## Shared Preview
<!-- chunkId: workspace-workstream-explorer-preview -->
<!-- keywords: markdown, image, html sandbox, source fallback -->

[live] One content viewer serves Workstream, Item Workspace, Explorer, Knowledge, Branch Review and verification evidence. Evidence: `web/src/features/content-viewer/ContentViewer.tsx:15`.

It picks a safe view for Markdown, structured text, images, HTML and source. Large or unsupported content falls back to source. Repository content never runs in the app.

## Not supported
<!-- chunkId: workspace-workstream-explorer-not-supported -->
<!-- keywords: standalone explorer, snapshot mutation, global branch selector -->

[live] A standalone `/explorer` route, and a global branch snapshot that changes operational reads. Evidence: `web/src/app/router.ts:9`, `internal/search/search_service_test.go:61`.

## Known conflicts
<!-- chunkId: workspace-workstream-explorer-known-conflicts -->
<!-- keywords: kanban, explorer route, branches route -->

- "Kode Stream has Kanban, Explorer, Items and Branches pages." The pages are Workstream, Knowledge, Agent Space, Workspaces and Settings. Explorer lives inside Item Workspace. Other branches open in Branch Review.

## Related pages

See [[workspace-workstream-explorer-reference]] and [[workspace-branch-context]].
