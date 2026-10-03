---
slug: workspace-workstream-explorer-reference
title: Workstream, Explorer, And Search Reference
pageType: REFERENCE
topics: workstream routes, search scopes, saved filters, recent items, content rendering, explorer
summary: Reference for the unified Workstream and embedded Explorer routes, search boundaries, navigation state, card moves, and content rendering.
sourceRef: |
  PM-005 (plan retired) | Search and navigation contracts
  PM-006 (plan retired) | Viewer presentation contract
  PM-007 (plan retired) | Explorer interaction contract
  PM-008 (plan retired) | Explorer search and mutation contract
  PM-009 (plan retired) | Content-search scopes
  PM-010 (plan retired) | Branch selector behavior
  PM-012 (plan retired) | Drag-and-drop behavior
  PM-028 (plan retired) | Surface unification
lastVerified: 2026-09-12
---

# Workstream, Explorer, And Search Reference

## Surface Ownership
<!-- chunkId: workspace-workstream-explorer-reference-surfaces -->
<!-- keywords: route, workstream, item workspace, explorer -->

[live] PM-028 the router exposes Workstream and Item Workspace, while `WorkstreamExplorer` is a component used inside Item Workspace rather than a route. Evidence: `web/src/app/router.ts:1`, `web/src/pages/ItemWorkspacePage.tsx:1159`.

| Concern                                                  | Owning surface               |
|----------------------------------------------------------|------------------------------|
| Browse and filter all structured items                   | Workstream                   |
| Inspect one item's metadata, files, diff, and automation | Item Workspace               |
| Browse repository paths around the item                  | Embedded Workstream Explorer |
| Review a non-checkout branch                             | Branch Review                |

## Search And Navigation Contracts
<!-- chunkId: workspace-workstream-explorer-reference-search -->
<!-- keywords: search endpoint, saved filter, recent item, operational query -->

[live] PM-005 and PM-009 the current API provides global item search, workspace content search, item content search, saved-filter storage, and recent-item storage. Evidence: `internal/server/api/gin_routes.go:92`, `internal/server/api/gin_routes.go:95`, `internal/server/api/gin_routes.go:125`, `internal/server/api/gin_routes.go:141`.

Global search excludes reviewed snapshot rows and can filter by workspace. Explorer content search resolves configured
sources separately from all-files mode and shares a bounded result budget across workspaces. Saved filters persist a
valid application route and filter state; recent items are owner-scoped navigation history.

## Card Move Contract
<!-- chunkId: workspace-workstream-explorer-reference-card-move -->
<!-- keywords: drag card, workflow status, optimistic update, rollback -->

[live] PM-012 Workstream calculates a status transition through the shared drag-and-drop model and persists it through the same item status operation used by menus. Evidence: `web/src/features/workstream/dragAndDrop.ts:13`, `web/src/pages/WorkstreamPage.tsx:380`, `web/src/pages/WorkstreamPage.tsx:1504`.

Only supported destination statuses accept a drop. The UI may show the move immediately, but server failure restores
authoritative state and exposes the error rather than leaving a visual-only status.

## Explorer And Viewer Boundaries
<!-- chunkId: workspace-workstream-explorer-reference-content -->
<!-- keywords: sources mode, all mode, content preview, sandbox, size limit -->

[live] PM-006, PM-007, and PM-008 Explorer maintains separate caches for configured-source and all-files trees, and the shared viewer supports a source fallback plus sandboxed HTML. Evidence: `web/src/features/workstream-explorer/tree.test.ts:31`, `web/src/features/content-viewer/ContentViewer.tsx:15`, `web/src/features/content-viewer/ContentViewer.test.tsx:68`.

Branch selection shown on operational surfaces changes the actual checkout through the guarded checkout picker. It does
not make one Explorer component an independent snapshot browser; non-checkout inspection belongs to Branch Review.

## Not supported
<!-- chunkId: workspace-workstream-explorer-reference-not-supported -->
<!-- keywords: snapshot search, arbitrary route, executable html -->

[live] PM-006 and PM-009 do not execute repository HTML in the application origin, and operational global search does not return reviewed snapshots. Evidence: `web/src/features/content-viewer/ContentViewer.test.tsx:68`, `internal/search/search_service_test.go:63`.

## Known conflicts
<!-- chunkId: workspace-workstream-explorer-reference-known-conflicts -->
<!-- keywords: removed route, old selector, historical component -->

[legacy] PM-010's Explorer-local branch selector component still exists for compatibility inside the embedded explorer, but checkout ownership is now expressed consistently by `BranchCheckoutPicker` across Workstream, Canvas, Knowledge, Review, and Item Workspace.

## Related pages

See [[workspace-workstream-explorer]], [[workspace-branch-context-reference]], and [[workspace-lifecycle-reference]].
