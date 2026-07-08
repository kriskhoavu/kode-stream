# PM-002: Plan Editing And Git Operations

## Overview

PM-002 turns Kode Stream from a read-only browser into a safe local authoring tool.

Users can edit Markdown files, update item metadata, create new items, move cards across the Kanban board, and run guarded Git operations from the app. The app still runs locally. It still writes only to registered workspaces that the user selected.

## Related Plans

| Item                          | Relationship   | Key Context                                                                                  |
|-------------------------------|----------------|----------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature | PM-001 created the read-only registry, scanner, plan index, Kanban board, and plan workspace |

### What PM-001 Established

- **Workspace**: a local Git repository registered in Kode Stream.
- **Source**: a configured scan root such as `plans` or `docs`.
- **Item**: a planning folder or docs item shown on the board and in the workspace.
- **Freestyle Docs Root**: a Markdown docs folder that does not use the source/item shape.
- **Read-only boundary**: all PM-001 plan and Git APIs only read target repositories.
- **File access guard**: all file reads must stay inside configured plan directories.
- **App state version**: registry or index changes update `/api/state`.

## Glossary

| Term             | Meaning                                                                              | Maps To (code)              |
|------------------|--------------------------------------------------------------------------------------|-----------------------------|
| Workspace        | A local Git repository registered in Kode Stream                                    | `RepositoryConfig`          |
| Source           | A configured scan root such as `plans`, `docs`, or `docs/plans`                      | `planDirectories`           |
| Source Items | Optional `repository-settings.yaml` in a source that maps arbitrary folders to cards | `RepositorySettings`        |
| Item             | A planning folder or docs item                                                       | `PlanSummary`, `PlanDetail` |
| Item Metadata    | Machine-readable item fields stored in `plan.yaml`                                   | `PlanMetadataUpdateInput`   |
| Edit Session     | The frontend state for one open file or metadata form with unsaved changes           | editor state                |
| Dirty State      | Local Git state with modified, staged, untracked, or conflicting files               | `GitStatus`                 |
| Write Guard      | Backend checks that block unsafe file writes and risky Git operations                | file writer, Git adapter    |
| Git Operation    | A guarded local Git action such as fetch, pull, push, commit, or switch              | `GitOperationResult`        |
| Commit Draft     | User-entered commit message and selected item paths                                  | `GitCommitInput`            |
| Branch Operation | Branch create or branch switch from the active repository                            | branch request models       |

## Components

| Layer    | Component        | Purpose                                                                              |
|----------|------------------|--------------------------------------------------------------------------------------|
| Backend  | Safe file writer | Writes editable files only inside configured sources                                 |
| Backend  | Metadata writer  | Creates and updates `plan.yaml` without changing unrelated fields                    |
| Backend  | Source settings  | Reads and writes `repository-settings.yaml`, then rescans configured sources         |
| Backend  | Plan creator     | Creates a structured item folder with starter documents                              |
| Backend  | Git adapter      | Runs guarded Git write operations with clear status and errors                       |
| Backend  | HTTP API         | Exposes item edit, status move, new item, and Git operation endpoints                |
| Frontend | Editor state     | Tracks selected file, content, dirty state, autosave state, and conflict warnings    |
| Frontend | Workspace editor | Adds Markdown editing, preview, autosave, metadata editing, and Work Item controls   |
| Frontend | Kanban actions   | Moves status and opens new-item flows from the board                                 |
| Frontend | Git controls     | Shows branch and dirty state, then runs guarded Git operations in details and drawer |

## Data Flow

```text
User opens a plan
  -> frontend loads plan detail, files, file content, diff, and Git status
  -> user edits Markdown or metadata
  -> frontend autosaves Markdown after a short pause
  -> user explicitly saves metadata when metadata fields change
  -> backend validates repository, plan ID, file ID, and path scope
  -> backend writes the file or plan.yaml
  -> file saves return the updated file hash immediately
  -> metadata/status/new-plan writes refresh the affected repository index
  -> frontend refreshes diff, Git status, board, workspace, and stale-content state as needed

User runs a Git operation
  -> frontend requests Git status
  -> backend reports branch, dirty files, staged files, and divergence
  -> frontend asks for confirmation when the operation is risky
  -> backend runs the guarded Git command
  -> backend rescans when repository content changed
  -> frontend shows the result and refreshed status
```

## Design Decisions

| Decision                                     | Alternatives Considered                 | Rationale                                                                                                                                                 |
|----------------------------------------------|-----------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| Keep PM-001 read APIs stable                 | Replace read APIs with edit APIs        | Existing board and workspace behavior should not regress.                                                                                                 |
| Add guarded write APIs                       | Let frontend write files directly       | Backend guards are needed for path scope, Git state, and clear errors.                                                                                    |
| Edit Markdown and metadata in PM-002         | Markdown only, metadata only            | A useful authoring MVP needs both content and board metadata changes.                                                                                     |
| Keep freestyle docs simple, but configurable | Force all docs roots to use `plan.yaml` | Plain docs should still work as one card. When users want cards, `repository-settings.yaml` describes the source layout.                                  |
| Rescan after metadata/status/Git writes      | Patch the in-memory index only          | A scan keeps fallback parsing, Git dates, authors, and warnings aligned. File autosave keeps the editor fast by returning the updated file/hash directly. |
| Guard and confirm risky Git actions          | Strict blocking, power-user passthrough | Users need useful Git operations without accidental data loss.                                                                                            |
| Keep Git credential handling external        | Store tokens in Kode Stream            | Local Git already owns credentials. The app should not store secrets.                                                                                     |

## Implementation Clarifications

- PM-002 supports the full authoring MVP.
- It includes edit, status move, new plan, commit, pull, push, fetch, branch create, and branch switch.
- Markdown file edits autosave in both the plan workspace and Kanban preview drawer.
- The Kanban preview drawer exposes the same Work Item Info/Git editing surface as the details view.
- Write operations must stay inside the active repository and configured plan directories.
- File write requests use file IDs from the file tree or document list.
- Metadata writes update `plan.yaml` for structured items.
- If a structured plan has no `plan.yaml`, status or metadata edit creates one.
- Source directories can opt into structured card discovery with `repository-settings.yaml`.
- Configured source cards support metadata editing; the first metadata save creates `plan.yaml` in the matched card folder.
- Freestyle docs roots without source settings appear in the `Unsorted` lane and support Markdown file editing but not structured plan metadata editing.
- The Kanban board separates `Unsorted` from workflow statuses with a compact action rail that points users to source items configuration.
- Commit operations must commit only selected plan paths.
- Pull, push, and branch switch show confirmation when the working tree or branch state is risky.
- The app does not auto-fetch in PM-002.
- The app never stores Git credentials.
- File autosave returns the updated `FileContent` and hash without a full rescan.
- Metadata, status, new-plan, pull, commit, and branch operations refresh the affected repository state.
- The stale-content popup from PM-001 remains the cross-tab notification model.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
