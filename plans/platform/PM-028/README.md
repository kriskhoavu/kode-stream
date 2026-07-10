# PM-028: Unify Workstream And Explorer Surfaces

PM-028 consolidates the Workstream board, item details, and Explorer branch/file workflows so they behave as one surface instead of separate products with separate controls. The merged implementation already landed in the latest `PM-028` commit. This plan documents the shipped behavior and the remaining cleanup needed before the standalone Explorer page can be removed safely.

## Related Plans

| Item                          | Relationship            | Key Context                                                             |
|-------------------------------|-------------------------|-------------------------------------------------------------------------|
| [PM-007](../PM-007/README.md) | Explorer foundation     | Added the standalone filesystem Explorer route and editor shell         |
| [PM-010](../PM-010/README.md) | Explorer branch control | Added per-workspace branch switching in Explorer                        |
| [PM-013](../PM-013/README.md) | Snapshot branch model   | Added Workstream branch snapshots and materialization rules             |
| [PM-022](../PM-022/README.md) | Knowledge integration   | Added "Open in Explorer" deep-links for arbitrary workspace files       |
| [PM-025](../PM-025/README.md) | Workstream foundation   | Established Workstream as the canonical planning board and route owner  |

## Glossary

| Term                    | Meaning                                                                    | Code                     |
|-------------------------|----------------------------------------------------------------------------|--------------------------|
| Workstream Surface      | Primary planning surface for board, item navigation, and branch context    | `WorkstreamPage`         |
| Embedded Explorer       | File tree and editor shown inside item details                             | `WorkstreamExplorer`     |
| Standalone Explorer     | Global file browser route for arbitrary workspace paths                    | `/explorer` route        |
| Branch Snapshot Picker  | Shared branch dropdown with snapshot and checkout awareness                | `BranchSnapshotPicker`   |
| Current Checkout Branch | The real repository branch in the working tree                             | `currentCheckoutBranch`  |
| Snapshot Branch         | A branch loaded through read-only Workstream snapshot semantics            | `sourceMode: snapshot`   |

## Components

| Layer      | Component                            | Purpose                                                                  |
|------------|--------------------------------------|--------------------------------------------------------------------------|
| Frontend   | `WorkstreamPage`                     | Canonical board surface and branch-scoped item discovery                 |
| Frontend   | `ItemWorkspacePage`                  | Item detail surface with embedded Explorer and snapshot-aware branch UI  |
| Frontend   | `WorkstreamExplorer`                 | Shared file browser/editor shell, still used both embedded and standalone|
| Frontend   | `BranchSnapshotPicker`               | Shared branch dropdown for board and item detail                         |
| Frontend   | `useWorkspaceBranches`               | Workspace branch inventory and guarded checkout switching                |
| Backend    | Workspace file/tree APIs             | Provide file browsing, edits, search, and Git state for Explorer shells  |

## Data Flow

```text
Workstream
  -> open item
  -> item details
  -> embedded explorer
  -> branch snapshot picker
  -> snapshot content or working tree content

Knowledge
  -> open file in explorer
  -> standalone explorer route
  -> arbitrary workspace file browsing
```

## Design Decisions

| Decision                                              | Alternatives Considered                        | Rationale                                                                      |
|-------------------------------------------------------|------------------------------------------------|--------------------------------------------------------------------------------|
| Reuse one branch picker for Workstream and item detail| Keep two separate branch dropdown implementations | Shared behavior avoids divergent snapshot labels, search, keyboard, and sizing |
| Load item-detail branch changes through snapshots     | Perform real checkout from item details        | Item details must follow PM-013 snapshot semantics and avoid unsafe checkout   |
| Keep standalone Explorer temporarily                  | Delete `/explorer` immediately                 | Knowledge and arbitrary file browsing still depend on a non-item file surface  |
| Keep embedded Explorer inside item details            | Replace item file view with a second custom tree | Reuse preserves file editing, search, Git activity, and path mutations        |
| Remove old branch UI incrementally                    | Rewrite all Explorer controls in one pass      | The shipped merge focused on high-value duplication first                      |

## Current Scope

- Shared branch snapshot UI across Workstream and item details is shipped.
- Item detail branch changes now follow snapshot loading instead of real checkout.
- Embedded Explorer and Workstream visuals are aligned more closely.
- Standalone Explorer route still exists and is not dead code yet.

## Follow-Up Scope

- Move arbitrary file browsing into the Workstream surface or another canonical route.
- Rehome Knowledge deep-links away from `/explorer`.
- Remove standalone Explorer route plumbing after those flows are preserved.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Frontend Design](design/design-01-frontend.md)
- [Implementation Plan](implementation-plan.md)
