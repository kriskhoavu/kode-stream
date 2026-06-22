# PM-010: Per-Workspace Explorer Branch Selection

PM-010 lets users choose the checked-out branch for each workspace in Explorer. Each workspace keeps its own branch because every workspace is an independent Git repository.

## Related Plans

| Ticket                        | Relationship    | Key Context                                                                  |
|-------------------------------|-----------------|------------------------------------------------------------------------------|
| [PM-009](../PM-009/README.md) | Parent feature  | Added the multi-workspace Explorer tree, file search, and guarded text edits |
| [PM-008](../PM-008/README.md) | Related feature | Added Explorer Git states and workspace path mutations                       |
| [PM-003](../PM-003/README.md) | Related feature | Added guarded Git status and branch switching                                |

### What PM-009 Established

- **Workspace Explorer** shows several registered repositories in one tree.
- **Workspace root** identifies one repository and owns its file cache.
- **Explorer search** reads the currently checked-out working trees.
- **Raw editor** writes to the currently checked-out working tree with hash guards.

## Goals

- Show the current branch on every workspace root.
- List local branches for one workspace.
- Switch only the selected workspace repository.
- Block branch changes when the workspace has local changes.
- Clear stale files, search results, Git states, and tree data after checkout.
- Refresh indexed items after checkout.

## Out Of Scope

- One global branch choice across all workspaces.
- Browsing Git trees without checking them out.
- Remote-only branch discovery or automatic tracking branch creation.
- Automatic stash, commit, or discard before switching.
- Comparing files between branches.

## Glossary

| Term               | Meaning                                                          | Code Target               |
|--------------------|------------------------------------------------------------------|---------------------------|
| Workspace Branches | Current branch and local branches for one registered workspace   | `WorkspaceBranches`       |
| Current Branch     | Branch checked out in one workspace working tree                 | `current`                 |
| Branch Selector    | Workspace-root control that lists and switches local branches    | `WorkspaceBranchSelector` |
| Branch Refresh     | Workspace-scoped cache, search, Git state, and item invalidation | `refreshWorkspaceBranch`  |

## Components

| Layer    | Component                           | Purpose                                                 |
|----------|-------------------------------------|---------------------------------------------------------|
| Backend  | `application/git.Service.Branches`  | Resolve a workspace and list its current/local branches |
| Backend  | Workspace branches API              | Expose branch data to Explorer                          |
| Frontend | `WorkspaceBranchSelector`           | Load and switch one workspace branch                    |
| Frontend | `useWorkspaceExplorer` invalidation | Remove branch-stale tree and Git data for one workspace |

## Data Flow

```text
Open Explorer
  -> each workspace root requests its current and local branches
  -> selector displays the current branch

Choose another branch
  -> verify the editor has no pending changes
  -> request a guarded checkout for that workspace
  -> backend blocks a dirty or conflicted working tree
  -> backend refreshes indexed items
  -> frontend clears that workspace cache and search state
  -> expanded directories reload from the new working tree
```

## Design Decisions

| Decision                                 | Alternatives Considered           | Rationale                                                         |
|------------------------------------------|-----------------------------------|-------------------------------------------------------------------|
| Put branch selection on workspace roots  | Add one global branch filter      | Branch names and working trees belong to independent repositories |
| Perform a real checkout                  | Read files with `git show`        | Existing tree, editor, search, and Git APIs operate on files      |
| List local branches only                 | Include remote-only refs          | Existing switch validation and adapter support local branches     |
| Block dirty workspaces                   | Confirm or stash automatically    | Prevents surprising loss or movement of user changes              |
| Invalidate one workspace after switching | Reload every registered workspace | Other repositories and selections are unaffected                  |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)

## Implementation Result

- Added a workspace-scoped local branch query and API.
- Added guarded branch state and workspace-only Explorer invalidation.
- Added one native branch selector to every Explorer workspace root.
- Preserved dirty-tree protection and cleared branch-stale files and searches.
- Verified 151 backend tests and 72 frontend tests.
