# Implementation Plan: PM-018 - External AI Session Launch

## Overview

Deliver provider detection, safe context selection, macOS external-terminal launch, and the configuration and item launch interfaces. The completed design opens at the workspace root and optionally gives the provider the selected card's workspace-relative path.

## Phases Summary

| Phase | Name                               | Status |
|-------|------------------------------------|--------|
| B1    | Capability And Settings Foundation | Done   |
| B2    | Context And Launch Service         | Done   |
| F1    | AI Settings                        | Done   |
| F2    | Item Launch Workflow               | Done   |
| V1    | Integrated Verification            | Done   |
| E1    | Workspace-Only Sessions            | Done   |
| E2    | Context-Only Session Model         | Done   |
| E3    | Direct Card Path Handoff           | Done   |
| E4    | Remembered Split Launch            | Done   |

## Phase B1: Capability And Settings Foundation

**Deliverables:**

- [x] Add AI settings models and `<data-dir>/ai-settings.yaml` store.
- [x] Detect supported providers and macOS terminals without starting them.
- [x] Validate executable paths, defaults, and approved template placeholders.
- [x] Add capability and settings endpoints with unit and API tests.

**Verification:** `go test ./internal/application/aisession ./internal/aisettings ./internal/api`

**Commit:** `PM-018: Add AI capability and settings foundation`

## Phase B2: Context And Launch Service

**Deliverables:**

- [x] Add item eligibility and workspace containment validation.
- [x] Add a guarded selected-card context handoff.
- [x] Add provider commands, Terminal/iTerm2/WezTerm adapters, and injected process runner.
- [x] Add launch endpoint, stable errors, audit events, and adapter tests.

**Verification:** `go test ./internal/application/aisession ./internal/ailaunch ./internal/api`

**Commit:** `PM-018: Launch external AI sessions with item context`

## Phase F1: AI Settings

**Deliverables:**

- [x] Add shared API types and methods.
- [x] Add capability/settings hook and Settings page section.
- [x] Support detected recommendations, explicit defaults, overrides, validation, and refresh.
- [x] Add interaction and error-state tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-settings`

**Commit:** `PM-018: Add AI provider and terminal settings`

## Phase F2: Item Launch Workflow

**Deliverables:**

- [x] Add item action and accessible launch dialog.
- [x] Add provider, terminal, and context selection.
- [x] Display card-context availability and recovery guidance.
- [x] Add successful, blocked, missing-tool, and duplicate-submit tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/ItemWorkspacePage.test.ts web/src/features/ai-session`

**Commit:** `PM-018: Add item AI session launch workflow`

## Phase V1: Integrated Verification

**Deliverables:**

- [x] Verify built-in adapters with fake executables and process runners.
- [x] Verify no context resource or terminal wrapper is written inside a workspace.
- [x] Update architecture, requirements baseline, and user documentation.
- [x] Run full backend, frontend, and production build checks.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/kode-stream`

**Commit:** `PM-018: Verify external AI session launch`

## Phase E1: Workspace-Only Sessions

**Deliverables:**

- [x] Add workspace-only context to backend and frontend launch contracts.
- [x] Launch the provider at the workspace root without provider prompt arguments or a context resource.
- [x] Allow workspace-only sessions for snapshot items while retaining card-context eligibility rules.
- [x] Explain manual file and directory references in the launch dialog.
- [x] Test context omission, snapshot behavior, and frontend submission.

**Verification:** `go test ./internal/application/aisession ./internal/api && npm run typecheck && npm test -- --run web/src/features/ai-session`

**Commit:** `PM-018: Add free prompt AI sessions`

## Phase E2: Context-Only Session Model

**Deliverables:**

- [x] Replace behavioral intents with `workspace_only` and `card_context`.
- [x] Remove `plan.yaml` and implementation-plan readiness requirements.
- [x] Make card context neutral and instruct the AI to wait for the user.
- [x] Provide selected-card context without prescribing behavior.
- [x] Update API, UI, migration behavior, tests, and product documentation.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run`

**Commit:** `PM-018: Simplify AI sessions to context selection`

## Phase E3: Direct Card Path Handoff

**Deliverables:**

- [x] Pass the workspace-relative card path directly to provider templates.
- [x] Remove obsolete context-resource configuration and generation.
- [x] Keep native-terminal wrappers self-deleting under the OS temporary directory.
- [x] Show guidance only for the selected context mode.
- [x] Migrate legacy built-in `{contextFile}` prompts to `{itemPath}`.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run`

**Commit:** `PM-018: Pass card paths directly to AI sessions`

## Phase E4: Remembered Split Launch

**Deliverables:**

- [x] Replace the single launch action with a main launch segment and a settings segment.
- [x] Store the last successful provider, terminal, and context mode in browser local storage.
- [x] Open configuration on the first launch and reuse the saved selection on later main-button clicks.
- [x] Reopen configuration when a remembered launch fails.
- [x] Indicate a saved choice with color and expose its details through the main action label and tooltip.
- [x] Test first-use, remembered-launch, settings, and failure behavior.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session && npm run build`

**Commit:** `PM-018: Remember AI session launch choices`
