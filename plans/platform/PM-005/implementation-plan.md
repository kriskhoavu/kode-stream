# Implementation Plan: PM-005 - Search And Navigation

## Overview

Add global search, saved filters, recent items, and quick navigation. Use the existing item index first. Keep search read-only.

## Phases Summary

| Phase | Name                         | Status |
|-------|------------------------------|--------|
| B1    | Search Domain And Ranking    | ✅     |
| B2    | Saved Filters And Recents    | ✅     |
| B3    | Search APIs                  | ✅     |
| B4    | Backend Tests                | ✅     |
| F1    | API Types And Client Methods | ✅     |
| F2    | Search State And Keyboard    | ✅     |
| F3    | Search And Filter UI         |        |
| F4    | Styling And Verification     |        |

## Backend Phases

### Phase B1: Search Domain And Ranking

**Deliverables:**

- [x] Add `SearchQuery` and `SearchResult` models.
- [x] Add search service over `itemindex`.
- [x] Add simple ranking for identifier, title, scope, workspace, branch, tags, and description.
- [x] Add tests for ranking and limits.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-005: Add indexed item search domain

- Add search query and result models
- Search existing item index
- Add ranking tests
```

---

### Phase B2: Saved Filters And Recents

**Deliverables:**

- [x] Add saved filter models.
- [x] Add recent item models.
- [x] Store saved filters and recents in app config YAML.
- [x] Add tests for create, list, delete, and ordering.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-005: Add saved filters and recent items

- Store saved filters locally
- Store recent item navigation locally
- Add persistence tests
```

---

### Phase B3: Search APIs

**Deliverables:**

- [x] Add `GET /api/search`.
- [x] Add saved filter CRUD endpoints.
- [x] Add recent item endpoints.
- [x] Return stable frontend routes in search results.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-005: Add search and navigation APIs

- Add global search endpoint
- Add saved filter endpoints
- Add recent item endpoints
```

---

### Phase B4: Backend Tests

**Deliverables:**

- [x] Add API tests for search queries.
- [x] Add workspace-scoped and all-workspace search tests.
- [x] Add saved filter validation tests.
- [x] Add recent item ordering tests.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-005: Add search API regression tests

- Cover all-workspace search
- Cover saved filters
- Cover recent item ordering
```

## Frontend Phases

### Phase F1: API Types And Client Methods

**Deliverables:**

- [x] Add frontend types for search results, saved filters, and recent items.
- [x] Add API client methods.
- [x] Add response normalization tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-005: Add search frontend API client

- Add search and navigation types
- Add API client methods
- Add client tests
```

---

### Phase F2: Search State And Keyboard

**Deliverables:**

- [x] Add `useGlobalSearch`.
- [x] Add `useQuickSwitcher`.
- [x] Add keyboard shortcut handling.
- [x] Add result selection and route navigation behavior.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-005: Add search state and keyboard navigation

- Add global search state
- Add quick switcher state
- Add keyboard selection behavior
```

---

### Phase F3: Search And Filter UI

**Deliverables:**

- [ ] Add global search dialog.
- [ ] Add grouped result rendering.
- [ ] Add saved filter controls on Kanban.
- [ ] Add recent item section.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-005: Add search and saved filter UI

- Add global search dialog
- Add saved filters on Kanban
- Add recent item shortcuts
```

---

### Phase F4: Styling And Verification

**Deliverables:**

- [ ] Add compact search dialog styles.
- [ ] Verify keyboard and mobile behavior.
- [ ] Run full production build.
- [ ] Update embedded frontend assets.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Draft Commit:**
```text
PM-005: Finalize search navigation UI

- Add search dialog styles
- Verify keyboard and mobile behavior
- Update embedded frontend build
```
