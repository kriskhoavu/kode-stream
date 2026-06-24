# Backend Design: Kanban Branch Snapshot Materialization

## Overview

PM-013 adds branch-aware Kanban loading without checkout and a safe materialization path for snapshot edits. The scanner must read either the filesystem or a Git tree through one abstraction. The index must store items by workspace and branch. Writes from snapshot items must first copy safe content into the current checkout branch.

## Data Model

### Entity: ItemSummary Extension

| Field        | Type   | Purpose                                            |
|--------------|--------|----------------------------------------------------|
| `branch`     | string | Selected branch whose content produced the item    |
| `branchRef`  | string | Full Git ref, such as `refs/heads/DI-445`          |
| `commit`     | string | Commit SHA used for snapshot scans                 |
| `sourceMode` | string | `working_tree` or `snapshot`                       |
| `editable`   | bool   | Whether direct edits write without materialization |

### Entity: BranchScanMetadata

| Field                     | Type      | Purpose                                       |
|---------------------------|-----------|-----------------------------------------------|
| `workspaceId`             | string    | Registered workspace ID                       |
| `branch`                  | string    | Local branch name                             |
| `branchRef`               | string    | Full ref resolved for scanning                |
| `commit`                  | string    | Commit SHA at scan time                       |
| `sourceConfigurationHash` | string    | Hash of workspace sources and source settings |
| `scannedAt`               | time      | UTC scan completion time                      |
| `warnings`                | []warning | Branch-scoped scan warnings                   |

### Entity: BranchLoadResult

| Field                   | Type          | Purpose                          |
|-------------------------|---------------|----------------------------------|
| `workspaceId`           | string        | Workspace ID                     |
| `branch`                | string        | Selected branch                  |
| `branchRef`             | string        | Resolved ref                     |
| `commit`                | string        | Resolved commit                  |
| `currentCheckoutBranch` | string        | Current working tree branch      |
| `mode`                  | string        | `working_tree` or `snapshot`     |
| `editable`              | bool          | Whether edits can write directly |
| `scannedAt`             | time          | Scan/cache timestamp             |
| `itemCount`             | int           | Returned item count              |
| `warnings`              | []warning     | Branch warnings                  |
| `items`                 | []ItemSummary | Indexed branch items             |

### Entity: MaterializeSnapshotInput

| Field       | Type   | Purpose                                                    |
|-------------|--------|------------------------------------------------------------|
| `itemId`    | string | Snapshot item being edited                                 |
| `fileId`    | string | Optional file ID for file edits                            |
| `operation` | string | `save_file`, `save_metadata`, `update_status`, or `create` |
| `confirmed` | bool   | User accepted the first-edit copy into current checkout    |

## SourceReader

Add a scanner abstraction:

```go
type SourceReader interface {
    ReadDir(path string) ([]DirEntry, error)
    ReadFile(path string) ([]byte, error)
    WalkDir(root string, fn WalkFunc) error
    Stat(path string) (FileInfo, error)
}
```

Implementations:

| Reader                   | Use Case                                | Backing Data                         |
|--------------------------|-----------------------------------------|--------------------------------------|
| `FilesystemSourceReader` | Selected branch equals current checkout | Workspace filesystem                 |
| `GitTreeSourceReader`    | Selected branch differs from checkout   | Git object database at branch commit |

The scanner should accept `ScanRequest` with workspace, branch, ref, commit, reader, mode, and editable flag. Scanner code must stop using direct `os.ReadDir`, `os.ReadFile`, `os.Stat`, and `filepath.WalkDir` except inside `FilesystemSourceReader`.

## Git Adapter

Add read-only Git object methods:

| Method            | Git Command Shape                                     | Mutates Working Tree |
|-------------------|-------------------------------------------------------|----------------------|
| `ResolveBranch`   | `git rev-parse --verify refs/heads/{branch}^{commit}` | No                   |
| `TreeReadDir`     | `git ls-tree refs/heads/{branch} -- {path}`           | No                   |
| `TreeReadFile`    | `git show refs/heads/{branch}:{path}`                 | No                   |
| `TreeWalk`        | `git ls-tree -r refs/heads/{branch} -- {root}`        | No                   |
| `LastAuthorAtRef` | `git log -1 --format=%an {ref} -- {path}`             | No                   |
| `LastUpdateAtRef` | `git log -1 --format=%cI {ref} -- {path}`             | No                   |

Branch names must pass existing branch validation before forming refs.

## Index Persistence

Keep a flat item list but replace workspace-wide scan replacement with branch replacement.

