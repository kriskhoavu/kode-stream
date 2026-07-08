# Scenarios: PM-013 Overview

## Scenario List

| #   | Title                          | Description                                                              |
|-----|--------------------------------|--------------------------------------------------------------------------|
| 1   | Load current checkout branch   | Kanban shows and edits the working tree branch                           |
| 2   | Load another branch snapshot   | Kanban shows a non-checkout branch without changing the working tree     |
| 3   | Materialize structured plan    | Editing a branch-only plan copies the whole plan into the checkout       |
| 4   | Materialize freestyle docs     | Editing a branch-only docs file copies only that file into the checkout  |
| 5   | Block materialization conflict | Existing target files prevent snapshot content from overwriting checkout |
| 6   | Refresh selected branch        | Refresh updates only the selected branch index                           |

## Scenario 1: Load Current Checkout Branch

## Goal

Show the current branch as normal editable Kanban content.

## Starting State

| #   | Title        | Summary                                     |
|-----|--------------|---------------------------------------------|
| 1   | Checkout     | Workspace is checked out on `master`        |
| 2   | Selected     | Kanban selected branch resolves to `master` |
| 3   | Working tree | Files are available through the filesystem  |

## Execution Flow

```text
User opens Kanban
  -> frontend requests branch context
  -> backend sees selectedBranch == currentCheckoutBranch
  -> scanner uses FilesystemSourceReader
  -> index replaces workspace + master
  -> board renders editable cards
```

## Expected Result

- Board title shows `Branch: master`.
- Mode is `working tree`.
- File edits, metadata edits, status moves, and new item creation write to the filesystem.
- IDEs see changes immediately.

## Scenario 2: Load Another Branch Snapshot

## Goal

Show a branch-only item without checking out the branch.

## Starting State

| #   | Title            | Summary                                             |
|-----|------------------|-----------------------------------------------------|
| 1   | Checkout         | Workspace is checked out on `master`                |
| 2   | Snapshot branch  | Local branch `DI-445` exists                        |
| 3   | Branch-only item | `DI-445` contains `plans/DI-445`, `master` does not |

## Execution Flow

```text
User selects DI-445
  -> frontend calls branch load API
  -> backend resolves refs/heads/DI-445 and commit
  -> scanner uses GitTreeSourceReader
  -> index replaces workspace + DI-445
  -> board renders DI-445 cards
```

## Expected Result

- Current checkout remains `master`.
- No `git checkout` or `git switch` runs.
- Board title shows `Branch: DI-445`.
- Write target shows `master`.
- The `plans/DI-445` card can be reviewed like a normal card.

## Scenario 3: Materialize Structured Plan

## Goal

Copy a structured snapshot plan into the current checkout branch on first edit.

## Starting State

| #   | Title           | Summary                                   |
|-----|-----------------|-------------------------------------------|
| 1   | Checkout        | Current checkout branch is `master`       |
| 2   | Selected branch | Kanban selected branch is `DI-445`        |
| 3   | Snapshot item   | `plans/DI-445` exists only in `DI-445`    |
| 4   | Target path     | `plans/DI-445` does not exist in `master` |

## Execution Flow

```text
User edits README.md on the DI-445 card
  -> frontend shows first-edit materialization confirmation
  -> user confirms
  -> backend validates item and target paths
  -> backend copies the whole plans/DI-445 directory from DI-445 into master
  -> backend applies the README.md edit in the working tree
  -> backend refreshes the master branch index
```

## Expected Result

- `plans/DI-445` appears in the current checkout branch working tree.
- The edited file contains the user's change.
- The current branch index includes the materialized item.
- The branch snapshot index remains available.

## Scenario 4: Materialize Freestyle Docs

## Goal

Avoid copying an entire broad docs tree when editing a snapshot docs card.

## Starting State

| #   | Title           | Summary                                                |
|-----|-----------------|--------------------------------------------------------|
| 1   | Checkout        | Current checkout branch is `master`                    |
| 2   | Selected branch | Kanban selected branch is `docs-update`                |
| 3   | Docs file       | Snapshot contains `docs/reference/install.md`          |
| 4   | Target file     | `docs/reference/install.md` does not exist in `master` |

## Execution Flow

```text
User edits docs/reference/install.md
  -> frontend shows first-edit materialization confirmation
  -> backend classifies item as freestyle docs
  -> backend copies only docs/reference/install.md into master
  -> backend applies the edit to that working-tree file
  -> backend refreshes current branch index if the path is under a configured source
```

## Expected Result

- Only the edited docs file is created in `master`.
- Kode Stream does not copy the full `docs/` source.
- If the docs path maps to a detected structured item, the structured whole-item rule applies instead.

## Scenario 5: Block Materialization Conflict

## Goal

Prevent snapshot materialization from overwriting current checkout files.

## Starting State

| #   | Title           | Summary                                       |
|-----|-----------------|-----------------------------------------------|
| 1   | Checkout        | Current checkout branch is `master`           |
| 2   | Selected branch | Kanban selected branch is `DI-445`            |
| 3   | Conflict        | `master` already has `plans/DI-445/README.md` |

## Execution Flow

```text
User edits snapshot DI-445 card
  -> frontend requests materialized write
  -> backend detects target path already exists
  -> backend blocks before writing any file
```

## Expected Result

- No working-tree file is overwritten.
- User sees: `This snapshot item cannot be copied because files already exist in the current checkout branch. Resolve the conflict manually or switch branches first.`
- The board remains on the selected snapshot branch.

## Scenario 6: Refresh Selected Branch

## Goal

Refresh only the branch currently selected in Kanban.

## Starting State

| #   | Title           | Summary                             |
|-----|-----------------|-------------------------------------|
| 1   | Indexed master  | Workspace has cached `master` items |
| 2   | Indexed DI-445  | Workspace has cached `DI-445` items |
| 3   | Selected branch | Kanban selected branch is `DI-445`  |

## Execution Flow

```text
User clicks Refresh
  -> frontend calls branch load API with force=true
  -> backend rescans DI-445 through the correct source reader
  -> index replaces only workspace + DI-445
```

## Expected Result

- `master` indexed items are preserved.
- `DI-445` indexed items reflect the latest branch commit.
- No checkout or switch happens.
