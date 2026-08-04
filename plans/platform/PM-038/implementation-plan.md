# Implementation Plan: PM-038 - Checkout-First Branch Context And Snapshot Review

## Overview

Replace ambiguous selected-snapshot state with a checkout-derived operational context, retain snapshots in a dedicated
read-only review route, and make cross-branch plan copying explicit.

## Phases Summary

| Phase | Name                                            | Track      | Status   |
|-------|-------------------------------------------------|------------|----------|
| P1    | Feature contract and browser playbook           | Full stack | Complete |
| B1    | Checkout loader, review, and explicit import    | Backend    | Complete |
| B2    | Canvas and Knowledge checkout synchronization   | Backend    | Complete |
| F1    | Checkout-first operational surfaces             | Frontend   | Complete |
| F2    | Dedicated read-only Branch Review               | Frontend   | Complete |
| I1    | Integration, durable documentation, and browser | Full stack | Complete |
| R1    | Fail-closed review and import remediation       | Full stack | Complete |
| R2    | Atomic import and snapshot verification         | Backend    | Complete |
| R3    | Routed import and index consistency             | Backend    | Complete |
| R4    | Review serialization and transactional index    | Full stack | Complete |
| R5    | Latest-request Branch Review state              | Frontend   | Complete |
| R6    | Commit-consistent reviewed file preview         | Frontend   | Complete |
| R7    | End-to-end commit-pinned file reads             | Full stack | Complete |
| R8    | Operational snapshot query isolation            | Backend    | Complete |
| R9    | Operational item detail isolation               | Full stack | Complete |
| R10   | Snapshot runbook and audit isolation            | Backend    | Complete |

## Phase P1: Feature Contract And Browser Playbook

**Deliverables:**

- [x] Lock checkout, review, import, compatibility, and synchronization terminology.
- [x] Document backend and frontend contracts plus safety failures.
- [x] Add a provider-neutral browser playbook for the coordinated journey.

**Verification:** Markdown formatting and plan review.

**Commit:** `PM-038: Define checkout-first branch review`

## Phase B1: Checkout Loader, Review, And Explicit Import

**Deliverables:**

- [x] Split checkout loading from commit-pinned branch review.
- [x] Add review and explicit structured-plan import APIs.
- [x] Reject snapshot mutations and legacy non-checkout operational loading.
- [x] Preserve branch caches while removing `lastSelectedBranch` behavior.
- [x] Add service tests for review isolation, conflicts, and compatibility plus transport error contracts.

**Verification:** `go test ./internal/workstream ./internal/item ./internal/server/api`

**Commit:** `PM-038: Add checkout and branch review services`

## Phase B2: Canvas And Knowledge Checkout Synchronization

**Deliverables:**

- [x] Make Canvas derive its layout branch from the checkout and ensure the item index is current before seeding.
- [x] Reject mismatching legacy Canvas branch requests.
- [x] Stamp Knowledge indexes with checkout branch and commit and rebuild after checkout changes.
- [x] Add direct Canvas load and Knowledge checkout-change tests.

**Verification:** `go test ./internal/canvas ./internal/knowledge ./internal/server/api`

**Commit:** `PM-038: Synchronize checkout projections`

## Phase F1: Checkout-First Operational Surfaces

**Deliverables:**

- [x] Remove snapshot selection and implicit materialization from Workstream and Item Workspace.
- [x] Remove Canvas branch selection and legacy branch route state.
- [x] Add shared checkout labels and focus/visibility invalidation.
- [x] Add route, page, and state tests for synchronized checkout changes.

**Verification:** `npm run typecheck && npm test -- --run web/src/app web/src/pages`

**Commit:** `PM-038: Align operational pages with checkout`

## Phase F2: Dedicated Read-Only Branch Review

**Deliverables:**

- [x] Add `/review` routing, review state, plan list/detail, and committed file reading.
- [x] Add the Review entry point to the operational workspace context.
- [x] Keep every mutation and execution control absent or disabled.
- [x] Add explicit import preview and guarded checkout actions.
- [x] Add read-only, missing-plan, import-conflict, and checkout tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/branch-review web/src/app`

**Commit:** `PM-038: Add read-only Branch Review`

## Phase I1: Integration, Durable Documentation, And Browser

**Deliverables:**

- [x] Run full Go, TypeScript, and frontend test suites.
- [x] Update architecture, affected plan history, and durable wiki guidance.
- [x] Execute the safe browser journey in a disposable two-branch Local workspace and record results.
- [x] Mark the plan done only after verification succeeds.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run`

**Commit:** `PM-038: Verify checkout-first branch context`

## Phase R1: Fail-Closed Review And Import Remediation

**Deliverables:**

- [x] Pin every snapshot tree read to the resolved commit while retaining the branch ref as metadata.
- [x] Require and revalidate the expected checkout branch immediately before import writes.
- [x] Reject an existing target item root, including empty directories and directories with unrelated files.
- [x] Refresh Branch Review checkout inventory after focus or visibility changes.
- [x] Add moved-ref, changed-checkout, target-root, and frontend refresh regression tests.

**Verification:** `go test ./internal/workstream ./internal/item ./internal/item/writer ./internal/server/api && npm run typecheck && npm test -- --run web/src/pages/BranchReviewPage.test.tsx web/src/features/workstream-explorer/useWorkspaceBranches.test.tsx web/src/shared/api/index.test.ts`

**Commit:** `PM-038: Close branch review race windows`

## Phase R2: Atomic Import And Snapshot Verification

**Deliverables:**

