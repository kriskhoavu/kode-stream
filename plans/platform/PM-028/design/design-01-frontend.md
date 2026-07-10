# Frontend Design: PM-028

## Objective

Align Workstream and Explorer behavior where they overlap, especially branch context, item file browsing, and surface terminology. Reduce duplicate UI logic first, then remove the standalone Explorer page only after all remaining route-only workflows move elsewhere.

## Current Structure

| Area                 | Current Owner            | Notes                                                                     |
|----------------------|--------------------------|---------------------------------------------------------------------------|
| Board route          | `WorkstreamPage`         | Canonical planning surface                                                |
| Item detail files    | `ItemWorkspacePage`      | Hosts `WorkstreamExplorer` in embedded mode for working-tree items        |
| Standalone explorer  | `WorkstreamExplorer`     | Still used by `/explorer` and by Knowledge deep-links                     |
| Shared branch UI     | `BranchSnapshotPicker`   | Now reused by Workstream and item detail                                  |

## Shipped Changes

| Change Area                  | Implementation                                                             |
|-----------------------------|----------------------------------------------------------------------------|
| Shared branch UI            | Replace duplicate Workstream and item-detail dropdowns with one component |
| Snapshot semantics          | Item details use branch snapshot loading instead of direct Git checkout    |
| Missing item branch state   | Keep selected branch visible and show an explicit empty snapshot state     |
| Dropdown behavior           | Add search, keyboard navigation, and pinned current checkout ordering      |
| Visual alignment            | Reuse shared chip layout and branch menu styling across both surfaces      |

## Remaining Cleanup

| Area                     | Current State                                                              | Cleanup Direction                                                         |
|--------------------------|----------------------------------------------------------------------------|---------------------------------------------------------------------------|
| Standalone Explorer route| Still required for Knowledge and arbitrary file browsing                  | Replace route dependency before removal                                   |
| `WorkstreamExplorer` name| A page-named component is also used as an embedded component              | Move to a feature-level component after route removal                     |
| `WorkspaceBranchSelector`| Explorer still uses its own select-based branch control                   | Decide whether Explorer adopts the shared picker or is removed entirely   |
| Router explorer helpers  | `Route.name === 'explorer'`, parser helpers, tests, and `App` route wiring remain | Remove together when the route disappears                                 |

## Route Removal Constraint

Deleting the standalone Explorer page now would break:

- Knowledge "Open in Explorer" for arbitrary files.
- Non-item workspace file browsing.
- Router parsing and navigation for existing `/explorer` URLs.

The safe order is:

1. Move arbitrary workspace file browsing into Workstream or another canonical surface.
2. Retarget Knowledge deep-links.
3. Remove the standalone Explorer route, page-level wiring, and route-only tests.
