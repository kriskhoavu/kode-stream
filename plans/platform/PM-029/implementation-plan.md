# Implementation Plan: PM-029 - External Automation Verification Runner

## Overview

PM-029 is the active plan for external automation verification. The target state is a Quality workflow where runtime health checks, card-linked automation specs, visible-browser execution, and metadata-first spec discovery work together without relying on README frontmatter.

## Terminology Lock

Use these terms consistently:

- `Quality`
- `Runtime Verification Profile`
- `Automation Repository`
- `Selected Spec`
- `Planned Automation Path`
- `Discovered Spec`
- `Run automation tests`
- `Silent`
- `Visible browser`

## Phases Summary

| Phase | Name                                      | Status |
|-------|-------------------------------------------|--------|
| B1    | Runtime Automation Execution              | Done   |
| B2    | Metadata-First Spec Discovery             | Done   |
| B3    | Wiki Enrichment Metadata Migration Review | Done   |
| F1    | Workspace Settings And Quality UI         | Done   |
| F2    | Visible Browser Run Feedback              | Done   |
| V1    | Verification And Regression Coverage      | Done   |

## Backend Phases

### Phase B1: Runtime Automation Execution

**Deliverables:**

- [x] Workspace automation config with repo path, runner, environment, command template, and artifacts.
- [x] Item `verificationTests` selection with selected specs, environment, display mode, and timestamp.
- [x] Automation verification job mode that runs after runtime prepare/up/health.
- [x] Safe spec path validation inside the automation repository.
- [x] Runtime setup log, automation log, and collected artifact output.
- [x] Silent and visible browser command rendering.

**Verification:** `rtk go test ./internal/verification ./internal/item/... ./internal/server/api`

---

### Phase B2: Metadata-First Spec Discovery

**Deliverables:**

- [x] Add planned automation path parsing from `automation-test[].path` in automation repo `plan.yaml`.
- [x] Check likely plan YAML locations without scanning Markdown files.
- [x] Ignore empty placeholder paths.
- [x] Remove Markdown fallback for automation suggestions.
- [x] Add tests proving Markdown-only plan docs are ignored.

**Verification:** `rtk go test ./internal/item/... ./internal/server/api`

---

### Phase B3: Wiki Enrichment Metadata Migration Review

**Deliverables:**

- [x] Update wiki-enrich instructions to write `plan.wiki_enriched` in `plan.yaml`.
- [x] Stop writing `wiki_enriched` README frontmatter.
- [x] Confirm Knowledge page behavior is unaffected because Knowledge indexes docs wiki pages, not feature-plan enrichment flags.
- [x] No app code reads `wiki_enriched`, so no Knowledge page code change is required.

**Verification:** focused skill/doc review plus existing Knowledge tests if app code changes.

## Frontend Phases

### Phase F1: Workspace Settings And Quality UI

**Deliverables:**

- [x] Workspace sections for Overview, Health, and Integrations.
- [x] Focused integration detail views with Back navigation.
- [x] Runtime verification and Automation tests as sibling settings tabs.
- [x] Automation repository Browse action.
- [x] Right-panel Quality tab with runtime actions and automation controls.
- [x] Main item tabs for Plan, Explorer, and Git.
- [x] Spec browser rooted at the registered automation workspace with multi-select.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/ItemWorkspacePage.test.ts web/src/features/workspaces/WorkspaceDetails.test.tsx`

---

### Phase F2: Visible Browser Run Feedback

**Deliverables:**

- [x] Silent and Visible browser run mode toggle.
- [x] Automation run payload includes `displayMode`.
- [x] `Run automation tests` shows an in-button progress bar while launching/running.
- [x] Visible mode shows `Starting browser...` while Chrome/Chromium is starting.
- [x] Friendly artifact labels for `Automation log`, `Runtime setup log`, and reports.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/ItemWorkspacePage.test.ts`

## Verification Phase

### Phase V1: Verification And Regression Coverage

**Deliverables:**

- [x] Backend tests for `automation-test` YAML parsing and discovery precedence.
- [x] Backend tests that empty planned paths are ignored.
- [x] Frontend tests remain green for Quality panel, spec browse, display mode, and run payload.
- [x] Confirm `wiki_enriched` move does not affect Knowledge page indexing.
- [x] Confirm generated `internal/server/frontend/index.html` hash churn is not committed.

**Verification:** `rtk go test ./internal/item/... ./internal/server/api ./internal/verification && rtk npm run typecheck && rtk npm test -- --run web/src/pages/ItemWorkspacePage.test.ts web/src/features/workspaces/WorkspaceDetails.test.tsx`
