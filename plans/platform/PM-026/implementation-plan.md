# Implementation Plan: PM-026 - Runtime-Adaptive Verification Harness

## Overview

Implement a per-workspace runtime instruction contract and adapter-based execution harness so Kode Stream can run and verify applications automatically after AI implementation checkpoints, with fast defaults for large multi-service repositories.

## Runtime Contract Baseline

Every workspace runtime config must support:

- `runtime.type`: `docker-compose | procfile | makefile | custom`
- `runtime.configPath`: optional adapter path (compose file, Procfile path)
- `commands.up`
- `commands.down`
- `commands.verify.smoke`
- `commands.verify.critical` (optional)
- `commands.verify.full` (optional)
- `rebuildPolicy`: `never | changed-only | always`
- `healthChecks`: URLs or command-based checks
- `artifacts`: output directories and file globs

AI integration must additionally support:

- provider-neutral checkpoint events from Claude, Codex, OpenCode, and future providers
- terminal mode metadata (`embedded | external`) for run attribution and UX

## Phases Summary

| Phase | Name | Status |
|-------|------|--------|
| B1 | Runtime domain contracts and persistence | Pending |
| B2 | Adapter execution engine | Pending |
| B3 | Verification orchestrator and artifacts | Pending |
| B4 | Changed-only rebuild strategy | Pending |
| F1 | Runtime configuration UX | Pending |
| F2 | Workstream run and artifact UX | Pending |
| A1 | AI retry-loop integration | Pending |
| V1 | Cross-track verification and documentation | Pending |

## Backend Phases

### Phase B1: Runtime Domain Contracts And Persistence

**Deliverables:**

- [ ] Add runtime configuration models and validation rules.
- [ ] Persist runtime config per workspace in existing workspace settings flow.
- [ ] Add API endpoints to read and update runtime config safely.
- [ ] Enforce adapter-specific required fields at API boundary.
- [ ] Add tests for valid and invalid contracts plus migration-safe defaults.

**Verification:** `rtk go test ./internal/workspace ./internal/server/api`

**Commit:** `PM-026: Add workspace runtime contracts`

---

### Phase B2: Adapter Execution Engine

**Deliverables:**

- [ ] Add adapter interface and registry (`docker-compose`, `procfile`, `makefile`, `custom`).
- [ ] Implement adapter start, stop, and health primitives with normalized results.
- [ ] Support adapter-level environment injection and timeout controls.
- [ ] Normalize stdout and stderr capture for run logs.
- [ ] Add adapter unit tests and API integration coverage.

**Verification:** `rtk go test ./internal/runtime ./internal/server/api`

**Commit:** `PM-026: Add runtime adapter engine`

---

### Phase B3: Verification Orchestrator And Artifacts

**Deliverables:**

- [ ] Add `VerificationJob` orchestration with profile selection.
- [ ] Standardize verify lifecycle: prepare -> start -> health -> test -> collect.
- [ ] Standardize exit codes (`0`, `10`, `20`, `30`) and failure categories.
- [ ] Collect and publish artifacts (Playwright report, screenshots, video, trace, logs).
- [ ] Add read APIs for run timeline and artifact metadata.

**Verification:** `rtk go test ./internal/verification ./internal/server/api`

**Commit:** `PM-026: Add verification job orchestration`

---

### Phase B4: Changed-Only Rebuild Strategy

**Deliverables:**

- [ ] Add optional changed-path mapping from files to services/targets.
- [ ] Implement rebuild policy behavior:
  - [ ] `never`: reuse existing images/processes.
  - [ ] `changed-only`: rebuild impacted targets only.
  - [ ] `always`: force full rebuild.
- [ ] Integrate rebuild decisions into adapter execution.
- [ ] Add tests for diff classification and policy outcomes.

**Verification:** `rtk go test ./internal/runtime ./internal/verification ./internal/server/api`

**Commit:** `PM-026: Add changed-only rebuild policy`

## Frontend Phases

### Phase F1: Runtime Configuration UX

**Deliverables:**

- [ ] Add Runtime Settings section in workspace configuration.
- [ ] Support adapter selection and adapter-specific form fields.
- [ ] Support profile command and health-check editing.
- [ ] Add validation and inline guidance for large microservice setups.
- [ ] Add focused component tests for validation and save flows.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-026: Add runtime settings UI`

---

### Phase F2: Workstream Run And Artifact UX

**Deliverables:**

- [ ] Add run timeline states (`Preparing`, `Coding`, `Verifying`, `Fixing`, `Passed`, `Failed`).
- [ ] Show app preview URL and latest verification result per checkpoint.
- [ ] Add artifact links/viewers for logs, screenshots, video, trace, and report.
- [ ] Add rerun controls (`Re-run smoke`, `Re-run profile`) with busy-state handling.
- [ ] Add responsive and accessibility coverage.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-026: Add verification run UX`

## AI Integration Phase

### Phase A1: Retry-Loop Integration

**Deliverables:**

- [ ] Feed normalized failure summaries and artifact references into active AI sessions.
- [ ] Add provider-neutral checkpoint hooks to auto-trigger configured verification profiles.
- [ ] Ensure agents use harness commands instead of raw runtime commands by default.
- [ ] Support both embedded and external terminal sessions with identical verify semantics.
- [ ] Preserve existing AI launch behavior when no runtime config exists.

**Verification:** `rtk go test ./internal/ai ./internal/server/api && rtk npm test -- --run`

**Commit:** `PM-026: Add AI verification retry loop`

## Verification Phase

### Phase V1: Cross-Track Verification And Documentation

**Deliverables:**

- [ ] Verify docker-compose, Procfile, Makefile, and custom adapters end-to-end.
- [ ] Verify large compose workspace runs with `--no-build` baseline and changed-only rebuild.
- [ ] Verify profile behavior (`smoke`, `critical`, `full`) and deterministic artifact outputs.
- [ ] Verify failure categories and retry-loop payloads.
- [ ] Verify checkpoint ingestion works consistently across supported AI providers.
- [ ] Verify embedded and external terminal sessions both trigger and surface verification runs.
- [ ] Update architecture and user documentation for runtime contract and workflow.

**Verification:** `rtk go test ./... && rtk npm test -- --run && rtk npm run build && rtk git diff --check`

**Commit:** `PM-026: Verify runtime-adaptive harness`

## Testing Strategy

- Backend unit tests for contract validation, adapter behavior, orchestration states, and rebuild policies.
- Backend integration tests for runtime config APIs and verify job lifecycle.
- Frontend tests for runtime settings forms, run states, artifact rendering, and rerun controls.
- Adapter smoke tests using fixture workspaces for compose, procfile, makefile, and custom modes.
- Manual pass on one large microservice compose app to validate performance assumptions.

## Implementation Constraints

- Do not default to full rebuild on every verify run.
- Do not require all services to start for smoke profile.
- Do not bypass health checks before Playwright execution.
- Do not couple runtime config persistence to transient run state.
- Do not block existing AI workflows when runtime config is absent.
- Complete and commit one phase before starting the next.
