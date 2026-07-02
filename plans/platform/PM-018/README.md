# PM-018: External AI Session Launch

PM-018 lets a user open Claude, Codex, Copilot, or OpenCode in an external terminal. Plan Manager selects the registered Git workspace and can either start clean or provide the selected card paths as neutral context before waiting for the user's request.

## Related Plans

| Item                          | Relationship                    | Key Context                                                     |
|-------------------------------|---------------------------------|-----------------------------------------------------------------|
| [PM-003](../PM-003/README.md) | Application architecture        | Reuse API, application-service, and frontend feature boundaries |
| [PM-015](../PM-015/README.md) | Current implementation baseline | Preserve local-only operation and verification conventions      |
| [PM-016](../PM-016/README.md) | Local Git integration           | Reuse workspace resolution, safety checks, and audit events     |
| [PM-017](../PM-017/README.md) | Local-first distribution        | Deliver macOS terminal adapters first and report capabilities   |

## Scope

### Goal

Launch an interactive AI CLI in the correct workspace with either workspace-only or selected-card context.

### Non-Goals

- No embedded terminal or PTY; PM-020 owns that work.
- No unattended or non-interactive AI execution.
- No automatic permission bypass for any provider.
- No repository file generated solely for session handoff.

## Glossary

| Term             | Meaning                                                       |
|------------------|---------------------------------------------------------------|
| AI Provider      | Supported CLI: Claude, Codex, Copilot, or OpenCode            |
| Terminal Adapter | Platform-specific launcher for Terminal, iTerm2, or WezTerm   |
| Context Mode     | User-selected `workspace_only` or `card_context` handoff      |
| Card Path        | Workspace-relative directory for the selected card            |
| Launch Template  | Executable and argument list containing approved placeholders |

## Data Flow

```text
Item workspace -> launch dialog -> capability/settings API
  -> launch request -> item and workspace validation
  -> optional card path -> provider command -> terminal adapter
  -> interactive CLI session in workspace root
```

## Design Decisions

| Decision                                       | Alternatives Considered    | Rationale                                                     |
|------------------------------------------------|----------------------------|---------------------------------------------------------------|
| External terminal first                        | Embedded PTY               | Delivers stable interaction before owning terminal lifecycle  |
| Pass the card path directly                    | Generate a context file    | Avoids temporary resources and lets the AI read current files |
| Context selection does not prescribe behavior  | Brainstorm/implement modes | The terminal user decides what the AI should do               |
| Workspace-only mode sends no initial prompt    | Empty context file         | Lets users manually reference workspace files and directories |
| Argument arrays with approved placeholders     | Arbitrary shell command    | Reduces quoting and command-injection risk                    |
| App-owned global settings                      | Settings in each workspace | Keeps machine-specific executable paths outside Git           |
| macOS terminal adapters first                  | Immediate cross-platform   | Matches the current supported distribution channel            |
| Remember the last successful launch in browser | Server-side user profile   | Enables one-click reuse without adding user-account state     |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
