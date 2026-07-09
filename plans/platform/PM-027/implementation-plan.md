# Implementation Plan: PM-027 - AI Launch Composer, Capability Picker, And Session Layout Modes

## Overview

Implement an editable prompt composer in Open AI Session, a provider-aware skill/agent selection flow, and improved embedded session presentation controls. Ensure composed prompts are applied consistently for both embedded and external launch modes, while embedded sessions can switch between movable floating and right-side panel layouts without restarting.

## Prompt Contract Baseline

Every launch path must support:

- `presetId` (optional) for named prompt intent.
- `customPrompt` or `promptDraft` for explicit editable launch text.
- `selectedSkills[]` and `selectedAgents[]` (optional).
- deterministic final prompt composition in backend.
- parity across `embedded` and `external` session surfaces.

Every embedded session layout must support:

- `floating` and `side_panel` presentation modes.
- mode switching without restarting the session transport.
- bounded client-side position and size state for floating mode.
- safe fallback to visible default geometry when stored layout is invalid.

## Phases Summary

| Phase | Name | Status |
|-------|------|--------|
| B1 | Prompt composition contract and bug fix | Completed |
| B2 | Provider capability catalog | Completed |
| B3 | Launch integration and audit updates | In Progress |
| F1 | Prompt editor UX for presets and free prompt | Completed |
| F2 | Skills/agents picker UX | Completed |
| F3 | Embedded session layout modes | In Progress |
| V1 | Cross-provider verification and documentation | In Progress |

## Backend Phases

### Phase B1: Prompt Composition Contract And Bug Fix

**Deliverables:**

- [x] Add explicit backend prompt composer that resolves preset + edited prompt + context directives.
- [x] Fix default provider template behavior so selected preset prompt is used by default.
- [x] Keep launch validation rule: either preset-derived prompt draft or explicit free prompt, not conflicting inputs.
- [x] Ensure `workspace_only` and `card_context` modes both apply the composed prompt consistently.
- [x] Add regression tests for preset selection producing distinct launch prompt text.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Add prompt composer and preset prompt fix`

---

### Phase B2: Provider Capability Catalog

**Deliverables:**

- [x] Add `GET /api/ai/providers/{id}/capabilities` endpoint.
- [x] Define `ProviderCapabilityCatalog` contract with skills, agents, and capability mode metadata.
- [x] Add provider-aware workspace/global capability discovery with safe fallback to empty catalog.
- [x] Support provider-specific workspace directories such as `.claude`, `.codex`, `.github`, `.agents`, and matching user-global locations where practical.
- [x] Restrict discovery to provider capability entrypoints so references, cache files, and state snapshots do not appear as selectable skills or agents.
- [x] Return capability scope and source-path metadata for UI grouping and traceability.
- [x] Include provider-level support flags (`supportsNativeSelection`, `supportsPromptFallback`).
- [x] Add tests for provider capability reads and unsupported provider behavior.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Add provider capability catalog API`

---

### Phase B3: Launch Integration And Audit Updates

**Deliverables:**

- [x] Extend launch inputs for selected skills/agents and resolved prompt draft fields.
- [x] Inject selected capabilities into final launch instruction (native or prompt fallback mode).
- [x] Apply same behavior for embedded and external launch services.
- [ ] Record preset ID, edited prompt indicator, and selected capabilities in audit event metadata.
- [x] Add integration tests covering both launch surfaces.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Wire capability injection into AI launch`

## Frontend Phases

### Phase F1: Prompt Editor UX For Presets And Free Prompt

**Deliverables:**

- [x] Always show editable prompt textarea in Open AI Session dialog.
- [x] On preset change, preload preset prompt into textarea unless user chooses to keep manual edits.
- [x] Keep `presetId` selection visible while allowing prompt edits.
- [x] Add clear reset action (`Use preset text again`) when prompt diverges.
- [x] Preserve existing context-mode and surface controls.
- [x] Preserve or hand off cleanly into embedded session state after launch.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-027: Add editable preset prompt composer UI`

---

### Phase F2: Skills/Agents Picker UX

**Deliverables:**

- [x] Load provider capability catalog when provider selection changes.
- [x] Render selectable skills and agents with improved visual grouping and compact rows that prioritize names and source paths over raw file content previews.
- [x] Group discovered capabilities by `Workspace` and `Global` scope.
- [x] Add search/filter, bulk selection, and overflow expansion controls so large catalogs remain usable.
- [x] Show capability source-path metadata in the picker and separate scope by tabs.
- [x] Handle empty/unavailable capabilities with non-blocking guidance.
- [x] Include selected capabilities in embedded/external launch payloads.
- [x] Add accessibility and responsive coverage for picker controls.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-027: Add provider capability picker to AI launch dialog`

---

### Phase F3: Embedded Session Layout Modes

**Deliverables:**

- [x] Add a presentation mode switch between `floating` and `right-side panel` for embedded AI sessions.
- [x] Make floating mode movable by dragging the window chrome, while constraining placement to the visible application viewport.
- [x] Make floating mode resizable from all four corners with bounded minimum and maximum dimensions.
- [x] Make right-side panel mode use full available application height and a stable docked width suitable for chat-style interaction.
- [x] Preserve the running session while switching between floating, panel, maximized, and minimized/collapsed states.
- [ ] Persist layout preference and last valid floating geometry in client state.
- [ ] Add focused tests for drag, resize, mode switching, viewport clamping, and restore behavior.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/features/ai-session && rtk npm run build`

**Commit:** `PM-027: Add embedded session layout modes`

## Verification Phase

### Phase V1: Cross-Provider Verification And Documentation

**Deliverables:**

- [x] Verify prompt composition differs correctly across all built-in presets.
- [x] Verify edited prompt is respected in both embedded and external launches.
- [ ] Verify capability selection behavior for native and fallback providers.
- [x] Verify no-regression for free prompt and workspace-only launch flows.
- [ ] Verify floating mode drag and four-corner resize behavior across supported desktop breakpoints.
- [x] Verify right-side panel mode uses full height, keeps the session interactive, and does not relaunch or drop terminal state.
- [x] Verify minimized/collapsed and maximized behavior still work after mode switches.
- [x] Update architecture and plan docs for PM-027 contracts and UI behavior.

**Verification:** `rtk go test ./... && rtk npm test -- --run && rtk npm run build && rtk git diff --check`

**Commit:** `PM-027: Verify prompt composer and capability picker`

## Testing Strategy

- Backend unit tests for prompt composition and capability injection rules.
- Backend integration tests for new capability endpoint and launch payload handling.
- Frontend component tests for preset-to-textarea sync, dirty-state handling, and picker selection.
- Frontend component tests for embedded dock presentation state, drag/resize math, and mode transitions.
- Manual smoke checks for embedded and external session launch parity plus floating/panel interaction.

## Implementation Constraints

- Do not remove existing preset API contract; extend safely.
- Do not assume every provider has native skills/agents runtime features.
- Do not silently override user-edited prompt text after manual edits.
- Do not diverge launch behavior between embedded and external session surfaces.
- Do not restart or recreate an embedded session only because presentation mode changes.
- Do not allow floating session chrome to become unreachable off-screen.
- Complete and commit one phase before starting the next.
