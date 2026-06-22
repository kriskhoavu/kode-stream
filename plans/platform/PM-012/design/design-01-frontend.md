# Frontend Design: PM-012

## Overview

PM-012 adds a drag interaction layer around the existing `KanbanPage` board. It reuses `ItemSummary.status`, `api.updateStatus`, `statusOrder`, `editableStatusOrder`, and the current card/status-menu UI. No backend or API contract changes are required.

## Dependency

Add `@dnd-kit/core` to the frontend dependencies. Use only the core package because this feature needs draggable cards and droppable columns, not sortable rank management.

## Interaction Model

| Interaction State | Responsibility                                                       |
|-------------------|----------------------------------------------------------------------|
| Idle              | Cards retain existing click, title-link, and status-menu behavior    |
| Activating        | Pointer distance or touch delay distinguishes dragging from clicking |
| Dragging          | Track the item ID and render a non-interactive drag overlay          |
| Over target       | Highlight one valid editable status column                           |
| Pending           | Show the optimistic card location and block another move for that ID |
| Failed            | Restore the previous status and show the existing board error        |

## Eligibility And Validation

Add small pure helpers under `web/src/features/kanban/dragAndDrop.ts`:

```typescript
isItemDraggable(item: ItemSummary): boolean
isDropStatus(status: ItemStatus): boolean
applyItemStatus(items: ItemSummary[], itemId: string, status: ItemStatus): ItemSummary[]
```

Eligibility matches the existing status control: the item must not be `unsorted`, must not use plain `docs` metadata, and must not already have a pending move. Drop statuses use `editableStatusOrder`, which excludes `unsorted`.

## Component Structure

```text
KanbanPage
  -> DndContext
     -> KanbanColumn (one droppable per editable status)
        -> PlanCard (draggable only when eligible)
     -> DragOverlay
        -> PlanCardPreview (non-interactive visual copy)
```

`KanbanColumn` and `PlanCardPreview` may remain in `KanbanPage.tsx` while small. Move them into `web/src/features/kanban/` only if the page becomes harder to read or test.

## Drag Data

| Field    | Type         | Purpose                                      |
|----------|--------------|----------------------------------------------|
| `itemId` | `string`     | Stable source item identity                  |
| `status` | `ItemStatus` | Source status captured at drag start         |
| `target` | `ItemStatus` | Destination status supplied by the drop zone |

Do not place a full mutable `ItemSummary` in the drag payload. Resolve the current item from page state at drag end so refreshes and pending-state checks use current data.

## Sensors And Event Boundaries

- Configure a pointer sensor with a small movement-distance activation constraint.
- Configure a touch sensor with a short hold delay and movement tolerance so vertical scrolling remains usable.
- Cancel with Escape through the library's standard cancellation behavior.
- Attach draggable listeners to the card surface while excluding child interactive elements from drag activation.
- Suppress the click generated immediately after a completed drag so the preview drawer does not open.
- Keep the existing `StatusMenu` as the keyboard path; do not create a second keyboard-only drag model.

## State And Persistence

`KanbanPage` owns:

| State            | Type            | Purpose                                      |
|------------------|-----------------|----------------------------------------------|
| `activeItemId`   | `string`        | Card currently represented by the overlay    |
| `pendingItemIds` | `Set<string>`   | Cards with status requests in flight         |
| `items`          | `ItemSummary[]` | Existing board source plus optimistic status |

Use one `movePlan(itemId, destination)` function for drag and `StatusMenu`:

1. Resolve the item from current state.
2. Return for an invalid, unchanged, or pending move.
3. Save the previous status and apply the destination status locally.
4. Add the item ID to `pendingItemIds`.
5. Call `api.updateStatus(itemId, { status: destination })`.
6. On success, replace the optimistic item with `WriteResult.item` and notify reliability/shared app state.
7. On failure, restore the previous status and set the existing board error.
8. Clear the pending item ID in `finally`.

The rollback must update only the matching item and previous status. It must not replace the full list with a stale snapshot.

## Filter Behavior

`grouped` remains derived from `items` through `filterPlans`. Optimistic state therefore updates column membership and counts without a separate drag-specific board model. If the destination does not match active status filters, the card disappears from the filtered result after drop; this is expected.

## Visual States

| Selector/State         | Treatment                                                         |
|------------------------|-------------------------------------------------------------------|
| Draggable card         | Preserve pointer cursor; show a subtle grab affordance            |
| Active source card     | Lower opacity without collapsing its layout                       |
| Drag overlay           | Elevated shadow, slight scale, no menus or links                  |
| Valid drop column      | Strengthen border and background when any eligible drag is active |
| Current drop target    | Use the column status accent and an inset highlight               |
| Pending card           | Reduce interaction and show progress without layout movement      |
| Invalid/protected card | Keep existing card appearance and interactions                    |

Respect `prefers-reduced-motion` by removing drag transition and scale effects.

## Accessibility

- Keep `StatusMenu` visible and operable for keyboard users.
- Give the drag context announcements using item title and status labels.
- Announce drag start, valid target changes, successful moves, cancellations, and rollback failures.
- Mark pending cards with `aria-busy="true"`.
- Do not remove the card's current `role="button"`, focusability, Enter, or Space preview behavior.
- Keep focus stable after menu-based status moves.

## Testing

| Test Area          | Coverage                                                                       |
|--------------------|--------------------------------------------------------------------------------|
| Pure helpers       | Eligible cards, protected cards, editable targets, immutable status updates    |
| Drag integration   | Valid cross-column drop, same-column no-op, outside drop, protected cards      |
| Persistence        | Optimistic movement, one API request, success reconciliation, failure rollback |
| Event boundaries   | Card click, title click, status-menu interaction, post-drag click suppression  |
| Filters            | Destination visibility and counts under active status filters                  |
| Responsive/browser | Pointer drag, touch drag, horizontal desktop board, stacked compact board      |

Use mocked drag events or a test adapter for deterministic component tests. Do not make pointer geometry in jsdom the only source of coverage.

## Design Decisions

| Decision                               | Rationale                                                                 |
|----------------------------------------|---------------------------------------------------------------------------|
| Share one status mutation path         | Drag and menu moves must have identical persistence and rollback behavior |
| Derive columns from optimistic `items` | Existing filtering and grouping stay the source of truth                  |
| Keep drag payload minimal              | Current React state remains authoritative during refreshes                |
| Use an overlay                         | The card does not clip inside columns while crossing the wide board       |
| Test helpers separately from geometry  | jsdom does not model browser drag layout reliably                         |
