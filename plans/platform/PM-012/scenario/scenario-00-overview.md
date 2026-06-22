# Scenarios: PM-012 Overview

## Scenario List

| #   | Title                    | Description                                                       |
|-----|--------------------------|-------------------------------------------------------------------|
| 0   | Existing status controls | Cards move through the status menu before drag support is added   |
| 1   | Drag to another column   | An editable card moves and its new status persists                |
| 2   | Cancel or invalid drop   | A drop outside a valid destination leaves the card unchanged      |
| 3   | Status update fails      | An optimistic move rolls back and exposes the persistence error   |
| 4   | Protected card           | Unsorted and freestyle docs cards cannot start a status drag      |
| 5   | Filtered board move      | Active filters immediately determine whether the moved card stays |
| 6   | Non-drag interaction     | Clicking, opening, and keyboard status selection continue to work |

---

## Scenario 0: Existing Status Controls

### Starting State

- The active workspace has indexed items in one or more status columns.
- Structured cards expose `StatusMenu`; freestyle docs and `Unsorted` cards do not.
- Selecting a status calls `PATCH /api/items/{id}/status` and reloads board items.
- Cards open the preview drawer on click and the item workspace from the title link.

### Available Actions

| Action           | Result                                              |
|------------------|-----------------------------------------------------|
| Click card       | Open the preview drawer                             |
| Click card title | Open the full item workspace                        |
| Choose status    | Persist a structured item's new status              |
| Filter board     | Show matching cards grouped by their current status |

---

## Scenario 1: Drag To Another Column

### Goal

Move an editable item to another workflow status with direct manipulation.

### Flow

```text
User presses an editable card
  -> pointer or touch activation constraint is met
  -> card becomes the active drag item
  -> editable columns become visible drop targets
  -> user releases over a different status column
  -> board changes the card's local status
  -> board marks the card pending
  -> PATCH /api/items/{id}/status
  -> backend updates plan.yaml and rescans the workspace
  -> board reconciles the returned item
  -> shared reliability/app state is notified
  -> pending state clears
```

### Expected State

- The card appears in the destination column immediately after drop.
- The destination count and source count update from local state.
- The card cannot be dragged again until its request completes.
- A later board refresh keeps the card in the persisted destination.

---

## Scenario 2: Cancel Or Invalid Drop

### Goal

Avoid writes when a drag does not identify a valid status change.

### Cases

| Drop Location              | Result                          |
|----------------------------|---------------------------------|
| Original status column     | No API call and no state change |
| Outside the Kanban columns | No API call and no state change |
| `Unsorted` column          | No API call and no state change |
| Separator or column button | No API call and no state change |
| Drag cancelled with Escape | No API call and no state change |

In every case, active drag styling and announcements clear.

---

## Scenario 3: Status Update Fails

### Goal

Keep the board consistent with persisted data when the write is rejected or unavailable.

### Flow

```text
Valid drop
  -> board saves the previous status
  -> board applies the destination status locally
  -> status API fails
  -> board restores the previous status
  -> pending state clears
  -> board shows the API error
  -> screen reader announces that the move failed
```

The card remains available for a retry through drag or `StatusMenu`.

---

## Scenario 4: Protected Card

### Goal

Prevent interactions that the existing metadata writer cannot persist.

### Cases

- A card with status `unsorted` has no drag activator.
- A card with `metadataSource === 'docs'` has no drag activator.
- The `Unsorted` column does not accept other cards.
- Hovering or pressing a protected card retains normal preview and title-link behavior.

---

## Scenario 5: Filtered Board Move

### Goal

Keep drag behavior consistent with the board's existing client-side filters.

### Cases

| Active Filter                         | Result After Moving Draft To Review                         |
|---------------------------------------|-------------------------------------------------------------|
| No status filter                      | Card appears in Review                                      |
| Draft only                            | Card disappears because it no longer matches                |
| Draft and Review                      | Card appears in Review                                      |
| Search excludes destination card data | Existing search result remains governed by item text fields |

Column counts always reflect the filtered item collection.

---

## Scenario 6: Non-Drag Interaction

### Goal

Preserve every existing way to inspect or move a card.

### Cases

- A press and release below the drag threshold opens the preview drawer once.
- Clicking the title opens the item workspace and does not start a drag.
- Opening or selecting `StatusMenu` does not start a drag.
- Keyboard users move status through `StatusMenu` and receive the same optimistic, pending, success, and rollback behavior.
- Touch users can scroll the board; a drag starts only after the configured touch activation delay and movement tolerance.
