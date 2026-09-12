---
slug: workspace-workstream-explorer
title: Workstream, Explorer, And Search
pageType: CONCEPT
topics: workstream, explorer, search, navigation, content viewer, drag and drop
summary: Workstream is the primary planning surface, with repository exploration, scoped search, preview, and editing embedded in an item's workspace.
sourceRef: |
  plans/platform/PM-005/README.md
  plans/platform/PM-006/README.md
  plans/platform/PM-007/README.md
  plans/platform/PM-008/README.md
  plans/platform/PM-009/README.md
  plans/platform/PM-010/README.md
  plans/platform/PM-011/README.md
  plans/platform/PM-012/README.md
  plans/platform/PM-028/README.md
lastVerified: 2026-09-12
---

# Workstream, Explorer, And Search

## One Planning Surface
<!-- chunkId: workspace-workstream-explorer-one-surface -->
<!-- keywords: workstream, board, status column, item workspace -->

[live] PM-011, PM-012, and PM-028 Workstream is the primary collection view, and a structured item opens into Item Workspace instead of a separate top-level Explorer product. Evidence: `web/src/app/router.ts:1`, `web/src/pages/WorkstreamPage.tsx:613`, `web/src/pages/ItemWorkspacePage.tsx:1158`.

Cards are grouped by workflow status and eligible cards can be dragged between columns. The same move remains available
through explicit status controls, so drag and drop is an interaction shortcut rather than a separate state model.

## Explorer Is Context, Not A Destination
<!-- chunkId: workspace-workstream-explorer-context -->
<!-- keywords: embedded explorer, item workspace, sources, all files -->

[live] PM-007, PM-008, PM-009, and PM-028 repository exploration is implemented by `WorkstreamExplorer` embedded in Item Workspace, with configured-source and all-files modes. Evidence: `web/src/pages/ItemWorkspacePage.tsx:1158`, `web/src/pages/WorkstreamExplorer.tsx:44`, `web/src/features/workstream-explorer/useWorkstreamExplorer.ts:136`.

The tree supplies repository context beside item metadata and content. It supports lazy directory loading, Git state,
path search, scoped content search, preview, and guarded editing without changing the application-wide route context.

## Search Has Three Scopes
<!-- chunkId: workspace-workstream-explorer-search-scopes -->
<!-- keywords: global search, quick switcher, path search, content search -->

[live] PM-005 and PM-009 search separates global item navigation, repository path search, and textual content search. Evidence: `internal/search/search_service.go:34`, `internal/search/content_search_service.go:47`, `internal/search/content_search_service.go:84`.

Global search powers the quick switcher and ranks structured items. Explorer path search finds repository entries.
Content search can be restricted to one item, configured sources, one workspace, or all registered workspaces according
to the calling surface.

## Shared Preview
<!-- chunkId: workspace-workstream-explorer-preview -->
<!-- keywords: markdown, json, image, html sandbox, source fallback -->

[live] PM-006 one content viewer is reused by Workstream, Item Workspace, Explorer, Knowledge, Branch Review, and verification evidence. Evidence: `web/src/features/content-viewer/ContentViewer.tsx:15`, `web/src/pages/WorkstreamPage.tsx:1916`, `web/src/features/knowledge/KnowledgeReader.tsx:24`.

It selects a safe presentation for Markdown, structured text, images, HTML, and plain source. Large or unsupported content
falls back to source-oriented presentation rather than executing repository content in the application context.

## Not supported
<!-- chunkId: workspace-workstream-explorer-not-supported -->
<!-- keywords: standalone explorer, snapshot mutation, global branch selector -->

[live] PM-010 and PM-028 do not provide a standalone `/explorer` route or a global branch snapshot that changes operational reads. Evidence: `web/src/app/router.ts:1`, `internal/search/search_service_test.go:63`.

## Known conflicts
<!-- chunkId: workspace-workstream-explorer-known-conflicts -->
<!-- keywords: kanban, explorer route, branches route, old navigation -->

[legacy] PM-005 through PM-012 use the former Kanban, Explorer, Items, and Branches navigation model. PM-028 removed the standalone Explorer surface, and PM-038 later replaced cross-branch snapshots with checkout-first operation and Branch Review.

## Related pages

See [[workspace-workstream-explorer-reference]], [[workspace-lifecycle]], and [[workspace-branch-context]].
