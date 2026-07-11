# Implementation Plan: PM-029 - External Automation Verification Runner

## Overview

Implement card-linked automation verification in behavior-preserving phases. Existing Smoke, Critical, and Full runtime profiles remain backward compatible. Automation adds optional workspace settings, item-level selected specs, test discovery, and a new verification job mode.

## Terminology Lock

All code, fields, API params, and UI labels should use:

- `Automation Repository`
- `Automation Runner`
- `Selected Spec`
- `Discovered Spec`
- `Run automation tests`
- `Runtime Verification Profile`

Avoid:

- `real automation` as a code term
- `test repo command` without runner context
- Reusing `smoke` or `critical` to mean card-linked automation specs

## Phases Summary

| Phase | Name                              | Status |
|-------|-----------------------------------|--------|
| B1    | Automation Config And Metadata    | Done   |
| B2    | Automation Verification Execution | Draft  |
| F1    | Frontend Types And API            | Draft  |
| F2    | Settings And Item Harness UI      | Draft  |
| V1    | End-To-End Verification           | Draft  |

## Backend Phases

### Phase B1: Automation Config And Metadata

**Deliverables:**

- [x] Add optional automation config to workspace runtime models and TypeScript-compatible JSON fields.
- [x] Normalize defaults for Cypress runner, local environment, command template, and artifact paths.
- [x] Validate enabled automation config without weakening existing runtime validation.
- [x] Add item verification-test read/save service for selected specs and optional environment override.
- [x] Add discovery service that scans matching automation plan docs for `cypress/e2e` and future `playwright` path references.
- [x] Add backend tests for normalization, validation, selected spec persistence, and discovery from plan docs.

**Verification:** `rtk go test ./internal/runtime ./internal/workspace/... ./internal/server/api`

**Commit:** `PM-029: Add automation config and selected specs`

---

### Phase B2: Automation Verification Execution

**Deliverables:**

- [ ] Extend verification job input and output with mode, environment, selected specs, automation repo path, and rendered command.
- [ ] Keep existing profile-only job creation as runtime mode.
- [ ] Validate selected specs are relative and stay inside the automation repository before command execution.
- [ ] Add automation execution after `prepare`, `up`, and `health`, running in the automation repository.
- [ ] Always attempt runtime teardown after automation pass or failure.
- [ ] Collect automation reports, videos, screenshots, and logs into the existing artifact response.
- [ ] Add tests for runtime-mode compatibility, automation pass, automation test failure, boot failure skip, and path traversal rejection.

**Verification:** `rtk go test ./internal/verification ./internal/runtime ./internal/server/api && rtk go test ./...`

**Commit:** `PM-029: Run selected automation specs from verification jobs`

---

## Frontend Phases

### Phase F1: Frontend Types And API

**Deliverables:**

- [ ] Extend runtime and verification TypeScript types with automation config and job metadata.
- [ ] Add API client methods for item verification-test read/save.
- [ ] Extend `createVerificationJob` input with automation mode, environment, and selected specs.
- [ ] Add focused tests or type coverage for runtime-mode backward compatibility.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-029: Add automation verification frontend contracts`

---

### Phase F2: Settings And Item Harness UI

**Deliverables:**

- [ ] Add Automation tests controls to workspace Runtime and verify settings.
- [ ] Show automation setup status in the item verification harness.
- [ ] Show discovered specs and selected specs with accept, remove, and manual-add actions.
- [ ] Persist selected specs before running automation.
- [ ] Add `Run automation tests` beside existing Smoke and Critical actions.
- [ ] Reuse current verification polling, steps, artifact preview, and open-path actions.
- [ ] Add frontend tests for settings editing, disabled states, spec selection, and automation run payload.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-029: Add automation verification UI`

---

## Verification Phase

### Phase V1: End-To-End Verification

**Deliverables:**

- [ ] Configure automation repository `/Users/kdvu/Documents/0. CC/1. Discovery/testing` in a local workspace.
- [ ] Confirm `DI-390` discovers Create Offer Cypress specs from the automation repo plan docs.
- [ ] Save selected specs for one card and reload the page to confirm persistence.
- [ ] Run `Run smoke verify` and confirm it still uses the existing runtime profile.
- [ ] Run `Run critical verify` and confirm it still uses the existing runtime profile or smoke fallback.
- [ ] Run `Run automation tests` against Cypress local environment and confirm artifacts are listed.
- [ ] Update PM-029 docs with any naming or behavior corrections found during implementation.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-029: Verify external automation runner`

## Post-Implementation Checklist

- [ ] Existing Smoke and Critical profile behavior is unchanged.
- [ ] Automation command execution rejects paths outside the automation repository.
- [ ] Card-selected specs are persisted explicitly.
- [ ] Discovery never runs tests by itself; it only suggests specs.
- [ ] Runtime teardown is attempted after automation failures.
- [ ] Planning docs match final API and UI names.
