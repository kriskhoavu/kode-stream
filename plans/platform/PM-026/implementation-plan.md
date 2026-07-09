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
| B1 | Runtime domain contracts and persistence | In Progress |
| B2 | Adapter execution engine | In Progress |
| B3 | Verification orchestrator and artifacts | In Progress |
| B4 | Changed-only rebuild strategy | In Progress |
| F1 | Runtime configuration UX | In Progress |
| F2 | Item details run and artifact UX | In Progress |
| A1 | AI retry-loop integration | In Progress |
| V1 | Cross-track verification and documentation | Pending |

## Current Implementation Flow Snapshot

```text
Workspace Runtime Config saved
  -> /api/workspaces/{id}/runtime
  -> stored on workspace registry

Manual verify
  -> /api/workspaces/{id}/verification-jobs
  -> job pipeline runs and writes artifacts

Auto verify (embedded)
  -> embedded session completion
  -> /api/workspaces/{id}/verification-checkpoints
  -> smoke job starts

Auto verify (external)
  -> launch wrapper script exits
  -> wrapper POSTs /verification-checkpoints
  -> smoke job starts

Workstream panel
  -> polls job state
  -> renders trigger badge + steps + artifacts + rerun
```

```mermaid
flowchart TD
  A[Save Workspace Runtime Config] --> B[/api/workspaces/{id}/runtime]
  B --> C[Registry Persistence]

  D[Manual Run Verify] --> E[/api/workspaces/{id}/verification-jobs]
  E --> F[VerificationJob]

  G[Embedded Session Complete] --> H[/api/workspaces/{id}/verification-checkpoints]
  I[External Wrapper Complete] --> H
  H --> F

  F --> J[prepare -> up -> health -> verify -> down]
  J --> K[Artifacts Indexed]
  K --> L[Workstream Poll + Render]
```

## Backend Phases

### Phase B1: Runtime Domain Contracts And Persistence

**Deliverables:**

- [x] Add runtime configuration models and validation rules.
- [x] Persist runtime config per workspace in existing workspace settings flow.
- [x] Add API endpoints to read and update runtime config safely.
- [ ] Enforce adapter-specific required fields at API boundary.
- [ ] Add tests for valid and invalid contracts plus migration-safe defaults.

**Verification:** `rtk go test ./internal/workspace ./internal/server/api`

**Commit:** `PM-026: Add workspace runtime contracts`

---

### Phase B2: Adapter Execution Engine

**Deliverables:**

- [x] Add adapter-style runtime execution service supporting `docker-compose`, `procfile`, `makefile`, and `custom` contract types.
- [x] Implement start, stop, and health primitives with normalized results.
- [ ] Support adapter-level environment injection and timeout controls.
- [ ] Normalize stdout and stderr capture for run logs.
- [ ] Add adapter unit tests and API integration coverage.

**Verification:** `rtk go test ./internal/runtime ./internal/server/api`

**Commit:** `PM-026: Add runtime adapter engine`

---

### Phase B3: Verification Orchestrator And Artifacts

**Deliverables:**

- [x] Add `VerificationJob` orchestration with profile selection.
- [x] Standardize verify lifecycle: prepare -> start -> health -> test -> collect.
- [x] Standardize exit codes (`0`, `10`, `20`, `30`) and failure categories.
- [x] Collect and publish artifacts (Playwright report, screenshots, video, trace, logs).
- [x] Add read APIs for run timeline and artifact metadata.

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
- [x] Integrate rebuild decisions into adapter execution (policy-driven command path).
- [ ] Add tests for diff classification and policy outcomes.

**Verification:** `rtk go test ./internal/runtime ./internal/verification ./internal/server/api`

**Commit:** `PM-026: Add changed-only rebuild policy`

## Frontend Phases

### Phase F1: Runtime Configuration UX

**Deliverables:**

- [x] Add Runtime Settings section in workspace configuration.
- [x] Support adapter selection and adapter-specific form fields.
- [x] Support profile command and health-check editing.
- [ ] Add validation and inline guidance for large microservice setups.
- [ ] Add focused component tests for validation and save flows.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-026: Add runtime settings UI`

---

### Phase F2: Item Details Run And Artifact UX

**Deliverables:**

- [ ] Add run timeline states (`Preparing`, `Coding`, `Verifying`, `Fixing`, `Passed`, `Failed`).
- [ ] Show app preview URL and latest verification result per checkpoint.
- [ ] Add artifact links/viewers for logs, screenshots, video, trace, and report.
- [x] Show latest verification result and trigger source per checkpoint.
- [x] Add artifact list with open-path actions.
- [x] Add artifact preview dialog for text logs.
- [x] Add artifact actions (`Open path`, `Re-run latest`) and step timeline details.
- [x] Add rerun controls (`Re-run smoke`, `Re-run profile`) with busy-state handling.
- [ ] Add responsive and accessibility coverage.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-026: Add verification run UX`

## AI Integration Phase

### Phase A1: Retry-Loop Integration

**Deliverables:**

- [ ] Feed normalized failure summaries and artifact references into active AI sessions.
- [x] Add provider-neutral checkpoint hooks to auto-trigger configured verification profiles.
- [x] Ingest embedded-session completion checkpoints and auto-trigger `smoke` verify by default.
- [x] Emit external-terminal completion checkpoints via launch wrappers so WezTerm and system terminals auto-trigger verify.
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
