# Implementation Plan: PM-004 - Reliability, Safety, And Observability

## Overview

Add local audit events, workspace health checks, safer operation feedback, and stale-file recovery. Keep existing workflows and API behavior stable.

## Phases Summary

| Phase | Name                         | Status |
|-------|------------------------------|--------|
| B1    | Audit Domain And Storage     | ✅     |
| B2    | Health And Safety Services   | ✅     |
| B3    | Application Integration      | ✅     |
| B4    | Reliability Tests            | ✅     |
| F1    | API Types And Client Methods |        |
| F2    | Health And Audit State       |        |
| F3    | Safety Feedback UI           |        |
| F4    | Styling And Verification     |        |

## Backend Phases

### Phase B1: Audit Domain And Storage

**Deliverables:**

- [x] Add `AuditEvent` models.
- [x] Add local JSONL audit store in the app config directory.
- [x] Add append and recent-event read methods.
- [x] Add tests for append, read order, and malformed lines.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-004: Add local audit event storage

- Add audit event models
- Store audit events in local JSONL
- Add audit store tests
```

---

### Phase B2: Health And Safety Services

**Deliverables:**

- [x] Add workspace health service.
- [x] Check workspace path, sources, Git root, branch, file permissions, and index state.
- [x] Add safety service for common operation preflight checks.
- [x] Add tests for healthy, warning, and failed states.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-004: Add workspace health and safety services

- Add read-only workspace health checks
- Centralize operation preflight checks
- Add health service tests
```

---

### Phase B3: Application Integration

**Deliverables:**

- [x] Add `GET /api/workspaces/{id}/health`.
- [x] Add `GET /api/audit-events`.
- [x] Log scan, save, metadata, status, and Git events.
- [x] Add recovery hints for blocked stale writes and risky Git states.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-004: Integrate audit and health APIs

- Add health and audit endpoints
- Record operation results
- Return recovery hints for risky operations
```

---

### Phase B4: Reliability Tests

**Deliverables:**

- [x] Add endpoint characterization tests for health and audit APIs.
- [x] Add stale-file and Git-blocking tests.
- [x] Add path safety regression tests around new safety checks.
- [x] Add config corruption tests for audit reads.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-004: Add reliability regression tests

- Cover health and audit endpoints
- Cover stale-file and Git blocking behavior
- Cover audit read resilience
```

## Frontend Phases

### Phase F1: API Types And Client Methods

**Deliverables:**

- [ ] Add frontend types for `AuditEvent`, `WorkspaceHealth`, and `HealthCheck`.
- [ ] Add API client methods for audit and health endpoints.
- [ ] Normalize optional arrays and status values.
- [ ] Add API client tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-004: Add reliability API client types

- Add audit and health types
- Add API client methods
- Add normalization tests
```

---

### Phase F2: Health And Audit State

**Deliverables:**

- [ ] Add `useWorkspaceHealth`.
- [ ] Add `useAuditEvents`.
- [ ] Add refresh behavior after write and Git operations.
- [ ] Add tests for loading, empty, and error states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-004: Add health and audit frontend state

- Add health and audit hooks
- Refresh reliability data after operations
- Add hook tests
```

---

### Phase F3: Safety Feedback UI

**Deliverables:**

- [ ] Add workspace Health panel.
- [ ] Add recent Activity panel.
- [ ] Show recovery hints in Git and editor flows.
- [ ] Add stale-file recovery actions.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-004: Add reliability UI panels

- Add workspace health UI
- Add recent activity UI
- Show recovery hints for blocked operations
```

---

### Phase F4: Styling And Verification

**Deliverables:**

- [ ] Add compact styles for health and activity panels.
- [ ] Verify desktop and mobile layout.
- [ ] Run full build.
- [ ] Update embedded frontend assets.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Draft Commit:**
```text
PM-004: Finalize reliability UI

- Add health and activity styles
- Verify responsive layout
- Update embedded frontend build
```
