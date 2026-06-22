# Implementation Plan: PM-012 - Kanban Card Drag And Drop

## Overview

Add cross-column drag-and-drop to editable Kanban cards. Reuse the existing status endpoint, apply optimistic moves with targeted rollback, retain `StatusMenu` as the accessible fallback, and verify pointer, touch, filtered, error, and responsive states.

## Terminology Lock

- Use `item`, not `plan`, for new public names and drag payload fields.
- Use `status`, not `lane`, for persisted workflow values.
- Use `column` for the visual Kanban destination.
- Use `drag` and `drop`; do not introduce persisted `rank`, `position`, or `order` fields.

## Phases Summary

| Phase | Name                             | Status |
|-------|----------------------------------|--------|
| F1    | Drag Model And State Transitions | ✅     |
| F2    | Board Drag Integration           | ✅     |
| F3    | Visual States And Verification   | ✅     |

## Frontend Phases

### Phase F1: Drag Model And State Transitions

**Deliverables:**

- [x] Add `@dnd-kit/core` to `package.json` and the lockfile.
- [x] Add pure drag eligibility, drop-status, and immutable status-update helpers under `web/src/features/kanban/`.
- [x] Refactor the status move path into one optimistic mutation shared by drag and `StatusMenu`.
- [x] Track pending item IDs and reject duplicate or unchanged moves.
- [x] Reconcile successful moves from `WriteResult.item` without a mandatory full item reload.
- [x] Roll back only the failed item's previous status and expose the API error.
- [x] Add focused helper and page tests for optimistic success, invalid moves, duplicate prevention, and rollback.

**Result:** Added `dragAndDrop.ts`, installed `@dnd-kit/core`, and expanded Kanban coverage to 10 focused helper/page tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/features/kanban web/src/pages/KanbanPage.test.tsx`

**Draft Commit:**

```text
PM-012: Add optimistic Kanban status transitions

Change summary:
- Add drag eligibility and status transition helpers
- Share optimistic persistence across status controls
- Cover success, no-op, pending, and rollback behavior
```

---

### Phase F2: Board Drag Integration

**Deliverables:**

- [x] Wrap the board in `DndContext` with pointer and touch activation constraints.
- [x] Make editable status columns droppable and keep `Unsorted` invalid.
- [x] Make eligible, non-pending `PlanCard` instances draggable.
- [x] Add a non-interactive drag overlay using the active card's current content.
- [x] Route valid drag completion through the shared optimistic status function.
- [x] Treat same-column, outside, invalid, and cancelled drops as no-ops.
- [x] Preserve preview clicks, title navigation, status-menu controls, Escape cancellation, and touch scrolling.
- [x] Add drag announcements and pending-card accessibility state.
- [x] Add deterministic component tests for valid drops, invalid drops, protected cards, cancellation, and event boundaries.

**Result:** Added pointer/touch drag sensors, protected droppable columns, card overlays, accessible announcements, post-drag click suppression, and 5 deterministic drag integration tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/KanbanPage.test.tsx`

**Draft Commit:**

```text
PM-012: Add Kanban card drag and drop

Change summary:
- Add draggable cards and editable column targets
- Persist valid cross-column drops through the status API
- Preserve card controls and protected item behavior
```

---

### Phase F3: Visual States And Verification

**Deliverables:**

- [x] Style active cards, drag overlay, valid columns, current target, and pending cards in `web/src/styles/app.css`.
- [x] Add reduced-motion behavior for drag transitions and overlay effects.
- [x] Verify status counts and card visibility with no filters, one status filter, and multiple status filters.
- [x] Verify desktop horizontal columns and compact stacked columns through the existing responsive rules and production build.
- [x] Verify pointer/touch configuration, card click suppression, cancellation, protected cards, and status-menu fallback through focused tests.
- [x] Run the full frontend and backend regression suites.
- [x] Rebuild production frontend assets in `internal/app/frontend`.
- [x] Update planning documents with final file names and implementation results.

**Result:** Added drag/drop/pending/reduced-motion styles, verified 91 frontend tests and 154 backend tests, built production assets, and confirmed the rebuilt server returns HTTP 200. Live browser review was attempted but the in-app browser was unavailable in this session.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**

```text
PM-012: Finish Kanban drag interaction states

Change summary:
- Style drag targets, overlays, and pending cards
- Verify responsive, filtered, pointer, and touch behavior
- Rebuild embedded frontend assets
```

---

## Post-Implementation Checklist

- [x] Confirm no backend endpoint or metadata schema change was introduced.
- [x] Confirm no rank, position, or within-column ordering state was added.
- [x] Confirm `Unsorted` and plain docs cards remain protected.
- [x] Confirm `StatusMenu` remains available and keyboard operable.
- [x] Confirm failed moves restore the prior status and remain retryable.
- [x] Confirm generated frontend assets are committed with source changes.
- [x] Update `plans/platform/PM-012/` with final implementation references.

## Testing Strategy

- Unit-test pure eligibility and immutable status-transition helpers.
- Component-test drag outcomes through deterministic events rather than browser geometry alone.
- Regression-test existing card preview, title navigation, status menu, filters, and error display.
- Browser-test real pointer and touch activation, drop feedback, cancellation, and responsive layouts.
- Run backend tests because every move still exercises the existing status writer and workspace rescan contract.

## Migration Notes

- No stored data migration is required.
- No API migration is required.
- The only new runtime dependency is `@dnd-kit/core`.
