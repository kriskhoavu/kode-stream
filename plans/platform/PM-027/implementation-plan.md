# Implementation Plan: PM-027 - AI Launch Prompt Composer And Capability Picker

## Overview

Implement an editable prompt composer in Open AI Session and a provider-aware skill/agent selection flow so users can inspect and customize launch instructions before session start. Ensure composed prompts are applied consistently for both embedded and external launch modes.

## Prompt Contract Baseline

Every launch path must support:

- `presetId` (optional) for named prompt intent.
- `customPrompt` or `promptDraft` for explicit editable launch text.
- `selectedSkills[]` and `selectedAgents[]` (optional).
- deterministic final prompt composition in backend.
- parity across `embedded` and `external` session surfaces.

## Phases Summary

| Phase | Name | Status |
|-------|------|--------|
| B1 | Prompt composition contract and bug fix | Pending |
| B2 | Provider capability catalog | Pending |
| B3 | Launch integration and audit updates | Pending |
| F1 | Prompt editor UX for presets and free prompt | Pending |
| F2 | Skills/agents picker UX | Pending |
| V1 | Cross-provider verification and documentation | Pending |

## Backend Phases

### Phase B1: Prompt Composition Contract And Bug Fix

**Deliverables:**

- [ ] Add explicit backend prompt composer that resolves preset + edited prompt + context directives.
- [ ] Fix default provider template behavior so selected preset prompt is used by default.
- [ ] Keep launch validation rule: either preset-derived prompt draft or explicit free prompt, not conflicting inputs.
- [ ] Ensure `workspace_only` and `card_context` modes both apply the composed prompt consistently.
- [ ] Add regression tests for preset selection producing distinct launch prompt text.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Add prompt composer and preset prompt fix`

---

### Phase B2: Provider Capability Catalog

**Deliverables:**

- [ ] Add `GET /api/ai/providers/{id}/capabilities` endpoint.
- [ ] Define `ProviderCapabilityCatalog` contract with skills, agents, and capability mode metadata.
- [ ] Add provider adapters for capability discovery with safe fallback to empty catalog.
- [ ] Include provider-level support flags (`supportsNativeSelection`, `supportsPromptFallback`).
- [ ] Add tests for provider capability reads and unsupported provider behavior.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Add provider capability catalog API`

---

### Phase B3: Launch Integration And Audit Updates

**Deliverables:**

- [ ] Extend launch inputs for selected skills/agents and resolved prompt draft fields.
- [ ] Inject selected capabilities into final launch instruction (native or prompt fallback mode).
- [ ] Apply same behavior for embedded and external launch services.
- [ ] Record preset ID, edited prompt indicator, and selected capabilities in audit event metadata.
- [ ] Add integration tests covering both launch surfaces.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-027: Wire capability injection into AI launch`

## Frontend Phases

### Phase F1: Prompt Editor UX For Presets And Free Prompt

**Deliverables:**

- [ ] Always show editable prompt textarea in Open AI Session dialog.
- [ ] On preset change, preload preset prompt into textarea unless user chooses to keep manual edits.
- [ ] Keep `presetId` selection visible while allowing prompt edits.
- [ ] Add clear reset action (`Use preset text again`) when prompt diverges.
- [ ] Preserve existing context-mode and surface controls.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-027: Add editable preset prompt composer UI`

---

### Phase F2: Skills/Agents Picker UX

**Deliverables:**

- [ ] Load provider capability catalog when provider selection changes.
- [ ] Render selectable skills and agents with concise descriptions.
- [ ] Handle empty/unavailable capabilities with non-blocking guidance.
- [ ] Include selected capabilities in embedded/external launch payloads.
- [ ] Add accessibility and responsive coverage for picker controls.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-027: Add provider capability picker to AI launch dialog`

## Verification Phase

### Phase V1: Cross-Provider Verification And Documentation

**Deliverables:**

- [ ] Verify prompt composition differs correctly across all built-in presets.
- [ ] Verify edited prompt is respected in both embedded and external launches.
- [ ] Verify capability selection behavior for native and fallback providers.
- [ ] Verify no-regression for free prompt and workspace-only launch flows.
- [ ] Update architecture and plan docs for PM-027 contracts and UI behavior.

**Verification:** `rtk go test ./... && rtk npm test -- --run && rtk npm run build && rtk git diff --check`

**Commit:** `PM-027: Verify prompt composer and capability picker`

## Testing Strategy

- Backend unit tests for prompt composition and capability injection rules.
- Backend integration tests for new capability endpoint and launch payload handling.
- Frontend component tests for preset-to-textarea sync, dirty-state handling, and picker selection.
- Manual smoke checks for embedded and external session launch parity.

## Implementation Constraints

- Do not remove existing preset API contract; extend safely.
- Do not assume every provider has native skills/agents runtime features.
- Do not silently override user-edited prompt text after manual edits.
- Do not diverge launch behavior between embedded and external session surfaces.
- Complete and commit one phase before starting the next.
