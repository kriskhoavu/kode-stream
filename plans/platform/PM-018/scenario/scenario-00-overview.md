# Scenarios: PM-018 External AI Session Launch

## Scenario List

| #   | Title                        | Expected Result                                                 |
|-----|------------------------------|-----------------------------------------------------------------|
| 1   | Open selected-card context   | AI receives the workspace-relative card path                    |
| 2   | Open workspace-only context  | AI opens at the workspace root without injected card context    |
| 3   | Missing provider or terminal | Launch is blocked with an actionable capability error           |
| 4   | Invalid custom template      | Settings reject unknown placeholders before process execution   |
| 5   | Snapshot-only item           | Card context is disabled while workspace-only remains available |
| 6   | Repeat preferred launch      | Main action reuses the last successful browser-local selection  |
| 7   | Change preferred launch      | Settings segment opens configuration without launching          |

## Flow 1: Launch an External Session

```text
User selects Open AI session
  -> UI loads detected capabilities and saved settings
  -> user selects provider, terminal, and context mode
  -> backend resolves the indexed item and registered workspace
  -> backend validates eligibility, executable, and template
  -> backend expands the workspace-relative card path when selected
  -> terminal adapter starts the provider in the workspace root
  -> API returns the accepted launch result
```

## Flow 2: Reject an Unsafe Launch

```text
Launch request
  -> workspace, item, context mode, or template validation fails
  -> no context-dependent process starts
  -> blocked audit event records identifiers but not prompt content
  -> UI displays the recovery hint
```

## Acceptance Scenarios

- Detection distinguishes installed, missing, overridden, and invalid tools.
- Provider authentication remains owned by each CLI.
- Card context is available for every editable working-tree item and does not require `plan.yaml`.
- Workspace-only context is available for any indexed item and passes no initial prompt.
- Neither context mode creates a context file or directory.
- Launch commands cannot introduce unapproved shell fragments through settings or item values.
- The first main-button click opens configuration when no browser preference exists.
- A successful configured launch becomes the next main-button choice on that browser.
- A failed remembered launch reports the error and reopens configuration.
