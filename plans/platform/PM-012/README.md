# PM-012: Kanban Card Drag And Drop

PM-012 lets users move editable item cards between Kanban status columns by dragging them. The board updates immediately, persists the new status through the existing status API, and restores the previous state when persistence fails. The existing status menu remains available for keyboard, touch, and precise status selection.

## Related Plans

| Ticket                        | Relationship   | Key Context                                                                 |
|-------------------------------|----------------|-----------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Board baseline | Added the workspace-scoped Kanban board, status columns, filters, and cards |
| [PM-002](../PM-002/README.md) | Parent feature | Added status persistence and specified drag-and-drop as an intended control |

## Goals

- Drag an editable card from its current column to another editable status column.
- Update the board optimistically while the existing status API persists the move.
- Restore the previous column and show an error when the status update fails.
- Make valid drop targets and pending cards visually clear.
- Preserve card preview, item navigation, filters, the status menu, and responsive behavior.
- Support pointer and touch dragging without turning a normal click into a drag.

## Out Of Scope

- Reordering cards within a status column.
- Persisting a card rank or custom board order.
- Moving cards into or out of `Unsorted`.
- Making freestyle docs cards editable.
- Multi-card dragging or bulk status updates.
- Changing `PATCH /api/items/{id}/status` or the item metadata schema.

## Glossary

| Term            | Meaning                                                        | Code Target                |
|-----------------|----------------------------------------------------------------|----------------------------|
| Draggable Card  | A structured item card whose status can be updated             | `PlanCard`                 |
| Drop Column     | An editable Kanban status column that accepts a draggable card | `KanbanColumn`             |
| Active Drag     | The card currently controlled by the drag interaction          | `activeDrag` state         |
| Optimistic Move | Immediate local status change before the API request finishes  | `movePlan`                 |
| Pending Move    | A status update request that has not completed                 | `pendingItemIds` state     |
| Rollback        | Restoring the card's previous status after a failed update     | status-move error handling |
| Status Menu     | Existing non-drag control for selecting an item's status       | `StatusMenu`               |

## Components

| Layer    | Component                | Purpose                                                                   |
|----------|--------------------------|---------------------------------------------------------------------------|
| Frontend | Drag interaction helpers | Define draggable eligibility, drop validation, and optimistic transitions |
| Frontend | `KanbanPage`             | Own active drag, pending moves, persistence, reconciliation, and errors   |
| Frontend | `KanbanColumn`           | Expose editable columns as drop targets and render target feedback        |
| Frontend | `PlanCard`               | Expose eligible cards as drag sources while preserving click interactions |
| Frontend | Drag overlay             | Keep a stable card preview above columns while the source card is moving  |

## Data Flow

```text
User presses and moves an editable card
  -> drag activation threshold is crossed
  -> board records the active card
  -> editable status columns show drop-target state
  -> user releases over a different editable status column
  -> board validates the source card and target status
  -> local item status changes immediately
  -> PATCH /api/items/{id}/status
     -> success: reconcile with returned item and notify shared app state
     -> failure: restore previous status and show the API error
  -> clear active and pending drag state
```

## Requirements

- Only items currently eligible for `StatusMenu` moves are draggable.
- `Unsorted` and freestyle docs cards are not draggable, and `Unsorted` is not a drop target.
- Dropping on the current status, outside a column, or on an invalid column is a no-op.
- A card with a pending status update cannot start another move.
- Drag activation uses movement or touch delay constraints so card clicks still open the preview drawer.
- Child controls such as the title link and status menu keep their existing click behavior.
- The drag overlay identifies the card being moved without duplicating interactive controls.
- Filtered boards move the card according to the active filter result. A card may disappear after a valid move when the destination status is excluded.
- The status menu remains the keyboard-accessible fallback and uses the same optimistic move path.
- Screen readers receive concise pickup, target, success, and failure announcements from the drag layer.

## Design Decisions

| Decision                                        | Alternatives Considered                | Rationale                                                                                   |
|-------------------------------------------------|----------------------------------------|---------------------------------------------------------------------------------------------|
| Use `@dnd-kit/core`                             | Native HTML drag events, custom events | It supports pointer and touch sensors, overlays, drop targets, and React-managed state      |
| Keep status as the only persisted value         | Add rank fields and ordering API       | The request is cross-column movement; current data has no rank and cards already have order |
| Apply optimistic local updates                  | Reload only after API success          | The dragged card should remain in the dropped column while persistence runs                 |
| Lock only the pending card                      | Disable the entire board               | Independent cards remain usable while duplicate requests for one card are prevented         |
| Retain `StatusMenu`                             | Replace all status controls with drag  | Drag is not sufficient for keyboard use, compact layouts, or precise selection              |
| Exclude `Unsorted` and plain docs from dragging | Let the backend reject invalid moves   | The current writer intentionally rejects metadata updates for freestyle docs                |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Frontend Design](design/design-01-frontend.md)
- [Implementation Plan](implementation-plan.md)

## Implementation Result

- Added `@dnd-kit/core` pointer and touch sensors to `KanbanPage`.
- Added draggable structured cards, editable status drop columns, and a non-interactive drag overlay.
- Kept `Unsorted` and freestyle docs cards protected from status moves.
- Shared optimistic persistence, pending protection, success reconciliation, and targeted rollback between drag and `StatusMenu`.
- Added accessible drag announcements, post-drag click suppression, reduced-motion styles, and responsive drop states.
- Added helper, optimistic mutation, filtered-state, and deterministic drag integration tests.
- Verified 91 frontend tests, 154 backend tests, production asset generation, and the rebuilt HTTP server.
- Live browser review remains unrecorded because the in-app browser was unavailable during final verification.
