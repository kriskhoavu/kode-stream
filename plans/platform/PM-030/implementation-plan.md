# Implementation Plan: PM-030 - Gin Transport And Core Boundary Hardening

## Overview

Implement Gin incrementally at the Go backend HTTP boundary while preserving existing routes, JSON contracts, local file behavior, and frontend expectations. Each phase must keep core packages free of Gin imports and must run focused verification before any old transport path is removed.

## Phases Summary

| Phase | Name                                               | Status      | Verification                                                                       |
|-------|----------------------------------------------------|-------------|------------------------------------------------------------------------------------|
| B1    | Baseline, route inventory, and proposal tightening | Complete    | `rtk go test ./...`                                                                |
| B2    | Domain error model and response mapper             | Not started | `rtk go test ./internal/common/... ./internal/server/...`                          |
| B3    | Gin bootstrap and middleware shell                 | Not started | `rtk go test ./internal/server/...`                                                |
| B4    | First read route-group migration                   | Not started | `rtk go test ./internal/server/... ./internal/navigation/... ./internal/audit/...` |
| B5    | Repository/service seams for migrated groups       | Not started | `rtk go test ./...`                                                                |
| B6    | Cache decorator pilot                              | Not started | `rtk go test ./...`                                                                |
| B7    | Concurrency policy pilot                           | Not started | `rtk go test ./...`                                                                |
| C1    | Boundary and CI checks                             | Not started | `rtk go test ./...`                                                                |
| C2    | Performance scorecard and old transport cleanup    | Not started | `rtk go test ./... && rtk npm run typecheck`                                       |

## Backend Phases

### Phase B1: Baseline, Route Inventory, And Proposal Tightening

**Deliverables:**

- [x] Route inventory for every `GET`, `POST`, `PUT`, `PATCH`, and `DELETE` route under `/api/`.
- [x] Migration risk label for each route group: low, medium, or high.
- [x] Baseline contract tests for selected first route group.
- [x] Baseline benchmark report for representative read routes.
- [x] Update `proposal.md` or add an amendment section with the narrowed migration gates from this plan.

**Result:** Added `route-inventory.md`, baseline controller contract tests, and benchmarks for `/api/health` and `/api/audit-events`.

**Verification:** `rtk go test ./...`

**Commit:** `PM-030: Add Gin migration baseline and route inventory`

---

### Phase B2: Domain Error Model And Response Mapper

**Deliverables:**

- [ ] Typed domain error codes for `not_found`, `validation`, `conflict`, `unauthorized`, `forbidden`, `unavailable`, and `infra`.
- [ ] Mapper from typed errors to HTTP status.
- [ ] Existing `httpx.WriteError` behavior preserved.
- [ ] Tests for current envelope fields and optional `code` field compatibility.
- [ ] Replace duplicated inline error mapping in one low-risk controller.

**Verification:** `rtk go test ./internal/common/... ./internal/server/... ./internal/navigation/...`

**Commit:** `PM-030: Add domain error mapping`

---

### Phase B3: Gin Bootstrap And Middleware Shell

**Deliverables:**

- [ ] Add Gin dependency.
- [ ] Add API transport bootstrap that can mount Gin route groups without changing SPA serving.
- [ ] Add middleware for recovery, request logging, request ID, and timeout where behavior is compatible.
- [ ] Add Gin response adapter that uses the same response envelope.
- [ ] Add tests proving non-migrated routes still resolve through the existing transport.

**Verification:** `rtk go test ./internal/server/... ./internal/common/...`

**Commit:** `PM-030: Bootstrap Gin API transport`

---

### Phase B4: First Read Route-Group Migration

**Deliverables:**

- [ ] Migrate `GET /api/health` and `GET /api/audit-events`, or another low-risk read group selected by the inventory.
- [ ] Add parity tests for status, content type, and JSON shape.
- [ ] Preserve query defaults such as audit `limit` behavior.
- [ ] Keep old route path available until parity tests pass.
- [ ] Document migration result in the route inventory.

**Verification:** `rtk go test ./internal/server/... ./internal/audit/... ./internal/workspace/...`

**Commit:** `PM-030: Migrate first read routes to Gin`

---

### Phase B5: Repository And Service Seams For Migrated Groups

**Deliverables:**

- [ ] Add narrow interfaces only for services or repositories used by migrated route groups.
- [ ] Add fakes for route and service tests.
- [ ] Keep concrete implementations unchanged unless tests expose a boundary issue.
- [ ] Confirm no Gin types enter service or repository signatures.
- [ ] Update backend design notes with final interface names.

**Verification:** `rtk go test ./...`

**Commit:** `PM-030: Harden service seams for Gin routes`

---

### Phase B6: Cache Decorator Pilot

**Deliverables:**

- [ ] Select one measured read-heavy candidate from the cache policy matrix.
- [ ] Add cache interface with TTL semantics and fake-clock tests.
- [ ] Add one decorator around the selected read path.
- [ ] Add explicit invalidation on related writes.
- [ ] Add hit, miss, and invalidation metrics or test-visible counters.

**Verification:** `rtk go test ./...`

**Commit:** `PM-030: Add measured cache decorator pilot`

---

### Phase B7: Concurrency Policy Pilot

**Deliverables:**

- [ ] Select one heavy workflow such as runtime verification, knowledge enrichment, Git command execution, or embedded AI session work.
- [ ] Add bounded worker or queue policy for that workflow.
- [ ] Add context deadline and cancellation propagation tests.
- [ ] Add queue-full and shutdown behavior tests.
- [ ] Document operational limits and defaults.

**Verification:** `rtk go test ./...`

**Commit:** `PM-030: Add bounded concurrency pilot`

---

## DevOps Phases

### Phase C1: Boundary And CI Checks

**Deliverables:**

- [ ] Add a test or script that fails when Gin is imported outside transport packages.
- [ ] Add route inventory check or golden snapshot where stable enough.
- [ ] Add verification instructions to the plan and repository README if needed.
- [ ] Confirm `go mod tidy` produces an expected dependency diff only.

**Verification:** `rtk go test ./...`

**Commit:** `PM-030: Add Gin boundary checks`

---

### Phase C2: Performance Scorecard And Old Transport Cleanup

**Deliverables:**

- [ ] Compare baseline and migrated route performance.
- [ ] Record p50, p95, allocation, and regression notes.
- [ ] Remove old `net/http` route code only for route groups with parity coverage.
- [ ] Run frontend typecheck to catch accidental API contract drift.
- [ ] Update proposal and README with final migration status.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-030: Remove migrated legacy transport paths`

## Post-Implementation Checklist

- [ ] Confirm no Gin imports exist outside the approved transport package.
- [ ] Confirm route inventory matches intended public API.
- [ ] Confirm all migrated routes preserve current JSON response contracts.
- [ ] Confirm cache invalidation is documented and tested.
- [ ] Confirm concurrency limits have cancellation and shutdown tests.
- [ ] Confirm `proposal.md` reflects the final phased plan rather than broad rewrite language.
