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
<!-- keywords: route, item workspace, embedded component -->

[live] The router exposes Workstream and Item Workspace. `WorkstreamExplorer` is a component inside Item Workspace, not a route. Evidence: `web/src/app/router.ts:9`, `web/src/pages/ItemWorkspacePage.tsx:594`.

| Concern                                                  | Owning surface               |
|----------------------------------------------------------|------------------------------|
| Browse and filter all structured items                   | Workstream                   |
| Inspect one item's metadata, files, diff, and automation | Item Workspace               |
| Browse repository paths around the item                  | Embedded Workstream Explorer |
| Review a non-checkout branch                             | Branch Review                |

## Search And Navigation Contracts
<!-- chunkId: workspace-workstream-explorer-reference-search -->
<!-- keywords: search endpoint, saved filter, recent item, result budget -->

[live] The API provides global item search, workspace and item content search, saved filters and recent items. Evidence: `internal/server/api/gin_routes.go:118`, `internal/server/api/gin_routes.go:148`.

- Global search excludes reviewed snapshot rows and can filter by workspace.
- Explorer content search resolves configured sources apart from all-files mode and shares one bounded result budget across workspaces.
- A saved filter stores a valid route and filter state. Recent items are per-owner navigation history.

## Card Move Contract
<!-- chunkId: workspace-workstream-explorer-reference-card-move -->
<!-- keywords: drag card, optimistic update, rollback -->

[live] A drag computes the status change through the shared drag-and-drop model and saves it through the same status operation as the menus. Evidence: `web/src/features/workstream/dragAndDrop.ts:13`.

Only supported statuses accept a drop. The UI may move the card at once, but a server failure restores the real state and shows the error.

## Explorer And Viewer Boundaries
<!-- chunkId: workspace-workstream-explorer-reference-content -->
<!-- keywords: tree cache, sandbox, checkout picker -->

[live] Explorer keeps separate caches for the sources tree and the all-files tree. The viewer has a source fallback and sandboxed HTML. Evidence: `web/src/features/workstream-explorer/tree.test.ts:31`, `web/src/features/content-viewer/ContentViewer.test.tsx:68`.

The branch picker on operational surfaces is `BranchCheckoutPicker`, and it switches the real checkout. Branch Review handles non-checkout inspection.

## Not supported
<!-- chunkId: workspace-workstream-explorer-reference-not-supported -->
<!-- keywords: snapshot search, executable html, app origin -->

[live] Running repository HTML in the app origin, and reviewed snapshots in global search. Evidence: `internal/search/search_service_test.go:61`.

## Known conflicts
<!-- chunkId: workspace-workstream-explorer-reference-known-conflicts -->
<!-- keywords: explorer branch selector, compatibility component, checkout picker -->

- "Explorer has its own branch selector." An older selector component still exists inside the embedded explorer, but `BranchCheckoutPicker` owns the checkout on every operational surface.

## Related pages

See [[workspace-workstream-explorer]] and [[workspace-branch-context-reference]].