- [x] Serialize in-app checkout changes and reviewed-plan imports with a shared workspace-scoped mutation lock.
- [x] Hold the lock from destination checkout validation through import publication and index refresh.
- [x] Read snapshot verification selection and discovered specs from the reviewed commit.
- [x] Stage complete plans in a sibling temporary directory and atomically publish them without partial targets.
- [x] Add lock serialization, commit-pinned verification, failure cleanup, and retry regression tests.

**Verification:** `go test ./internal/git ./internal/item ./internal/item/writer ./internal/server/api`

**Commit:** `PM-038: Make reviewed plan import atomic`

## Phase R3: Routed Import And Index Consistency

**Deliverables:**

- [x] Require the reviewed item to belong to the workspace named by the import route before locking or writing.
- [x] Roll back an atomically published target when checkout index refresh fails so retry remains possible.
- [x] Serialize checkout scan-and-replace with import publication through the shared workspace mutation lock.
- [x] Add cross-workspace route, refresh rollback, retry, and checkout-load serialization regression tests.

**Verification:** `go test ./internal/workstream ./internal/item ./internal/item/writer ./internal/server/api`

**Commit:** `PM-038: Preserve routed import index consistency`

## Phase R4: Review Serialization And Transactional Index

**Deliverables:**

- [x] Hold the workspace mutation lock across review checkout validation, snapshot scanning, and index replacement.
- [x] Disable review branch switching while a snapshot refresh is loading.
- [x] Restore the complete prior file-index state when persistence fails after any replacement or deletion mutation.
- [x] Add review-lock, switch-guard, and failed-persistence ghost-item regression tests.

**Verification:** `go test ./internal/workstream ./internal/item/index ./internal/server/api && npm run typecheck && npm test -- --run web/src/pages/BranchReviewPage.test.tsx`

**Commit:** `PM-038: Serialize review refresh and index persistence`

## Phase R5: Latest-Request Branch Review State

**Deliverables:**

- [x] Assign each review load a monotonically increasing request identity and route-context key.
- [x] Allow only the latest request to update review, error, selected-plan, and loading state.
- [x] Add reverse-resolution coverage proving stale branch content and actions cannot replace the current branch.
- [x] Synchronize the frontend design and durable branch-context reference.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/BranchReviewPage.test.tsx && npm test -- --run`

**Commit:** `PM-038: Ignore superseded branch review responses`

## Phase R6: Commit-Consistent Reviewed File Preview

**Deliverables:**

- [x] Include the reviewed commit in the selected plan's file-loading identity.
- [x] Clear and reload the file tree and preview when refresh advances the commit with a stable item ID.
- [x] Add regression coverage returning different file content for the same item ID after refresh.
- [x] Synchronize the frontend design and durable commit-consistency reference.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/BranchReviewPage.test.tsx && npm test -- --run`

**Commit:** `PM-038: Reload reviewed files when commit advances`

## Phase R7: End-To-End Commit-Pinned File Reads

**Deliverables:**

- [x] Require and validate `expectedCommit` for snapshot file-tree and content endpoints.
- [x] Include the reviewed commit in client URLs so GET deduplication cannot cross commits.
- [x] Apply one request-generation guard to automatic and manual reviewed-file loads.
- [x] Add service, transport, API-client, and late-manual-response regression coverage.
- [x] Synchronize backend/frontend designs and the durable commit-consistency reference.

**Verification:** `go test ./internal/item ./internal/server/api && npm run typecheck && npm test -- --run web/src/pages/BranchReviewPage.test.tsx web/src/shared/api/index.test.ts && go test ./... && npm test -- --run`

**Commit:** `PM-038: Pin reviewed file requests end to end`

## Phase R8: Operational Snapshot Query Isolation

**Deliverables:**

- [x] Exclude reviewed snapshot rows from item listing, workspace consumers, and global search by default.
- [x] Keep snapshot rows available only through the explicit branch-scoped review query.
- [x] Reject snapshot diff and item content-search requests before they can inspect the checkout filesystem.
- [x] Preserve snapshot rows during storage migration and synchronization without exposing them operationally.
- [x] Add file-index, SQLite, search, service, and transport regression coverage.

**Verification:** `go test ./internal/item/index ./internal/item ./internal/search ./internal/storage ./internal/server/api && go test ./...`

**Commit:** `PM-038: Isolate snapshots from operational queries`

## Phase R9: Operational Item Detail Isolation

**Deliverables:**

- [x] Reject snapshot IDs from the operational item-detail service and API with `snapshot_review_only`.
- [x] Clear stale Item Workspace state before resolving a new item route.
- [x] Gate operational file and diff loading on successful working-tree item detail.
- [x] Render only loading or fail-closed navigation while item identity is unresolved or rejected.
- [x] Add service, transport, component, and reusable browser-runbook regression coverage.

**Verification:** `go test ./internal/item ./internal/server/api && npm run typecheck && npm test -- --run web/src/pages/ItemWorkspacePage.test.ts && go test ./... && npm test -- --run`

**Commit:** `PM-038: Reject snapshots from Item Workspace`

## Phase R10: Snapshot Runbook And Audit Isolation

**Deliverables:**

- [x] Reject snapshot E2E runbook requests before reading checkout plan, automation, or result files.
- [x] Resolve mutation audit identity through a raw index lookup that does not expose operational snapshot detail.
- [x] Keep blocked snapshot file, metadata, and status audit events scoped to their workspace and item.
- [x] Add service and transport regression coverage for both isolation boundaries.

**Verification:** `go test ./internal/item ./internal/server/api && go test ./...`

**Commit:** `PM-038: Isolate snapshot runbooks and mutation audits`
