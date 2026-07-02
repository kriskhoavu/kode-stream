# Frontend Design: External AI Session Launch

## Overview

Add an AI settings section and an item-level launch dialog. The UI recommends detected tools but keeps selection explicit. It does not emulate a terminal or display provider output.

## Types and API

Add frontend equivalents of `AICapability`, `AISettings`, `LaunchTemplate`, `AISessionLaunchInput`, and `AISessionLaunchResult` to the shared type layer and API facade.

## State Management

| State                     | Owner                  | Behavior                                            |
|---------------------------|------------------------|-----------------------------------------------------|
| Capabilities and settings | AI settings hook       | Load on entry; explicit refresh reruns detection    |
| Launch selections         | Launch dialog          | Initialize from settings; reset between items       |
| Card-context availability | Item detail/API result | Disable card context for non-working-tree items     |
| Launch request and error  | Launch dialog          | Prevent duplicate submit and show recovery guidance |
| Last successful selection | Browser local storage  | Reuse provider, terminal, and context mode          |

## User Interface

- Settings lists provider and terminal presets with detected state, executable override, argument template, enable toggle, and default selection.
- Item workspace adds a split `Open AI session` action.
- The main segment opens configuration when no preference exists, then launches directly with the last successful selection.
- A colored indicator and tooltip identify the remembered provider, terminal, and context mode.
- The settings segment always opens configuration so the selection can be changed.
- Dialog requires provider, terminal, and context mode.
- Workspace-only explains that no card context is injected and workspace paths can be referenced manually.
- Card context explains that paths are provided without prescribing the AI's task.
- Successful launch closes the dialog and shows a transient confirmation.
- Errors remain in the dialog with backend recovery hints.
- A failed remembered launch reopens configuration for correction.

## Accessibility

- Dialog traps focus, supports Escape, and restores focus to the launch button.
- Capability state and validation errors use text in addition to color.
- Disabled card context references visible explanatory text.

## Design Decisions

| Decision                          | Rationale                                                    |
|-----------------------------------|--------------------------------------------------------------|
| Keep launch state local to dialog | No active external session is managed by the browser         |
| Select context, not behavior      | The user directs the interactive AI after terminal launch    |
| Offer workspace-only context      | Supports exploration without a predefined card task          |
| Put machine settings globally     | Provider and terminal availability is not workspace metadata |
| Remember choices in the browser   | The preference is device-local UI state, not shared settings |
