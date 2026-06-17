# PM-002: Plan Editing And Git Operations

## Overview

PM-002 turns Plan Manager from a read-only browser into a safe local authoring tool.

Users can edit Markdown files, update plan metadata, create new plans, move cards across the Kanban board, and run guarded Git operations from the app. The app still runs locally. It still writes only to registered repositories that the user selected.

## Related Plans

| Ticket                        | Relationship   | Key Context                                                                                  |
|-------------------------------|----------------|----------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature | PM-001 created the read-only registry, scanner, plan index, Kanban board, and plan workspace |

### What PM-001 Established

- **Repository**: a local Git repository registered as a workspace.
- **Plan Directory**: a configured scan root such as `plans` or `docs`.
- **Plan**: a ticket-level folder shown on the board and in the workspace.
- **Freestyle Docs Root**: a Markdown docs folder that does not use the service/ticket shape.
- **Read-only boundary**: all PM-001 plan and Git APIs only read target repositories.
- **File access guard**: all file reads must stay inside configured plan directories.
- **App state version**: registry or index changes update `/api/state`.

## Glossary

| Term             | Meaning                                                                    | Maps To (code)              |
|------------------|----------------------------------------------------------------------------|-----------------------------|
| Repository       | A local Git repository registered in Plan Manager                          | `RepositoryConfig`          |
| Plan Directory   | A configured scan root such as `plans`, `docs`, or `docs/plans`            | `planDirectories`           |
| Plan             | A ticket-level planning folder or docs item                                | `PlanSummary`, `PlanDetail` |
| Plan Metadata    | Machine-readable plan fields stored in `plan.yaml`                         | `PlanMetadataUpdateInput`   |
| Edit Session     | The frontend state for one open file or metadata form with unsaved changes | editor state                |
| Dirty State      | Local Git state with modified, staged, untracked, or conflicting files     | `GitStatus`                 |
| Write Guard      | Backend checks that block unsafe file writes and risky Git operations      | file writer, Git adapter    |
| Git Operation    | A guarded local Git action such as fetch, pull, push, commit, or switch    | `GitOperationResult`        |
| Commit Draft     | User-entered commit message and selected plan paths                        | `GitCommitInput`            |
| Branch Operation | Branch create or branch switch from the active repository                  | branch request models       |

## Components

| Layer    | Component        | Purpose                                                                       |
|----------|------------------|-------------------------------------------------------------------------------|
| Backend  | Safe file writer | Writes editable files only inside configured plan directories                 |
| Backend  | Metadata writer  | Creates and updates `plan.yaml` without changing unrelated fields             |
| Backend  | Plan creator     | Creates a structured plan folder with starter documents                       |
| Backend  | Git adapter      | Runs guarded Git write operations with clear status and errors                |
| Backend  | HTTP API         | Exposes plan edit, status move, new plan, and Git operation endpoints         |
| Frontend | Editor state     | Tracks selected file, content, dirty state, save state, and conflict warnings |
| Frontend | Workspace editor | Adds Markdown editing, preview, metadata editing, and save controls           |
| Frontend | Kanban actions   | Moves status and opens new-plan flows from the board                          |
| Frontend | Git controls     | Shows branch and dirty state, then runs guarded Git operations                |

## Data Flow

```text
User opens a plan
  -> frontend loads plan detail, files, file content, diff, and Git status
  -> user edits Markdown or metadata
  -> frontend marks the edit session dirty
  -> user saves
  -> backend validates repository, plan ID, file ID, and path scope
  -> backend writes file or plan.yaml
  -> backend rescans the affected repository
  -> plan index and app state version update
  -> frontend refreshes board, workspace, and stale-content state

User runs a Git operation
  -> frontend requests Git status
  -> backend reports branch, dirty files, staged files, and divergence
  -> frontend asks for confirmation when the operation is risky
  -> backend runs the guarded Git command
  -> backend rescans when repository content changed
  -> frontend shows the result and refreshed status
```

## Design Decisions

| Decision                              | Alternatives Considered                 | Rationale                                                                 |
|---------------------------------------|-----------------------------------------|---------------------------------------------------------------------------|
| Keep PM-001 read APIs stable          | Replace read APIs with edit APIs        | Existing board and workspace behavior should not regress.                 |
| Add guarded write APIs                | Let frontend write files directly       | Backend guards are needed for path scope, Git state, and clear errors.    |
| Edit Markdown and metadata in PM-002  | Markdown only, metadata only            | A useful authoring MVP needs both content and board metadata changes.     |
| Treat docs roots as Markdown-only     | Force docs roots to use `plan.yaml`     | Freestyle docs should stay simple and should not need fake plan metadata. |
| Rescan after writes                   | Patch the in-memory index only          | A scan keeps fallback parsing, Git dates, authors, and warnings aligned.  |
| Guard and confirm risky Git actions   | Strict blocking, power-user passthrough | Users need useful Git operations without accidental data loss.            |
| Keep Git credential handling external | Store tokens in Plan Manager            | Local Git already owns credentials. The app should not store secrets.     |

## Implementation Clarifications

- PM-002 supports the full authoring MVP.
- It includes edit, status move, new plan, commit, pull, push, fetch, branch create, and branch switch.
- Write operations must stay inside the active repository and configured plan directories.
- File write requests use file IDs from the file tree or document list.
- Metadata writes update `plan.yaml` for structured plans.
- If a structured plan has no `plan.yaml`, status or metadata edit creates one.
- Freestyle docs roots support Markdown file editing but not structured plan metadata editing.
- Commit operations must commit only selected plan paths.
- Pull, push, and branch switch show confirmation when the working tree or branch state is risky.
- The app does not auto-fetch in PM-002.
- The app never stores Git credentials.
- After a successful write or Git content change, the app rescans the affected repository.
- The stale-content popup from PM-001 remains the cross-tab notification model.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
