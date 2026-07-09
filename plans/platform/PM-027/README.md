# PM-027: AI Launch Prompt Composer And Capability Picker

PM-027 improves Open AI Session so users can review and edit the exact launch prompt before starting a session, and optionally select provider-supported skills/agents that are injected into the launch instruction. The ticket also fixes prompt resolution drift where preset selection can still launch with the default static card-context sentence.

## Scope

### Goals

- Add a visible, editable prompt textarea for all AI prompt modes (preset and free prompt).
- Preload selected preset content into the textarea and keep it user-editable before launch.
- Preserve preset identity for audit/analytics while sending the edited prompt text.
- Add provider-aware capability discovery for skills and agents.
- Let users select available skills/agents and inject them into the launch prompt deterministically.
- Support capability injection for both embedded and external AI sessions.
- Fix baseline prompt construction so preset text is actually used by default provider templates.

### Non-Goals

- No autonomous background multi-agent orchestration in this ticket.
- No per-user persistent library of custom presets yet.
- No cross-provider guarantee that native skill/agent switches exist.
- No replacement of provider executable detection/settings workflow.

## Glossary

| Term | Meaning | Code |
|------|---------|------|
| Prompt Composer | Launch-time instruction text assembled from preset/free prompt + optional capability directives | `LaunchPromptComposer` |
| Prompt Draft | Editable prompt content shown in the Open AI Session dialog | `promptDraft` |
| Capability Catalog | Provider-specific list of selectable skills and agents | `ProviderCapabilityCatalog` |
| Capability Injection | Prompt augmentation with selected skill/agent directives | `CapabilitySelection` |
| Native Capability Mode | Provider supports explicit skill/agent flags/arguments | `supportsNativeSelection` |
| Fallback Injection Mode | Provider lacks native capability flags; directives are appended to prompt text | `prompt_fallback` |

## Components

| Layer | Component | Purpose |
|-------|-----------|---------|
| AI Service | Prompt composer and capability catalog service | Resolve preset/free prompt, merge selections, and return final launch instruction |
| API | Preset and provider-capability endpoints | Expose prompt/capability contracts to frontend |
| Frontend | Open AI Session dialog | Prompt editor, preset sync behavior, skill/agent picker, and validation |
| Embedded Runtime | Embedded launch input builder | Ensure composed prompt and capability metadata are used for embedded sessions |
| External Runtime | External launch wrapper input builder | Ensure composed prompt and capability metadata are used for external sessions |
| Audit | Launch telemetry | Record preset ID, edited prompt usage, and selected capabilities |

## Data Flow

```text
User opens Open AI Session
  -> preset list and provider capability catalog are loaded
  -> user selects preset (or free prompt)
  -> preset prompt is inserted into editable textarea
  -> user edits prompt and optionally selects skills/agents
  -> launch request sends prompt draft + selected capabilities
  -> backend composes final provider instruction
  -> embedded/external session starts with composed prompt
```

```mermaid
flowchart TD
  A[Open AI Session Dialog] --> B[Load Presets]
  A --> C[Load Provider Capability Catalog]
  B --> D[Populate Editable Prompt Draft]
  C --> E[Render Skill/Agent Picker]
  D --> F[User Edits Prompt]
  E --> G[User Selects Capabilities]
  F --> H[Launch Request]
  G --> H
  H --> I[Compose Final Prompt Server-Side]
  I --> J[Start Embedded or External Session]
```

## Current Gap To Fix

- Current default provider argument templates use a static card-context sentence rather than the preset `Prompt` value.
- Result: selecting `Create implementation plan`, `Create technical design`, or `Create test scenarios` can still start with the same default sentence.
- PM-027 makes preset/free prompt composition explicit and ensures launch templates consume the composed prompt consistently.

## Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Keep prompt composition server-side | Build final prompt only in frontend | Server-side composition is consistent across embedded/external launch paths |
| Always show editable prompt textarea | Keep textarea only for free prompt | Users need visibility and control for presets before launch |
| Add provider capability catalog endpoint | Hard-code skill/agent options in frontend | Provider capabilities evolve and should be discovered centrally |
| Support fallback prompt injection mode | Block capability selection when no native provider support | Keeps UX useful across providers with uneven feature sets |
| Preserve preset ID even after editing | Drop preset identity once prompt is changed | Maintains traceability for audits and future analytics |

## Related Plans

| Ticket | Relationship | Key Context |
|--------|--------------|-------------|
| [PM-020](../PM-020/README.md) | AI session foundation | Owns embedded/external session runtime contracts |
| [PM-025](../PM-025/README.md) | Prompt preset origin | Added preset selection and free-prompt launch controls |
| [PM-026](../PM-026/README.md) | Verification loop integration | Uses AI session launch metadata for checkpoint attribution |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
