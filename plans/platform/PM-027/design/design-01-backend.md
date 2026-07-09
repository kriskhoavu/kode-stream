# Backend Design: AI Launch Composer, Capability Picker, And Session Layout Modes

## Overview

Extend AI launch services with a deterministic prompt composer and provider capability catalog. The backend must build the final launch instruction from preset/free prompt input, selected skills/agents, and context mode while preserving parity across embedded and external launches. Session layout mode changes introduced in PM-027 remain frontend-owned presentation state and do not require new server session contracts.

## Data Model

| Type | Key Fields | Purpose |
|------|------------|---------|
| `PlanPreset` | `id`, `name`, `prompt`, `contextMode` | Built-in prompt intent definitions |
| `PromptComposeInput` | `presetId`, `promptDraft`, `contextMode`, `selectedSkills[]`, `selectedAgents[]` | Server-side prompt composition input |
| `PromptComposeResult` | `resolvedPrompt`, `presetId`, `injectionMode`, `warnings[]` | Final launch prompt and metadata |
| `ProviderCapabilityCatalog` | `provider`, `skills[]`, `agents[]`, `supportsNativeSelection`, `supportsPromptFallback` | Provider-scoped capability discovery contract |
| `CapabilityDescriptor` | `id`, `name`, `description`, `kind`, `scope`, `sourcePath`, `provider` | User-selectable skill/agent item |
| `LaunchAuditMetadata` | `status`, `message`, `durationMs` | Current launch audit shape; capability-specific metadata is still follow-up work |

## API Contract

| Method | Endpoint | Request | Response |
|--------|----------|---------|----------|
| GET | `/api/ai/presets` | None | `AIPlanPreset[]` |
| GET | `/api/ai/providers/{id}/capabilities?itemId=...` | Optional current item/workspace context | `ProviderCapabilityCatalog` |
| POST | `/api/items/{id}/ai-sessions` | `LaunchInput` (extended) | `LaunchResult` |
| POST | `/api/items/{id}/ai-sessions/embedded` | `EmbeddedInput` (extended) | `EmbeddedResult` |

## Prompt Composition Flow

```text
launch input received
  -> validate context mode and provider availability
  -> resolve base prompt from preset or prompt draft
  -> validate and normalize selected capabilities
  -> choose injection mode (native vs prompt_fallback)
  -> compose final prompt text
  -> expand provider args with {prompt}
  -> start embedded or external session
```

```mermaid
flowchart TD
  A[LaunchInput] --> B[Resolve Base Prompt]
  B --> C[Load Provider Capability Catalog]
  C --> D[Normalize Selected Capabilities]
  D --> E{Native Capability Support?}
  E -->|Yes| F[Map to Provider Native Args]
  E -->|No| G[Append Prompt Directive Block]
  F --> H[Compose Final Prompt]
  G --> H
  H --> I[Expand Provider Args with {prompt}]
  I --> J[Start Session]
```

## Prompt Resolution Fix

- Current issue: default provider templates can use static prompt text and ignore preset `Prompt` content.
- PM-027 requirement: launch templates must include composed prompt placeholder semantics.
- If provider args do not include `{prompt}`, backend should either:
  - append composed prompt argument by convention, or
  - reject with explicit configuration validation error.

Recommended default behavior for backward compatibility:

- auto-append composed prompt argument when absent,
- emit warning metadata to support future migration to explicit `{prompt}` templates.

## Capability Discovery Model

- Provider capability catalogs can be sourced from:
  - workspace-local provider directories and files (for example `.claude/agents`, `.claude/commands`, `.claude/skills/<name>/SKILL.md`, `.codex/skills/<name>/SKILL.md`, `.github/chatmodes`, `.agents`, or provider-specific equivalents),
  - user-global provider directories and files in the current OS home/config area,
  - provider CLI introspection or local config override later when needed.
- Capability discovery should filter entries by selected provider and current item workspace.
- Capability discovery must ignore nested references, cache/state files, and arbitrary text/json/markdown documents that are not provider capability entrypoints.
- Catalog items should identify whether they came from `workspace` or `global` scope and expose a source path for UI traceability.
- Discovery failure must not block session launch.
- Catalog responses should be cacheable for short periods per provider.

## Validation Rules

- Reject launch when both incompatible prompt inputs are provided in strict mode.
- Normalize stale capability IDs by dropping anything not present in the provider catalog for the launch.
- Keep provider-neutral behavior when catalog is empty.

## Audit And Observability

- Keep existing `ai_session_launch` success/blocked/failed audit behavior.
- Do not block PM-027 launch parity on richer audit metadata; preset/capability audit enrichment remains follow-up work.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Compose prompt in backend | Prevents frontend/backend divergence between embedded and external launch |
| Preserve preset ID with editable text | Keeps launch intent traceable while supporting user control |
| Support native and fallback injection modes | Works across uneven provider feature sets |
| Non-blocking capability discovery failures | AI launch remains available even when catalogs cannot be loaded |
| Make `{prompt}` usage explicit | Fixes current preset prompt mismatch and prevents silent drift |
| Discover from workspace and user-global provider locations first | Depend on a static built-in skill list | Real provider assets already live beside the workspace or user profile and should drive the picker |
