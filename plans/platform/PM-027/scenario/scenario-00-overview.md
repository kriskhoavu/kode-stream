# Scenarios: PM-027 Overview

## Scenario List

| # | Title | Description |
|---|-------|-------------|
| 0 | Editable preset prompt before launch | User selects a preset, reviews prompt text, and edits it before opening session |
| 1 | Provider capability picker | User selects skills/agents based on provider catalog |
| 2 | Embedded and external launch parity | Composed prompt and selections apply equally across both surfaces |
| 3 | Floating embedded session window | User drags and resizes the embedded session in floating mode |
| 4 | Right-side panel mode | User docks the running session as a full-height side panel |
| 5 | Prompt fallback mode | Provider lacks native skill/agent switches but prompt injection still works |
| 6 | Invalid capability selections and layout state | System ignores stale capability IDs and keeps the floating shell visible during viewport/layout changes |

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
  -> frontend requests provider capability catalog for the current item workspace
  -> backend discovers matching workspace-local and user-global provider assets
  -> dialog lists available skills and agents grouped by Workspace and Global scope
  -> user filters long lists, expands collapsed groups when needed, and can bulk select within a scope
  -> user selects capability items
  -> selections are sent in launch payload
```

## Expected Result

- Capability options match selected provider.
- Capability options are limited to assets discovered for the selected provider in the current workspace or user environment.
- UI shows whether each capability comes from the workspace or the user machine.
- UI keeps large catalogs manageable through filtering, scoped bulk actions, and collapsed overflow lists.
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

# Scenario 3: Floating Embedded Session Window

## Goal

Let users treat the embedded AI session as a movable, resizable working window instead of a fixed dialog.

## Execution Flow

```text
User launches embedded AI session
  -> session opens in floating mode
  -> user drags the title bar to reposition the window
  -> user resizes from any corner
  -> dock clamps position and size to safe viewport bounds
```

## Expected Result

- The same running session remains active while the window moves or resizes.
- Window controls remain reachable after drag and resize actions.
- The terminal refits correctly after each size change.

---

# Scenario 4: Right-Side Panel Mode

## Goal

Support a chat-style AI workflow with the session docked as a full-height panel on the right side of the application.

## Execution Flow

```text
User opens an embedded AI session
  -> switches presentation from floating to right-side panel
  -> dock moves the same session into full-height right-edge layout
  -> user continues interacting with AI while keeping the main screen visible
```

## Expected Result

- The session is not restarted during the mode switch.
- The panel uses full available app height and a stable readable width.
- User can switch back to floating mode and recover the last valid floating geometry.

---

# Scenario 5: Prompt Fallback Mode

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

# Scenario 6: Invalid Capability Selections And Layout State

## Goal

Protect launch behavior from stale capability IDs and protect embedded layout behavior from off-screen or oversized floating geometry.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Capability removed after dialog opened | Backend ignores invalid ID and returns warning metadata |
| Provider switched after capability selection | UI clears incompatible selections |
| Payload includes unknown capability IDs | API rejects or normalizes according to strict mode policy |
| Catalog unavailable | Launch remains possible without capability selection |
| Floating position drifts off-screen during viewport changes | UI clamps back to a visible location |
| Floating size exceeds viewport | UI shrinks to bounded size and refits terminal |
