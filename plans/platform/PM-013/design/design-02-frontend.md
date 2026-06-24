# Frontend Design: Kanban Branch Snapshot Materialization

## Overview

Kanban changes from a multi-branch filter to a single branch view. The selected branch controls which indexed content appears on the board. The current checkout branch controls where writes go. Snapshot edits require one confirmation before copying content into the checkout branch.

## Data Model

### Entity: KanbanBranchState

| Field                   | Type          | Purpose                              |
|-------------------------|---------------|--------------------------------------|
| `selectedBranch`        | string        | One branch shown by the board        |
| `currentCheckoutBranch` | string        | Branch currently checked out in Git  |
| `mode`                  | string        | `working_tree` or `snapshot`         |
| `editable`              | boolean       | Direct write availability            |
| `items`                 | ItemSummary[] | Branch-scoped board items            |
| `loading`               | boolean       | Branch load state                    |
| `refreshing`            | boolean       | Forced branch refresh state          |
| `error`                 | string        | Branch load or materialization error |

### Entity: MaterializationPrompt

| Field       | Type   | Purpose                                  |
|-------------|--------|------------------------------------------|
| `itemId`    | string | Snapshot item being edited               |
| `operation` | string | File, metadata, status, or create action |
| `message`   | string | User-facing copy explanation             |
| `confirmed` | bool   | Whether the current action can proceed   |

## State Management

| Store/State     | Responsibility                                                          |
|-----------------|-------------------------------------------------------------------------|
| `KanbanPage`    | Own selected branch, branch load result, board items, and selected item |
| API client      | Add branch load and materialization-aware write calls                   |
| Branch selector | Render one selected branch and mode labels                              |
| Preview drawer  | Route snapshot edits through the materialization prompt                 |
| New item dialog | Disable in snapshot mode unless later tied to explicit materialization  |

No browser-only cache should define the selected branch. Last selected branch belongs in backend workspace registry state so startup behavior is stable.

## Branch Selector

The Kanban branch selector is single-select.

Examples:

- `master (working tree)`
- `DI-445 (snapshot)`
- `feature/foo (snapshot)`

Behavior:

- Selecting a branch calls the branch load endpoint.
- It never calls `api.switchBranch`.
- It never invokes Explorer branch switching state.
- It resets branch-specific errors and pending snapshot prompts.
- It preserves non-branch filters such as status, source, author, and text.

## Board Header

Show branch and write target near the board title:

```text
Kanban board
Branch: DI-445
Write target: master
```

When selected branch equals checkout branch, write target text can be compact:

```text
Branch: master (working tree)
```

## Editing UX

Working tree mode:

- File autosave works normally.
- Metadata save works normally.
- Status menu and drag/drop write normally.
- New item dialog writes normally.

Snapshot mode:

- Cards, details, and files are viewable.
- First edit to a snapshot item shows a confirmation dialog.
- Structured plans say Plan Manager will copy the whole plan folder.
- Freestyle docs say Plan Manager will copy the edited file.
- After confirmation, the write request includes `materializeConfirmed: true`.
- On success, the board reloads current checkout branch data enough to show the materialized item as working-tree content.

Confirmation copy:

```text
This item is loaded from branch DI-445. To edit it, Plan Manager will copy plans/DI-445 into the current checkout branch master, then apply your change there.
```

Docs copy:

```text
This file is loaded from branch docs-update. To edit it, Plan Manager will copy this file into the current checkout branch master, then apply your change there.
```

Conflict error:

```text
This snapshot item cannot be copied because files already exist in the current checkout branch. Resolve the conflict manually or switch branches first.
```

## Drag And Drop

Snapshot-mode drag/drop status changes are allowed only through materialization confirmation.

Flow:

```text
User drops snapshot card into another status
  -> board pauses optimistic move
  -> prompt explains whole-plan materialization
  -> confirmed request calls status endpoint with materializeConfirmed
  -> backend copies plan, applies status, returns working-tree item
  -> board reconciles with returned item and refreshes branch context
```

If the user cancels, no optimistic move is applied.

## Saved Filters

Branch is no longer part of multi-select filters. Saved Kanban views should omit `filters.branches`. If old saved views contain branch arrays, load only the first branch as selected branch and keep the rest ignored.

## Loading And Error States

Show concise states:

- `Loading branch...`
- `Refreshing branch...`
- `Failed to load branch snapshot`
- `Selected branch no longer exists`
- `Snapshot copied into master`
- `Snapshot copy blocked by existing files`

Do not show memory/YAML cache status in normal UI.

## Component Changes

| Component           | Change                                                              |
|---------------------|---------------------------------------------------------------------|
| `KanbanPage`        | Load branch result instead of all workspace items                   |
| `FacetMenu`         | Remove branch facet from normal filters                             |
| `PlanCard`          | Accept snapshot mode and route moves through confirmation           |
| `PlanPreviewDrawer` | Add materialization prompt around file, metadata, and status writes |
| `api` client        | Add branch load and materialization-aware write options             |
| `filtering.ts`      | Remove branch from `FilterKey` after migration support is handled   |

## Design Decisions

| Decision                                 | Rationale                                                              |
|------------------------------------------|------------------------------------------------------------------------|
| Keep branch selection in board header    | Branch controls the board’s content, not a secondary facet             |
| Prompt before first snapshot edit        | Copying content into checkout is useful but must be explicit           |
| Pause optimistic drag until confirmation | Avoid showing a move before the user agrees to materialize files       |
| Keep non-branch filters client-side      | Existing source/status/author/text filters still operate on one branch |
| Hide cache implementation details        | Cache status is not useful during normal branch browsing               |
