# Frontend Design: PM-002

## Overview

PM-002 adds editing and Git controls to the existing React app.

The UI keeps the PM-001 shell and board shape. It adds clear write actions where users already work: the board, the preview drawer, the plan workspace, and the repository Git panel.

## Service Layer

Add API client methods:

| Method           | Endpoint                                   | Purpose                     |
|------------------|--------------------------------------------|-----------------------------|
| `saveFile`       | `PUT /api/plans/{id}/files/{fileID}`       | Save Markdown content       |
| `updateMetadata` | `PATCH /api/plans/{id}/metadata`           | Save plan metadata          |
| `updateStatus`   | `PATCH /api/plans/{id}/status`             | Move Kanban status          |
| `createPlan`     | `POST /api/repositories/{id}/plans`        | Create a structured plan    |
| `gitStatus`      | `GET /api/repositories/{id}/git/status`    | Load dirty and branch state |
| `gitFetch`       | `POST /api/repositories/{id}/git/fetch`    | Fetch refs                  |
| `gitPull`        | `POST /api/repositories/{id}/git/pull`     | Pull guarded changes        |
| `gitPush`        | `POST /api/repositories/{id}/git/push`     | Push guarded changes        |
| `gitCommit`      | `POST /api/repositories/{id}/git/commit`   | Commit selected paths       |
| `createBranch`   | `POST /api/repositories/{id}/git/branches` | Create branch               |
| `switchBranch`   | `POST /api/repositories/{id}/git/switch`   | Switch branch               |

## State

| State Area         | Responsibility                                                    |
|--------------------|-------------------------------------------------------------------|
| Editor state       | Current file content, saved content, dirty flag, save state       |
| Metadata form      | Editable plan metadata and validation errors                      |
| Git status state   | Branch, dirty changes, conflicts, ahead/behind, current operation |
| New plan form      | Source root, service, ticket, title, owner, tags, and validation  |
| Confirmation state | Risky operation dialog type, message, and confirm action          |

## Workspace Editing

- The raw tab becomes an editor when the user clicks Edit.
- Preview keeps rendering the current editor content.
- Save writes the current file.
- Cancel restores the last saved content.
- Unsaved changes show a clear dirty marker.
- Navigating away with unsaved changes shows a browser-level and in-app confirmation.
- Metadata editor lives in the right panel or a focused modal.
- Freestyle docs show Markdown editing but hide metadata editing.

## Kanban Editing

- Cards can move status through a menu or drag-and-drop.
- Status move calls the backend and waits for success.
- Failed status move restores the old column.
- Docs cards do not expose status move.
- `New Plan` opens a modal tied to the active repository and source root.

## Git Controls

- Show current branch in the workspace header and repository shell.
- Show dirty state in a compact Git panel.
- Show changed files with checkboxes for commit selection.
- Commit requires a non-empty message and at least one selected path.
- Pull, push, and branch switch show confirmation when backend status marks the action risky.
- Operation results appear as concise success or error messages.

## Components

```text
PlanWorkspacePage
  -> MarkdownEditor
  -> MetadataEditor
  -> GitStatusPanel
  -> ConfirmOperationDialog

KanbanPage
  -> StatusMoveControl
  -> NewPlanDialog

App shell
  -> BranchSelector
  -> GitOperationMenu
```

## UX Rules

- Do not auto-save.
- Do not auto-fetch.
- Do not hide dirty state.
- Disable write controls while a save or Git operation is running.
- Keep read-only labels only when the app is in read-only state or the item cannot be edited.
- Use the existing stale-content popup for cross-tab changes.
- Keep actions close to the content they affect.

## Design Decisions

| Decision                   | Rationale                                                       |
|----------------------------|-----------------------------------------------------------------|
| Edit in the workspace      | Users already inspect files there, so editing stays contextual. |
| Keep preview live          | Markdown authors need fast feedback.                            |
| Use guarded confirmations  | Risky Git actions need friction without blocking normal edits.  |
| Commit selected paths only | Repositories may contain unrelated local work.                  |
| Keep docs metadata hidden  | Freestyle docs do not have structured plan fields.              |
