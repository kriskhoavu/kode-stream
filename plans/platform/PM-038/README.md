# PM-038: Checkout-First Branch Context And Snapshot Review

PM-038 makes the checked-out Git branch the only operational workspace context. Workstream, Item Workspace, Canvas,
Knowledge, editing, terminal launch, and verification stay aligned with the working tree. A separate Branch Review
surface retains commit-pinned snapshot reading without letting a review ref become application-wide mutable state.

## Related Plans

| Ticket                        | Relationship             | Key Context                                                                     |
|-------------------------------|--------------------------|---------------------------------------------------------------------------------|
| [PM-013](../PM-013/README.md) | Snapshot foundation      | Reuse Git-tree readers and branch-scoped indexes; replace edit materialization. |
| [PM-028](../PM-028/README.md) | Shared branch UI         | Replace operational snapshot pickers with checkout and explicit review actions. |
| [PM-037](../PM-037/README.md) | Canvas execution context | Keep plan, Git, terminal, and verification on one checkout-derived branch.      |

## Goals

- Derive operational branch context from Git rather than `lastSelectedBranch` or page-local snapshot state.
- Provide a dedicated, shareable, read-only review route for plans and committed plan files on another branch.
- Replace edit-triggered snapshot materialization with an explicit plan import into the checkout.
- Refresh operational surfaces after guarded in-app checkout and detected external checkout changes.
- Preserve branch-scoped indexes, Canvas layouts, and provider snapshot infrastructure.

## Non-Goals

- Snapshot-backed Knowledge graphs or arbitrary repository browsing.
- Editing, terminal launch, verification, or Git mutation inside Branch Review.
- Merge, overwrite, rename, or automatic conflict resolution during import.
- Automatic stash, reset, clean, or worktree creation.
- Removing Agentless Remote Snapshot provider contracts.

## Glossary

| Term                | Meaning                                                                       | Code                                     |
|---------------------|-------------------------------------------------------------------------------|------------------------------------------|
| Operational Context | Current checkout plus working-tree content used by mutable application pages. | `WorkstreamBranchLoadResult`             |
| Branch Review       | Commit-pinned, read-only view of another local branch's plans and files.      | `BranchReviewPage`, `BranchReviewResult` |
| Reviewed Commit     | Commit resolved when a review is loaded and required by import.               | `expectedCommit`                         |
| Plan Import         | Explicit copy of one structured snapshot plan into the checkout.              | `ImportReviewedPlan`                     |
| Checkout Refresh    | Re-index and UI invalidation after the Git checkout changes.                  | workspace content refresh                |

## Data Flow

```text
Open operational page
  -> resolve current checkout
  -> refresh/reuse working-tree indexes
  -> render checkout content and capabilities

Open Branch Review
  -> resolve requested branch and commit
  -> scan/reuse Git-tree snapshot
  -> render read-only plans and committed files

Import reviewed plan
  -> revalidate reviewed commit and checkout
  -> reject any existing target path
  -> copy the structured plan directory
  -> refresh checkout index
  -> open the imported operational item
```

## Design Decisions

| Decision                                      | Alternatives Considered              | Rationale                                                                       |
|-----------------------------------------------|--------------------------------------|---------------------------------------------------------------------------------|
| Checkout is the only operational branch       | Global selected snapshot             | Mutable actions and displayed Git state must share one authoritative context.   |
| Review uses a separate route                  | Mode toggle inside Workstream        | A route boundary makes read-only behavior visible, shareable, and testable.     |
| Review is strictly read-only                  | Copy automatically on first edit     | Editing must never mutate a branch different from the one the user is viewing.  |
| Import is an explicit action                  | Remove cross-branch copying entirely | Safe plan reuse remains useful when source and destination are unambiguous.     |
| Existing Canvas layouts remain branch-scoped  | Migrate to one workspace-wide layout | Returning to a checkout should restore its prior arrangement without data loss. |
| Legacy selection state is ignored, not erased | Destructive app-state migration      | Existing scans and layouts remain useful and rollback stays safe.               |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
- [UI Automation](automation/README.md)
