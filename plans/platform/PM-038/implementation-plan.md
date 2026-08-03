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
