# Frontend Design: AI Launch Prompt Composer And Capability Picker

## Overview

Enhance the Open AI Session dialog to make launch instructions explicit and editable, while adding provider-aware skill/agent selection. The UX should preserve current session surface and context controls and remain responsive on narrow layouts.

## Routes And Surface

| Route | Behavior |
|-------|----------|
| `/workstream` | User opens Open AI Session modal from board/intake workflows |
| `/items/:id` | User opens Open AI Session modal from item details workflows |

## Data Model

| Type | Fields | Owner |
|------|--------|-------|
| `AIPlanPreset` | `id`, `name`, `prompt`, `contextMode` | Preset selector |
| `PromptDraftState` | `selectedPresetId`, `promptText`, `dirty` | Prompt composer UI |
| `ProviderCapabilityCatalogView` | `skills[]`, `agents[]`, support flags | Capability picker |
| `CapabilitySelectionState` | `selectedSkills[]`, `selectedAgents[]` | Launch payload builder |
| `LaunchPayload` | existing fields + prompt/capability fields | Embedded/external submit |

## State Management

| State | Owner | Behavior |
|-------|-------|----------|
| Preset selection | AI launch dialog | Sets default prompt draft and context hints |
| Prompt draft text | AI launch dialog textarea | Always editable, tracked for dirty state |
| Capability catalog | AI launch dialog | Reloads when provider changes |
| Capability selection | AI launch dialog | Clears or normalizes on provider switch |
| Submit eligibility | AI launch dialog | Requires provider/context readiness; capability catalog optional |

## Prompt Composer UX

- Always render a textarea under the AI prompt selector.
- Selecting a preset preloads its prompt text.
- If user edits the textarea, mark prompt state as dirty.
- Show a small action: `Reset to preset prompt` when dirty and preset is selected.
- If `Free prompt` is selected, textarea starts empty and remains primary input.

## Capability Picker UX

- Show `Skills` and `Agents` sections below prompt composer.
- Load provider capabilities after provider selection.
- Render compact selectable chips or checkbox rows with description tooltips.
- Show non-blocking guidance for empty catalog:
  - `No provider-defined skills or agents available. Launch will continue without capability injection.`
- Keep picker keyboard navigable and screen-reader labeled.

## Submit Payload Behavior

- Include prompt text as user sees it at submit time.
- Include `presetId` when a preset remains selected.
- Include selected skills/agents arrays.
- Use same payload semantics for embedded and external launches.

## Responsive And Accessibility Behavior

- Prompt textarea expands vertically and avoids horizontal overflow.
- Capability lists stack cleanly on narrow widths.
- Long capability names wrap and keep control hit targets usable.
- Validation and load states use clear ARIA status messaging.

## Error States

- Capability catalog load failure shows warning but does not block launch.
- Invalid/stale capability selections returned by API are surfaced as inline warnings.
- Prompt conflicts are shown near textarea and submit action.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Keep one shared dialog for all prompt modes | Reduces mode switching and hidden behavior |
| Prompt is always visible and editable | Users can verify what will be sent before launch |
| Capability picker is provider-scoped | Prevents unsupported cross-provider selections |
| Non-blocking catalog failures | Launch availability is more important than optional capability metadata |
| Preserve existing context/surface controls | PM-027 adds composition features without changing core session flow |
