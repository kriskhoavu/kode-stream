# Frontend Design: AI Launch Composer, Capability Picker, And Session Layout Modes

## Overview

Enhance the Open AI Session dialog to make launch instructions explicit and editable, add provider-aware skill/agent selection, and extend the embedded session shell with richer layout controls. The UX should preserve current session surface and context controls while supporting movable floating windows and a full-height right-side panel mode.

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
| `ProviderCapabilityCatalogView` | `skills[]`, `agents[]`, support flags, scope/path metadata | Capability picker |
| `CapabilitySelectionState` | `selectedSkills[]`, `selectedAgents[]` | Launch payload builder |
| `LaunchPayload` | existing fields + prompt/capability fields | Embedded/external submit |
| `EmbeddedSessionPresentationMode` | `floating`, `side_panel`, `maximized`, `minimized` | Embedded dock shell |
| `FloatingLayoutState` | `x`, `y`, `width`, `height` | Floating shell geometry |

## State Management

| State | Owner | Behavior |
|-------|-------|----------|
| Preset selection | AI launch dialog | Sets default prompt draft and context hints |
| Prompt draft text | AI launch dialog textarea | Always editable, tracked for dirty state |
| Capability catalog | AI launch dialog | Reloads when provider changes |
| Capability selection | AI launch dialog | Clears or normalizes on provider switch |
| Submit eligibility | AI launch dialog | Requires provider/context readiness; capability catalog optional |
| Embedded session layout | App-level terminal dock | Tracks mode, geometry, and safe restore behavior while the dock is mounted |

## Prompt Composer UX

- Always render a textarea under the AI prompt selector.
- Selecting a preset preloads its prompt text.
- If user edits the textarea, mark prompt state as dirty.
- Show a small action: `Reset to preset prompt` when dirty and preset is selected.
- If `Free prompt` is selected, textarea starts empty and remains primary input.

## Embedded Session Presentation UX

- Keep one embedded session shell implementation and allow the user to switch presentation without relaunching.
- `Floating` mode is the default enhanced replacement for the current normal window mode.
- Floating mode supports:
  - drag by title bar,
  - resize from all four corners,
  - viewport clamping so the header and close controls never leave the screen,
  - bounded minimum and maximum size.
- `Right-side panel` mode docks the session to the right edge, uses full available application height, and keeps a stable width suited to chat-style interaction similar to Copilot-style side chat.
- Existing `minimized/collapsed` behavior remains available from either floating or right-side panel mode.
- Existing `maximized` behavior remains available as a distinct full-workspace focus mode.
- Mode switches should preserve terminal scrollback, focus behavior, and connection state.

## Capability Picker UX

- Show `Skills` and `Agents` sections below prompt composer.
- Load provider capabilities after provider selection using the current item/workspace context.
- Render capabilities grouped by `Workspace` and `Global` scope.
- Add lightweight search/filter inputs per section so large catalogs stay usable.
- Add `Select all` and `Clear` actions per scope group.
- Collapse long capability lists by default and expose an explicit expand/collapse control.
- Show source path for each capability and separate workspace/global tabs so the user can see where it came from.
- Show non-blocking guidance for empty catalog:
  - `No provider-defined workspace or global skills/agents were discovered for this provider. Launch will continue without capability injection.`
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
- Floating resize handles must remain pointer-friendly and visually discoverable.
- Right-side panel mode should degrade to maximized presentation on narrow widths if a docked panel would leave unusable remaining content space.
- Dragging and resizing should respect reduced-motion preferences where animated snapping is used.

## Error States

- Capability catalog load failure shows warning but does not block launch.
- Invalid/stale capability selections returned by API are surfaced as inline warnings.
- Prompt conflicts are shown near textarea and submit action.
- Default floating geometry starts from a visible bounded position and size.
- Viewport changes that would strand the floating window should clamp it back into view.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Keep one shared dialog for all prompt modes | Reduces mode switching and hidden behavior |
| Prompt is always visible and editable | Users can verify what will be sent before launch |
| Capability picker is provider-scoped | Prevents unsupported cross-provider selections |
| Show workspace and global sources separately | Users need to understand whether a capability belongs to the repo or their machine |
| Non-blocking catalog failures | Launch availability is more important than optional capability metadata |
| Preserve existing context/surface controls | PM-027 adds composition features without changing core session flow |
| Reuse the app-level dock from PM-020 | Session ownership already survives navigation and fits mode switching |
| Treat layout as presentation state, not session state | Drag/resize/dock changes should never affect PTY lifecycle |
| Prefer a right-docked panel over a second modal mode | Better matches chat-style workflows while keeping the main screen visible |
