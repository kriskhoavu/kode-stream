# Implementation Plan: PM-031 - Complete Gin API Migration

## Overview

Migrate the rest of the backend API from legacy `ServeMux` fallback to Gin route groups. Each phase moves one route family, removes its legacy registration after parity passes, and updates the route inventory. The final phase removes the fallback and leaves Gin as the only `/api/` router.

## Phases Summary

| Phase | Name                                       | Status      | Verification                                                                                 |
|-------|--------------------------------------------|-------------|----------------------------------------------------------------------------------------------|
| B1    | Route inventory v2 and parity harness      | Not started | `rtk go test ./internal/server/api/...`                                                      |
| B2    | Navigation and system route migration      | Not started | `rtk go test ./internal/navigation/... ./internal/system/... ./internal/server/api/...`      |
| B3    | State, search, AI settings route migration | Not started | `rtk go test ./internal/server/api/... ./internal/search/... ./internal/ai/...`              |
| B4    | Workspace read route migration             | Not started | `rtk go test ./internal/server/api/... ./internal/workspace/...`                             |
| B5    | Item read route migration                  | Not started | `rtk go test ./internal/server/api/... ./internal/item/... ./internal/search/...`            |
| B6    | Workspace and item write route migration   | Not started | `rtk go test ./... && rtk npm run typecheck`                                                 |
| B7    | Knowledge and verification route migration | Not started | `rtk go test ./internal/server/api/... ./internal/knowledge/... ./internal/verification/...` |
| B8    | Git route migration                        | Not started | `rtk go test ./internal/server/api/... ./internal/git/...`                                   |
| B9    | Streaming and WebSocket route migration    | Not started | `rtk go test ./internal/server/api/... ./internal/ai/...`                                    |
| C1    | Gin-only cutover and fallback removal      | Not started | `rtk go test ./... && rtk npm run typecheck`                                                 |
| C2    | Documentation, scorecard, and final checks | Not started | `rtk go test ./... && rtk npm run typecheck`                                                 |

## Backend Phases

### Phase B1: Route Inventory V2 And Parity Harness

**Deliverables:**

- [ ] Split PM-030 inventory into Gin-owned, fallback-owned, and removed route sections.
- [ ] Add route-family status table for every current `/api/` route.
- [ ] Add reusable API test helpers for JSON status, content type, error envelope, and query defaults.
- [ ] Add missing baseline tests for navigation and system route families.

**Verification:** `rtk go test ./internal/server/api/...`

**Commit:** `PM-031: Add full Gin migration inventory and parity harness`

---

### Phase B2: Navigation And System Route Migration

**Deliverables:**

- [ ] Migrate saved filters and recent items routes to Gin.
- [ ] Migrate system config path read/write routes to Gin.
- [ ] Decide whether native picker/open-path routes stay in this phase or move with high-risk writes.
- [ ] Preserve nil-repository behavior, JSON decode errors, route validation, and not-found responses.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/navigation/... ./internal/system/... ./internal/server/api/...`

**Commit:** `PM-031: Migrate navigation and system routes to Gin`

---

### Phase B3: State, Search, And AI Settings Route Migration

**Deliverables:**

- [ ] Migrate app state and indexed search routes.
- [ ] Migrate AI capabilities, presets, provider capabilities, settings read, and settings write routes.
- [ ] Preserve unavailable behavior when optional AI services are nil.
- [ ] Preserve settings validation and persisted YAML behavior.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/search/... ./internal/ai/...`

**Commit:** `PM-031: Migrate state search and AI settings routes to Gin`

---

### Phase B4: Workspace Read Route Migration

**Deliverables:**

- [ ] Migrate workspace list, runtime read, health read, source structure read, tree, file read, diff, path status, path search, and content search reads.
- [ ] Preserve path traversal guards, source filters, content limits, and file classification behavior.
- [ ] Add contract tests for representative file, tree, and content-search reads.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/workspace/...`

**Commit:** `PM-031: Migrate workspace read routes to Gin`

---

### Phase B5: Item Read Route Migration

**Deliverables:**

- [ ] Migrate item list, detail, AI eligibility, Jira read, Jira attachment, verification tests read, files, content search, file content, and diff routes.
- [ ] Preserve item not-found behavior, file ID mapping, Jira attachment guards, and content response shape.
- [ ] Add representative tests for Jira attachment and item file reads.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/item/... ./internal/search/...`

