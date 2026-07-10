# Implementation Plan: PM-028 - Unify Workstream And Explorer Surfaces

## Overview

PM-028 merges overlapping Workstream and Explorer behaviors by consolidating branch controls, aligning snapshot semantics, and embedding the Explorer shell into item details. The latest commit already delivered the shared branch picker and item-detail snapshot fixes. The remaining cleanup is the actual removal of the standalone Explorer route after its surviving workflows move elsewhere.

## Terminology Lock

Use these names consistently:

- `Workstream` for the canonical planning surface.
- `Embedded Explorer` for the file browser inside item details.
- `Standalone Explorer` for the `/explorer` route that still exists today.
- `Current Checkout Branch` for the real working-tree branch.
- `Snapshot Branch` for read-only branch loads.

## Phases Summary

| Phase | Name                                   | Status      |
|-------|----------------------------------------|-------------|
| F1    | Shared Branch Snapshot Picker          | Done        |
| F2    | Item Detail Snapshot Alignment         | Done        |
| F3    | Embedded Explorer Visual Convergence   | Done        |
| F4    | Standalone Explorer Route Removal Audit| In Progress |
| F5    | PM-028 Documentation                   | Done        |

## Frontend Phases

### Phase F1: Shared Branch Snapshot Picker

**Deliverables:**

- [x] Replace duplicate Workstream and item-detail branch dropdown implementations.
- [x] Add shared search, snapshot label, checkout icon, and width behavior.
- [x] Add keyboard navigation and Enter selection support.
- [x] Pin `main`, `master`, and the current checkout branch at the top of the list.

**Verification:** `rtk npm run typecheck && rtk npm test -- WorkstreamPage.test.tsx ItemWorkspacePage.test.ts WorkstreamExplorer.test.tsx useWorkspaceBranches.test.tsx`

**Commit:** `PM-028: Unify Workstream page and Explorer page`

---

### Phase F2: Item Detail Snapshot Alignment

**Deliverables:**

- [x] Load item-detail branch changes through Workstream snapshot APIs.
- [x] Preserve selected branch state when the target branch does not contain the item.
- [x] Show snapshot-empty messaging instead of silently falling back to current checkout content.
- [x] Keep item edits guarded by snapshot materialization rules.

**Verification:** `rtk npm run typecheck && rtk npm test -- ItemWorkspacePage.test.ts ItemWorkspacePage.search.test.tsx`

**Commit:** `PM-028: Unify Workstream page and Explorer page`

---

### Phase F3: Embedded Explorer Visual Convergence

**Deliverables:**

- [x] Align left-panel sizing and collapse behavior with the item-detail shell.
- [x] Reuse branch picker styling between Workstream and embedded Explorer-related flows where applicable.
- [x] Remove duplicate details-only branch styles after the shared picker replacement.

**Verification:** `rtk npm run typecheck && rtk npm test -- WorkstreamPage.test.tsx ItemWorkspacePage.test.ts WorkstreamExplorer.test.tsx`

**Commit:** `PM-028: Unify Workstream page and Explorer page`

---

### Phase F4: Standalone Explorer Route Removal Audit

**Deliverables:**

- [ ] Identify every remaining consumer of the standalone `/explorer` route.
- [ ] Rehome Knowledge arbitrary-file deep-links to the canonical replacement surface.
- [ ] Move non-item workspace file browsing into Workstream or another retained route.
- [ ] Remove `Route.name === 'explorer'`, standalone route rendering in `App`, and route-only tests.
- [ ] Remove route-only Explorer props and route plumbing after the above is complete.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-028: Remove standalone Explorer route`

---

### Phase F5: PM-028 Documentation

**Deliverables:**

- [x] Add PM-028 plan documents after the implementation commit.
- [x] Record the shipped merge behavior and the route-removal blocker.
- [x] Link PM-028 to Explorer, Knowledge, snapshot, and Workstream foundation plans.

**Verification:** `rtk sed -n '1,200p' plans/platform/PM-028/README.md`

## Review Notes

- No backend dead code is removable yet from this merge alone. Workspace file/tree/search/Git APIs still back both the embedded Explorer and the standalone route used by Knowledge.
- The main remaining frontend cleanup is route-level, not component-level. The standalone Explorer cannot be deleted safely until arbitrary file browsing is rehomed.
