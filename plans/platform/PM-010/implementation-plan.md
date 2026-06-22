# Implementation Plan: PM-010 - Per-Workspace Explorer Branch Selection

## Overview

Add guarded local branch selection to each Explorer workspace root. Reload only the repository whose checked-out branch changed.

## Phases Summary

| Phase | Name                                  | Status |
|-------|---------------------------------------|--------|
| B1    | Workspace Branch Query And API        | ✅     |
| F1    | Branch Client And Workspace State     | ✅     |
| F2    | Explorer Branch Selector And Final QA | ✅     |

## Backend Phases

### Phase B1: Workspace Branch Query And API

**Deliverables:**

- [x] Add the `WorkspaceBranches` response model.
- [x] Add a workspace-scoped branch query to the Git application service.
- [x] Expose `GET /api/workspaces/{id}/git/branches` beside branch creation.
- [x] Sort, deduplicate, and preserve the current local branch.
- [x] Cover service and API behavior.

**Verification:** `rtk go test ./internal/application/git ./internal/api && rtk go test ./...`

**Draft Commit:**
```text
PM-010: Add workspace branch query API

- Return current and local branches per workspace
- Reuse registered workspace and Git safety boundaries
- Cover service and HTTP behavior
```

## Frontend Phases

### Phase F1: Branch Client And Workspace State

**Deliverables:**

- [x] Add the `WorkspaceBranches` frontend type.
- [x] Add the branches API client.
- [x] Add workspace-scoped branch loading and switching state.
- [x] Add workspace-scoped tree and Git cache invalidation.
- [x] Cover API and state behavior.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/shared/api web/src/features/workspace-explorer`

**Draft Commit:**
```text
PM-010: Add Explorer branch state

- Load branch choices for each workspace
- Switch one repository through the guarded Git API
- Invalidate only branch-stale workspace data
```

### Phase F2: Explorer Branch Selector And Final QA

**Deliverables:**

- [x] Render a compact branch selector on every workspace root.
- [x] Keep selector interaction separate from root expansion.
- [x] Save pending editor changes before checkout.
- [x] Clear switched workspace selection and unified search state.
- [x] Show workspace-local loading and errors.
- [x] Build and verify the complete application.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-010: Add per-workspace branch selection

- Show branch controls on Explorer workspace roots
- Refresh only the repository that changes branch
- Protect pending edits and dirty working trees
```

## Post-Implementation Checklist

- [x] Switching one workspace leaves other workspace roots unchanged.
- [x] Dirty workspaces cannot switch from Explorer.
- [x] Files missing on the new branch do not remain open.
- [x] Tree, search, Git states, and item data refresh after checkout.
- [x] Existing user changes remain unmodified and unstaged.
