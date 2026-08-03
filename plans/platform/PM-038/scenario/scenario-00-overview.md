# Scenario 0: Review Another Branch Without Changing Operational Context

## Goal

> Review committed plans from another branch, optionally import one plan, and make that branch operational only through
> a guarded checkout.

## Starting State

| #   | Title                | Summary                                                                 |
|-----|----------------------|-------------------------------------------------------------------------|
| 1   | Checkout branch      | Workspace is checked out on branch A with optional uncommitted changes. |
| 2   | Review branch        | Branch B contains at least one committed structured plan.               |
| 3   | Operational surfaces | Workstream, Canvas, Knowledge, and Item Workspace are available.        |

## Execution Flows

### Flow 0.1: Review Without Checkout

```text
User opens Review branch
  -> backend acquires the workspace mutation lock
  -> backend confirms branch B is not the checkout and resolves it to a commit
  -> Git-tree scanner, file readers, and index replacement stay pinned to that commit SHA
  -> backend releases the workspace mutation lock
  -> review renders read-only plans and files
  -> Git checkout and branch A working tree remain unchanged
```

### Flow 0.2: Import A Reviewed Plan

```text
User previews Import into checkout
  -> backend requires the reviewed item to belong to the workspace named by the route
  -> backend acquires the workspace mutation lock shared with in-app checkout switching
  -> backend revalidates branch B commit and branch A destination under that lock
  -> destination item root is checked for any existing filesystem entry
  -> complete plan is staged beside the destination
  -> staged directory is atomically published into branch A working tree
  -> branch A index refreshes
  -> workspace mutation lock is released
  -> imported operational item opens
```

### Flow 0.3: Make Reviewed Branch Operational

```text
User chooses Switch workspace to this branch
  -> guarded checkout requests confirmation for dirty state
  -> Git performs the switch without reset or clean
  -> operational indexes refresh
  -> Workstream, Canvas, Knowledge, and Item Workspace reload branch B
```

## Expected Safety Outcomes

- Review never changes Git, files, Canvas layout, sessions, or verification.
- Import fails closed on moved refs, changed destination checkouts, existing item roots, unsafe paths, or symlink escape.
- Failed staging removes temporary content and leaves no target item root, so the import can be retried.
- Failed checkout index refresh rolls publication back, so the API error does not strand an occupied target.
- Failed file-index persistence restores the prior in-memory state, so rolled-back imports do not remain as ghost items.
- In-app checkout switching waits until import publication and checkout index refresh complete.
- Checkout load holds the same lock through scan-and-replace, so an older scan cannot overwrite the imported index.
- Review refresh holds that lock through checkout validation and snapshot scan-and-replace, so it cannot overwrite a
  newly operational checkout index.
- When review branch requests overlap, only the newest workspace, branch, and checkout context may update the page or
  enable review actions; superseded responses are ignored regardless of completion order.
- Snapshot mutation payloads return `snapshot_read_only` and never trigger a copy.
- External Git checkout changes are detected when the application regains focus.
