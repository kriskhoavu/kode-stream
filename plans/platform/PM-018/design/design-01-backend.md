# Backend Design: External AI Session Launch

## Overview

Add an `aisession` application service, app-owned settings store, provider adapters, and macOS terminal adapters. Process execution is abstracted for testing. The service resolves items through the index and workspaces through the registry before generating any manifest or command.

## Data Model

| Type                    | Key Fields                                                     |
|-------------------------|----------------------------------------------------------------|
| `AICapability`          | `id`, `kind`, `detected`, `executable`, `version`, `reason`    |
| `AISettings`            | `defaultProvider`, `defaultTerminal`, `providers`, `terminals` |
| `LaunchTemplate`        | `executable`, `args`, `enabled`                                |
| `AISessionLaunchInput`  | `provider`, `terminal`, `contextMode`                          |
| `AISessionLaunchResult` | `accepted`, `provider`, `terminal`, `contextMode`, `startedAt` |

Settings are stored in `<data-dir>/ai-settings.yaml`. Session context is passed through validated command arguments; no context file is created.

## Template Contract

Allowed placeholders are `{workspace}`, `{itemPath}`, `{identifier}`, and `{contextMode}`. Legacy `{contextFile}` and `{intent}` placeholders remain compatible but resolve to the card path and context mode. Each argument is expanded independently. Templates cannot specify environment overrides, redirections, pipes, command substitution, or additional working directories.

Built-in provider presets start an interactive selected-card session with an initial prompt containing `{itemPath}`. Built-ins never add flags that bypass provider approvals or sandboxing.

For `workspace_only`, the launcher omits all provider template arguments. For `card_context`, it supplies the workspace-relative card path with a neutral instruction to read relevant documents and wait for the user's request. Neither mode creates context resources or prescribes behavior.

## API Contract

| Method | Endpoint                                 | Request                | Response                     |
|--------|------------------------------------------|------------------------|------------------------------|
| GET    | `/api/ai/capabilities`                   | None                   | Capability list              |
| GET    | `/api/ai/settings`                       | None                   | Effective AI settings        |
| PUT    | `/api/ai/settings`                       | `AISettings`           | Validated effective settings |
| GET    | `/api/items/{id}/ai-session-eligibility` | None                   | Item launch eligibility      |
| POST   | `/api/items/{id}/ai-sessions`            | `AISessionLaunchInput` | `AISessionLaunchResult`      |

## Validation and Failure Modes

- Resolve the item and require `sourceMode=working_tree` and `editable=true`.
- Resolve the canonical workspace root and require the item path to remain beneath it.
- Resolve executables without invoking a shell and reject non-executable paths.
- Write manifests atomically with mode `0600`; remove expired files at startup and before launch.
- Return stable error codes: `ai_provider_missing`, `terminal_missing`, `invalid_context_mode`, `invalid_launch_template`, `item_not_editable`, and `launch_failed`.
- Audit success, blocked, and failed launches without arguments or manifest content.

## Terminal Adapters

- macOS Terminal: open a generated mode-0700 wrapper that changes to the validated workspace and executes the argument array.
- iTerm2: use macOS `open -a` with the validated app and wrapper paths.
- WezTerm: invoke `wezterm start --cwd <workspace> -- <provider args>` as an argument array.
- Wrapper files contain only pre-quoted validated arguments and remove themselves after process start.

## Design Decisions

| Decision                                | Rationale                                                    |
|-----------------------------------------|--------------------------------------------------------------|
| Separate capability and launch services | Detection must remain read-only and independently testable   |
| Inject process runner                   | Tests must not open real terminal applications               |
| Return acceptance, not PID              | External terminal owns the interactive child lifecycle       |
| Do not inherit custom environment       | Prevent settings from becoming a secret or execution channel |
