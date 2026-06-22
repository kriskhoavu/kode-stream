# Implementation Plan: PM-011 - Consolidate Primary Navigation

## Overview

Remove duplicate Items and Branches pages while retaining item detail and Git capabilities used by Kanban and Explorer.

## Phases Summary

| Phase | Name                          | Status |
|-------|-------------------------------|--------|
| F1    | Routes And Navigation Removal | ✅     |
| F2    | Documentation And Final QA    | ✅     |

## Frontend Phases

### Phase F1: Routes And Navigation Removal

**Deliverables:**

- [x] Remove desktop and mobile Items and Branches navigation.
- [x] Remove top-level route variants and rendering.
- [x] Delete the redundant page components and obsolete tests.
- [x] Keep `/items/{id}` item workspace navigation.
- [x] Cover legacy URL fallback behavior.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/App.test.tsx web/src/app/router.test.ts`

**Draft Commit:**
```text
PM-011: Remove redundant Items and Branches pages

- Consolidate item discovery in Kanban
- Keep repository branch selection in Explorer
- Preserve item workspace routes and supported APIs
```

### Phase F2: Documentation And Final QA

**Deliverables:**

- [x] Update README capabilities and architecture page inventory.
- [x] Record final PM-011 behavior.
- [x] Run the complete frontend and backend suites.
- [x] Build embedded production assets.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-011: Update consolidated navigation documentation

- Document Kanban and Explorer as primary discovery surfaces
- Remove obsolete page references from architecture docs
- Rebuild and verify embedded frontend assets
```

## Post-Implementation Checklist

- [x] Desktop navigation has no Items or Branches entries.
- [x] Mobile navigation has no Items or Branches entries.
- [x] `/items/{id}` still opens the item workspace.
- [x] `/items` and `/branches` fall back to Kanban.
- [x] Item search and Explorer branch controls remain functional.