**Commit:** `PM-031: Migrate item read routes to Gin`

---

### Phase B6: Workspace And Item Write Route Migration

**Deliverables:**

- [ ] Migrate workspace create, import preview, import, update, delete, scan, runtime save, source structure save/reset, file write/create/revert, directory create, and path rename routes.
- [ ] Migrate item file save/revert, metadata/status patch, verification tests save, and item create routes.
- [ ] Preserve stale-hash recovery hints, scan refreshes, index updates, app state version changes, and audit events.
- [ ] Run frontend typecheck after migrated writes.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-031: Migrate workspace and item write routes to Gin`

---

### Phase B7: Knowledge And Verification Route Migration

**Deliverables:**

- [ ] Migrate knowledge wiki read routes and graph route.
- [ ] Migrate knowledge rescan, sync, and enrich action routes.
- [ ] Migrate verification job create, checkpoint ingest, job read, artifact read, and rerun routes.
- [ ] Preserve bounded verification policy, artifacts, job status responses, and knowledge not-found mapping.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/knowledge/... ./internal/verification/...`

**Commit:** `PM-031: Migrate knowledge and verification routes to Gin`

---

### Phase B8: Git Route Migration

**Deliverables:**

- [ ] Migrate Git status, activity, branches, fetch, pull, push, commit, create branch, and switch branch routes.
- [ ] Preserve dirty-state guards, branch validation, path scope validation, recovery hints, audit events, and scan refresh behavior.
- [ ] Add tests for conflict and blocked-operation responses.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/git/...`

**Commit:** `PM-031: Migrate Git routes to Gin`

---

### Phase B9: Streaming And WebSocket Route Migration

**Deliverables:**

- [ ] Migrate workspace stream-create route after normal workspace writes are stable.
- [ ] Migrate embedded AI session metadata, cancel, and WebSocket channel routes.
- [ ] Preserve WebSocket upgrade behavior, origin rules, message shape, reconnect, cancel, and shutdown cleanup.
- [ ] Add lifecycle tests for disconnect and cancellation where practical.
- [ ] Remove migrated legacy mux registrations.

**Verification:** `rtk go test ./internal/server/api/... ./internal/ai/...`

**Commit:** `PM-031: Migrate streaming and AI channel routes to Gin`

---

## DevOps Phases

### Phase C1: Gin-only Cutover And Fallback Removal

**Deliverables:**

- [ ] Remove legacy `ServeMux` fallback from the Gin transport.
- [ ] Remove API `mux.HandleFunc` registrations from `API.Routes()`.
- [ ] Update boundary test to fail on new API `ServeMux` route registrations.
- [ ] Update route inventory check to require all `/api/` routes to be Gin-owned.
- [ ] Confirm SPA serving still bypasses Gin and works through `internal/server`.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-031: Remove legacy API fallback`

---

### Phase C2: Documentation, Scorecard, And Final Checks

**Deliverables:**

- [ ] Update `README.md` and `ARCHITECTURE.md` from fallback migration language to Gin-only API language.
- [ ] Add final route-family status table and completion report to PM-031.
- [ ] Run representative benchmarks for health, audit, state/search, workspace read, and item read routes.
- [ ] Record performance notes and any accepted regressions.
- [ ] Confirm no unchecked PM-031 checklist items remain.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-031: Document completed Gin API cutover`

## Post-Implementation Checklist

- [ ] All `/api/` routes are registered on Gin.
- [ ] Legacy `ServeMux` fallback is removed from API transport.
- [ ] SPA serving remains unchanged.
- [ ] Gin import boundary still passes.
- [ ] Route inventory has zero fallback-owned routes.
- [ ] Frontend typecheck passes.
- [ ] README and architecture docs describe Gin-only final state.