```yaml
items:
  - workspaceId: ws1
    branch: master
    branchRef: refs/heads/master
    commit: abc123
    itemPath: plans/platform/PM-013
  - workspaceId: ws1
    branch: DI-445
    branchRef: refs/heads/DI-445
    commit: def456
    itemPath: plans/DI-445
branchScans:
  ws1:
    master:
      branchRef: refs/heads/master
      commit: abc123
      sourceConfigurationHash: h1
      scannedAt: 2026-06-24T00:00:00Z
    DI-445:
      branchRef: refs/heads/DI-445
      commit: def456
      sourceConfigurationHash: h2
      scannedAt: 2026-06-24T00:01:00Z
```

Add:

- `ReplaceWorkspaceBranch(workspaceID, branch string, items []ItemDetail, metadata BranchScanMetadata)`.
- `BranchScan(workspaceID, branch string)`.
- `BranchItems(workspaceID, branch string)`.

`DeleteWorkspace` still removes all branches for a workspace.

## Branch Load Flow

1. Resolve selected branch to `branchRef` and commit.
2. Read current checkout branch.
3. Choose mode:
   - `working_tree` when selected branch equals checkout branch.
   - `snapshot` otherwise.
4. Compute source configuration hash from workspace sources and source settings read through the selected source reader.
5. Check memory cache by `workspaceID + branchRef + commit + sourceConfigurationHash`.
6. Check YAML branch scan metadata for same branch, commit, and hash.
7. Scan through `FilesystemSourceReader` or `GitTreeSourceReader` on miss or `force=true`.
8. Persist with `ReplaceWorkspaceBranch`.
9. Return branch items to UI.

Working tree mode must also consider source mtimes or force refresh because uncommitted changes do not change commit SHA.

## Materialization Flow

For a snapshot edit:

1. Load the indexed item and workspace.
2. Verify selected item is from snapshot mode.
3. Verify user confirmation is present for first materialization.
4. Resolve current checkout branch.
5. Classify materialization scope:
   - Whole item directory for structured plans.
   - One edited file for freestyle docs or unsorted docs.
   - Whole item directory if docs path maps to a detected supported item.
6. Enumerate source files through `GitTreeSourceReader`.
7. Build target working-tree paths under the same relative paths.
8. Validate every target path against safety rules.
9. If any target file exists, block before writing anything.
10. Create parent directories and write copied files.
11. Apply the requested edit to the working-tree copy.
12. Refresh the current checkout branch index.

Materialization is all-or-nothing for the initial copy. If a copy fails after a file write, the service should report the partial failure and leave files visible for manual review; it must not run broad cleanup commands.

## API Contract

| Method | Endpoint                             | Request                                   | Response                            |
|--------|--------------------------------------|-------------------------------------------|-------------------------------------|
| POST   | `/api/workspaces/{id}/kanban/branch` | `{ "branch": "DI-445", "force": false }`  | `BranchLoadResult`                  |
| POST   | `/api/items/{id}/materialize`        | `MaterializeSnapshotInput`                | `WriteResult` or branch load result |
| PATCH  | `/api/items/{id}/metadata`           | Existing payload + `materializeConfirmed` | `WriteResult`                       |
| PATCH  | `/api/items/{id}/status`             | Existing payload + `materializeConfirmed` | `WriteResult`                       |
| POST   | `/api/items/{id}/files/{fileID}`     | Existing payload + `materializeConfirmed` | `FileContent`                       |

Existing edit endpoints may call the materialization service internally when the item is snapshot-derived.

## Safety Rules

- Never run checkout or switch from Kanban branch loading.
- Never run `git reset --hard`, `git checkout .`, `git restore .`, `git clean -fd`, or `git revert`.
- Git restore remains allowed only for explicitly validated plan paths.
- Every write, delete, restore, revert, and materialization target must be inside workspace root.
- Target must be inside configured sources.
- Target must belong to a detected supported plan item, or be the one selected freestyle docs file.
- Reject path traversal and absolute outside paths.
- Reject symlink escapes by resolving real paths.
- Block if target file already exists during snapshot materialization.

## Design Decisions

| Decision                                   | Rationale                                                                 |
|--------------------------------------------|---------------------------------------------------------------------------|
| Refactor scanner around `SourceReader`     | One scanner path should support filesystem and branch snapshots           |
| Persist branch scans separately            | Refreshing one branch must preserve other branch snapshots                |
| Materialize before applying snapshot edits | Writes stay in the current checkout branch and remain visible to IDEs     |
| Copy whole structured items                | Structured plans are multi-file units, not independent Markdown fragments |
| Copy one freestyle docs file by default    | Broad docs roots can include unrelated files                              |
| Block conflicts                            | Avoid silent overwrites in the current checkout branch                    |
