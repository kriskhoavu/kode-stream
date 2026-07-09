# Scenarios: PM-027 Overview

## Scenario List

| # | Title | Description |
|---|-------|-------------|
| 0 | Editable preset prompt before launch | User selects a preset, reviews prompt text, and edits it before opening session |
| 1 | Provider capability picker | User selects skills/agents based on provider catalog |
| 2 | Embedded and external launch parity | Composed prompt and selections apply equally across both surfaces |
| 3 | Prompt fallback mode | Provider lacks native skill/agent switches but prompt injection still works |
| 4 | Invalid capability selections | System rejects or normalizes stale/unsupported capability IDs safely |

---

# Scenario 0: Editable Preset Prompt Before Launch

## Starting State

- User opens Open AI Session from an item.
- A preset is selected (`Create implementation plan`, `Create technical design`, or `Create test scenarios`).

## Execution Flow

```text
Dialog loads presets
  -> selected preset text appears in editable textarea
  -> user edits the text to add constraints or scope
  -> launch request includes edited prompt draft + preset ID
  -> backend composes final instruction and starts session
```

## Expected Result

- User sees and can modify the exact instruction before launch.
- Edited prompt content is used by runtime launch command.
- Preset ID remains available for traceability.

---

# Scenario 1: Provider Capability Picker

## Goal

Allow users to choose provider-supported skills/agents and include them in launch instructions.

## Execution Flow

```text
User changes AI provider
  -> frontend requests provider capability catalog
  -> dialog lists available skills and agents
  -> user selects capability items
  -> selections are sent in launch payload
```

## Expected Result

- Capability options match selected provider.
- Unsupported providers return empty/non-blocking capability states.
- Selected capabilities are reflected in final composed prompt.

---

# Scenario 2: Embedded And External Launch Parity

## Goal

Ensure prompt composition and capability injection are identical across session surfaces.

## Execution Flow

```text
User launches embedded session with edited preset prompt + skills
  -> backend composes prompt and starts embedded process
User launches external session with same inputs
  -> backend composes equivalent prompt and starts external process
```

## Expected Result

- Both launches receive equivalent composed instructions.
- Prompt behavior does not regress by terminal mode.

---

# Scenario 3: Prompt Fallback Mode

## Goal

Keep capability selection usable when provider does not support native skill/agent switches.

## Execution Flow

```text
Catalog marks provider as prompt-fallback mode
  -> user still selects skills/agents
  -> backend appends capability directive block to prompt
  -> session starts normally
```

## Expected Result

- No launch failure due to missing native switches.
- Capability intent is still visible in the final prompt.

---

# Scenario 4: Invalid Capability Selections

## Goal

Protect launch behavior from stale or invalid capability IDs.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Capability removed after dialog opened | Backend ignores invalid ID and returns warning metadata |
| Provider switched after capability selection | UI clears incompatible selections |
| Payload includes unknown capability IDs | API rejects or normalizes according to strict mode policy |
| Catalog unavailable | Launch remains possible without capability selection |
