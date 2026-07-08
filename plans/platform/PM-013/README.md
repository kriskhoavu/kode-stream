# PM-013: Kanban Branch Snapshot Materialization

PM-013 lets Kanban view one selected branch at a time without checking it out. When a user edits content loaded from a non-checkout branch, Kode Stream copies safe plan content into the current checkout branch and applies the edit there.

## Related Plans

| Ticket                        | Relationship     | Key Context                                                                               |
|-------------------------------|------------------|-------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Board baseline   | Added workspace registration, scanning, item index, Kanban, and read-only file access     |
| [PM-002](../PM-002/README.md) | Editing baseline | Added safe Markdown edits, metadata writes, status moves, new items, and Git operations   |
| [PM-010](../PM-010/README.md) | Branch contrast  | Added Explorer branch selection by real checkout; PM-013 does not checkout from Kanban    |
| [PM-012](../PM-012/README.md) | Kanban movement  | Added editable drag/drop status moves that PM-013 must guard for snapshot-derived content |

## Goals

- Replace the Kanban multi-branch filter with one selected branch.
- Load selected branch content through Git objects when it is not the current checkout branch.
- Never run `git checkout` or `git switch` when the Kanban branch selection changes.
- Keep the board content aligned with the selected branch.
- Keep writes aligned with the current checkout branch.
- Materialize a structured snapshot plan into the current checkout branch on first edit.
- Block materialization when target files already exist.
- Keep Kode Stream writes inside configured sources and detected plan item boundaries.

## Out Of Scope

- Merged or deduplicated multi-branch board views.
- Branch comparison.
- Arbitrary writes directly into a non-checkout branch.
- Overwrite conflict resolution.
- Copying entire broad docs roots such as `docs/`.
- Remote-only branch browsing.

## Glossary

| Term                    | Meaning                                                                                        | Maps To (code)              |
|-------------------------|------------------------------------------------------------------------------------------------|-----------------------------|
| Selected Branch         | The one branch whose content Kanban is showing                                                 | `selectedBranch`            |
| Current Checkout Branch | The branch currently checked out in the workspace working tree                                 | `GitStatus.branch`          |
| Working Tree Mode       | Mode used when selected branch equals current checkout branch; reads and writes the filesystem | `sourceMode: working_tree`  |
| Snapshot Mode           | Mode used when selected branch differs from checkout; reads committed Git objects              | `sourceMode: snapshot`      |
| Source Reader           | Scanner input abstraction for filesystem or Git tree reads                                     | `scanner.SourceReader`      |
| Branch Scan             | A scan result scoped to one workspace and branch                                               | `BranchScanMetadata`        |
| Branch-Aware Index      | Item index that stores items by workspace and branch                                           | `ReplaceWorkspaceBranch`    |
| Materialization         | Copying snapshot content into the current checkout branch before applying an edit              | `MaterializeSnapshotItem`   |
| Structured Plan         | A detected supported item directory such as `plans/platform/PM-013`                            | `ItemDetail.MetadataSource` |
| Freestyle Docs          | A docs source/card that is not a structured plan item                                          | `metadataSource: docs`      |

## Components

| Layer    | Component                  | Purpose                                                                |
|----------|----------------------------|------------------------------------------------------------------------|
| Backend  | Git tree source reader     | Read branch snapshots with `rev-parse`, `ls-tree`, and `show`          |
| Backend  | Filesystem source reader   | Read the current checkout working tree                                 |
| Backend  | Scanner                    | Scan either source reader without knowing where files came from        |
| Backend  | Branch-aware item index    | Replace and query one workspace branch without deleting other branches |
| Backend  | Materialization service    | Copy safe snapshot content into the current checkout before writes     |
| Backend  | Write safety guard         | Validate source roots, item ownership, symlink safety, and conflicts   |
| Frontend | Kanban branch selector     | Choose exactly one branch and label working tree versus snapshot modes |
| Frontend | Snapshot edit confirmation | Explain first-edit copy behavior before materialization                |
| Frontend | Kanban board/drawer        | Render selected branch content and route edits through materialization |

## Data Flow

```text
Open Kanban
  -> frontend loads current checkout branch and local branches
  -> backend chooses last selected branch, baseline branch, or checkout branch
  -> frontend asks backend to load the selected branch
  -> backend resolves branch ref and commit
  -> backend chooses FilesystemSourceReader or GitTreeSourceReader
  -> scanner indexes selected branch content
  -> item index replaces only workspace + selected branch
  -> frontend renders one-branch board

Edit snapshot item
  -> frontend shows first-edit materialization confirmation
  -> backend validates selected branch differs from checkout branch
  -> backend copies whole structured plan or one freestyle docs file into checkout
  -> backend blocks if any target file already exists
  -> backend applies the requested edit to the checkout working tree
  -> backend refreshes the current checkout branch index
  -> frontend shows the materialized item as editable working-tree content
```

## Design Decisions

| Decision                                     | Alternatives Considered                        | Rationale                                                                               |
|----------------------------------------------|------------------------------------------------|-----------------------------------------------------------------------------------------|
| One selected Kanban branch                   | Multi-select branch filter                     | A board should show one branch snapshot, not an implied merge or comparison             |
| No checkout on Kanban selection              | Reuse Explorer switch behavior                 | Selection is for viewing content; checkout remains a user-owned Git action              |
| Snapshot edits materialize into checkout     | Make snapshots read-only                       | Users want to review branch-only plans and copy useful changes into their active branch |
| Copy whole structured plans on first edit    | Copy only changed structured file              | Plan documents, metadata, scenarios, and designs should stay together                   |
| Copy one file for freestyle docs by default  | Copy entire docs source                        | Broad docs roots can be huge and unrelated to the edited file                           |
| Block existing target paths                  | Overwrite after confirmation, create copy path | PM-013 must not silently replace current-branch work or change plan identity            |
| Persist branch scans by workspace and branch | Replace all workspace items                    | Refreshing one branch must not delete snapshots for other branches                      |
| Keep conflict resolution out of scope        | Add merge/overwrite UI                         | The first version should prefer safe blocking over complex merge semantics              |

## Safety Boundary

Kode Stream only owns supported plan content under configured workspace sources. It must never reset, clean, checkout, or restore a whole repository.

Allowed write targets must pass all checks:

- Path is inside the registered workspace root.
- Path is inside configured sources.
- Path belongs to a detected supported plan item, or is the one selected freestyle docs file.
- Path traversal is rejected.
- Symlink escape is rejected.
- Existing target files block snapshot materialization.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
