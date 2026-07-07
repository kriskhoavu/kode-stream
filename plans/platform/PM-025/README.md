# PM-025: Jira-First Workspace

PM-025 renames the board surface from Kanban to Workspace and adds a Jira-first intake flow. A user can fetch a Jira ticket before any local item exists, create a structured item from that context, then start an AI session with either a free prompt or a saved preset.

## Related Plans

| Item                                  | Relationship           | Key Context                                                           |
|---------------------------------------|------------------------|-----------------------------------------------------------------------|
| [PM-002](../PM-002/README.md)         | Item creation base     | Owns structured item creation, guarded writes, and board status moves |
| [PM-019](../PM-019/README.md)         | Jira foundation        | Provides workspace Jira settings, normalized issue reads, and caching |
| [PM-020](../PM-020/README.md)         | AI terminal foundation | Provides embedded AI sessions and external terminal fallback          |
| [PM-024](../PM-024/README.md)         | Compatibility stance   | Confirms this project can remove old routes and legacy compatibility  |
| [PM-021](../pending-PM-021/README.md) | Separated Jira writes  | Jira mutation workflows stay outside this intake feature              |

## Glossary

| Term              | Meaning                                                                 | Code                      |
|-------------------|-------------------------------------------------------------------------|---------------------------|
| Workspace Surface | Main page for board views, intake, item opening, Jira context, and AI   | `WorkspacePage`           |
| Board View        | Kanban-style grouping inside the Workspace surface                      | existing board components |
| Jira Intake       | New-item flow that fetches a Jira issue before creating a local item    | `JiraIssueLookupState`    |
| Jira-Backed Item  | Structured item scaffolded from a Jira key, summary, labels, and people | `NewItemInput.jiraKey`    |
| AI Preset         | Named prompt template for starting implementation planning              | `AIPlanPreset`            |
| Free Prompt       | User-authored prompt passed to the chosen AI provider                   | `customPrompt`            |

## Components

| Layer      | Component                | Purpose                                                              |
|------------|--------------------------|----------------------------------------------------------------------|
| Jira       | Explicit issue lookup    | Fetch a normalized issue by workspace ID and Jira key before item ID |
| Item       | Jira-aware item creation | Scaffold README and metadata from fetched Jira context               |
| AI         | Prompt preset handoff    | Start existing AI sessions with preset or free-form prompt context   |
| Controller | Workspace and item APIs  | Expose Jira lookup, creation, and AI preset contracts                |
| Frontend   | Workspace surface        | Replace Kanban page identity and host intake, board, and AI launch   |

## Data Flow

```text
Workspace -> New Work Item -> From Jira
  -> fetch issue by workspace and key
  -> preview normalized Jira context
  -> create structured item with Jira defaults and README context
  -> rescan item index
  -> open created item
  -> optionally start embedded or external AI with a preset or free prompt
```

## Design Decisions

| Decision                                      | Alternatives Considered                       | Rationale                                                                         |
|-----------------------------------------------|-----------------------------------------------|-----------------------------------------------------------------------------------|
| Rename Kanban to Workspace without redirects  | Keep `/kanban` as an alias                    | The user explicitly does not need backward compatibility                          |
| Fetch Jira by workspace and key before create | Create blank item first, then use item lookup | BA, PM, and customer workflows often start with only a Jira ticket                |
| Create the local item before launching AI     | Launch AI against a temporary intake context  | Current AI sessions are item-path based and should work from real workspace files |
| Store Jira context in README only             | Persist a separate Jira snapshot file         | README gives the AI useful context without adding a stale second source of truth  |
| Add prompt presets before agent objects       | Build a full skills and agents registry       | Presets cover the first workflow while keeping provider execution unchanged       |
| Do not add Jira writes                        | Sync fields or transition Jira during intake  | PM-021 owns guarded Jira editing and mutation controls                            |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
