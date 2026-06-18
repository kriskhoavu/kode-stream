# Frontend Design: PM-002

## Overview

PM-002 adds editing and Git controls to the existing React app.

The UI keeps the PM-001 shell and board shape. It adds clear write actions where users already work: the board, the preview drawer, the plan workspace, and the repository Git panel.

## Service Layer

Add API client methods:

| Method           | Endpoint                                   | Purpose                     |
|------------------|--------------------------------------------|-----------------------------|
| `saveFile`       | `POST /api/plans/{id}/files/{fileID}`      | Autosave Markdown content   |
| `updateMetadata` | `PATCH /api/plans/{id}/metadata`           | Save plan metadata          |
| `updateStatus`   | `PATCH /api/plans/{id}/status`             | Move Kanban status          |
| `createPlan`     | `POST /api/repositories/{id}/plans`        | Create a structured plan    |
| `sourceSettings` | `GET /api/repositories/{id}/source-settings?directory={dir}` | Load source structure settings |
| `saveSourceSettings` | `PUT /api/repositories/{id}/source-settings?directory={dir}` | Save source structure settings |
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
| Editor state       | Current file content, saved content, dirty flag, autosave state   |
| Metadata form      | Editable plan metadata and validation errors                      |
| Git status state   | Branch, dirty changes, conflicts, ahead/behind, current operation |
| New plan form      | Source root, service, ticket, title, owner, tags, and validation  |
| Source settings    | Selected repository/source root, path pattern, field mappings, warnings |
| Confirmation state | Risky operation dialog type, message, and confirm action          |

## Workspace And Drawer Editing

- The raw tab is editable in both the full details workspace and Kanban preview drawer.
- Preview keeps rendering the current editor content.
- Markdown changes autosave after a short pause and show pending/saving/saved/error state.
- Navigating away flushes pending Markdown changes instead of showing routine discard prompts.
- Metadata editor lives in the Work Item Info tab and keeps an explicit Save Metadata action.
- The Work Item Git tab exposes branch status, changed-file selection, commit, fetch, pull, push, and branch create controls.
- Freestyle docs show Markdown editing but hide metadata editing unless a valid source settings file maps them into configured cards.

## Kanban Editing

- Cards can move status through a menu or drag-and-drop.
- Status move calls the backend and waits for success.
- Failed status move restores the old column.
- Docs cards do not expose status move.
- `New Plan` opens a modal tied to the active repository and source root.
- Clicking a card opens a large resizable preview drawer.
- The preview drawer includes Preview, Raw, Diff, and a right-side Work Item panel.
- The Work Item panel hides at compact drawer widths and stretches full height when visible.

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
  -> WorkItemPanel
  -> ConfirmOperationDialog

KanbanPage
  -> StatusMoveControl
  -> NewPlanDialog
  -> PlanPreviewDrawer
     -> MarkdownEditor
     -> WorkItemPanel

App shell
  -> BranchSelector
  -> GitOperationMenu
```

## UX Rules

- Autosave Markdown file edits after a short pause.
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
| Autosave Markdown edits    | File edits are frequent and Git is the durable review boundary. |
| Use guarded confirmations  | Risky Git actions need friction without blocking normal edits.  |
| Commit selected paths only | Repositories may contain unrelated local work.                  |
| Keep plain docs metadata hidden | Freestyle docs do not have structured plan fields unless source settings provide a card mapping. |
