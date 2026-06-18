# Scenarios: PM-002 Overview

## Scenario List

| #   | Title                    | Description                                               |
|-----|--------------------------|-----------------------------------------------------------|
| 0   | Read-only baseline       | PM-001 behavior before PM-002 starts                      |
| 1   | Edit Markdown file       | User edits a plan document and autosave writes it         |
| 2   | Edit plan metadata       | User updates title, status, owner, tags, and documents    |
| 3   | Move Kanban status       | User moves a plan card to another status column           |
| 4   | Create new plan          | User creates a structured plan from the active workspace  |
| 5   | Commit local changes     | User commits selected plan changes                        |
| 6   | Pull and push            | User syncs with the remote using guarded operations       |
| 7   | Create and switch branch | User creates or switches branches with dirty-state checks |
| 8   | Handle risky state       | User sees warnings and confirmations before risky writes  |

---

# Scenario 0: Read-only Baseline

## Starting State

- A repository is registered.
- One or more plan directories are configured.
- The board and workspace can read plan data.
- Pull is disabled.
- No file, metadata, or Git write API exists.

## Visual State

```text
Repository
  -> Plan directories
  -> Scanner
  -> Plan index
  -> Kanban board
  -> Read-only workspace
```

## Available Actions

| Action       | Result                                   |
|--------------|------------------------------------------|
| Scan         | Rebuilds cached plan metadata            |
| Open plan    | Shows file tree, preview, raw, and diff  |
| Open card    | Shows preview drawer                     |
| Filter board | Filters cached summaries in the frontend |

---

# Scenario 1: Edit Markdown File

## Goal

The user edits a Markdown file and saves it to the local repository.

## Flow

```text
User opens a plan file
  -> frontend loads file content
  -> user edits raw Markdown in the details workspace or preview drawer
  -> frontend tracks pending autosave state
  -> frontend autosaves after a short pause
  -> POST /api/plans/{id}/files/{fileID}
  -> backend validates file scope
  -> backend writes the file
  -> backend returns updated file content and hash
  -> frontend refreshes diff and Git status
```

## Edge Cases

- If the file no longer exists, show an error and reload the file tree.
- If the file path escapes the plan root, reject the request.
- If the user closes or navigates before the timer fires, flush the pending autosave first.
- If another tab changed the plan, show the stale-content popup.

---

# Scenario 2: Edit Plan Metadata

## Goal

The user edits structured plan metadata without manually editing `plan.yaml`.

## Flow

```text
User opens metadata editor
  -> frontend shows Work Item Info fields in details or preview drawer
  -> user changes fields
  -> user clicks Save Metadata
  -> PATCH /api/plans/{id}/metadata
  -> backend validates structured plan scope
  -> backend updates or creates plan.yaml
  -> backend rescans the repository
  -> frontend reloads plan detail and board data
```

## Edge Cases

- Freestyle docs show a Markdown-only message.
- Invalid status values are rejected.
- Empty title is rejected.
- Existing unknown `plan.yaml` fields are preserved where possible.

---

# Scenario 3: Move Kanban Status

## Goal

The user moves a card between columns and the plan status changes.

## Flow

```text
User drags or chooses status
  -> frontend sends PATCH /api/plans/{id}/status
  -> backend writes status to plan.yaml
  -> backend rescans
  -> frontend updates board columns
```

## Edge Cases

- If the plan has no `plan.yaml`, the backend creates one with required fields.
- If the plan is a docs item, status move is disabled.
- Failed saves restore the card to its previous column.

---

# Scenario 4: Create New Plan

## Goal

The user creates a new structured plan in a configured plan directory.

## Flow

```text
User clicks New Plan
  -> frontend asks for source, service, ticket, title, branch, and template
  -> POST /api/repositories/{id}/plans
  -> backend validates the target root
  -> backend creates folder and starter files
  -> backend rescans
  -> frontend opens the new plan
```

## Edge Cases

- Duplicate ticket folders are rejected.
- Only structured plan directories can create structured plans.
- Service and ticket names must produce relative paths.

---

# Scenario 5: Commit Local Changes

## Goal

The user commits selected plan changes from the app.

## Flow

```text
User opens Git panel
  -> frontend loads Git status
  -> user selects changed plan files
  -> user enters commit message
  -> POST /api/repositories/{id}/git/commit
  -> backend stages selected paths
  -> backend commits with the provided message
  -> backend rescans
  -> frontend reloads Git status and diff
```

## Edge Cases

- Empty commit message is rejected.
- Empty path selection is rejected.
- Commit only stages selected plan paths.
- Conflicted files block commit.

---

# Scenario 6: Pull And Push

## Goal

The user syncs the active branch with the remote.

## Flow

```text
User clicks Pull or Push
  -> frontend requests Git status
  -> frontend sends confirmation payload when dirty or divergent state requires it
  -> POST /api/repositories/{id}/git/pull or /git/push
  -> backend runs Git
  -> backend returns stdout, stderr summary, and updated status
  -> backend rescans when content changed
```

## Edge Cases

- Missing upstream returns a clear error.
- Merge conflict returns a conflict state and stops follow-up operations.
- Push rejection returns the remote message.

---

# Scenario 7: Create And Switch Branch

## Goal

The user creates or switches a branch from the active workspace.

## Flow

```text
User opens branch menu
  -> frontend loads branches and Git status
  -> user creates or selects a branch
  -> backend validates branch name and dirty state
  -> backend runs git branch or git switch
  -> backend rescans the repository
  -> frontend reloads repository plans
```

## Edge Cases

- Invalid branch names are rejected.
- Existing branch names cannot be created twice.
- Dirty state requires confirmation before switch.

---

# Scenario 8: Handle Risky State

## Goal

The user is protected from accidental data loss.

## Flow

```text
Backend detects dirty or conflicting state
  -> API returns a guarded error or warning
  -> frontend shows a focused confirmation dialog
  -> user cancels or confirms
  -> backend only runs risky operation when confirmation is explicit
```

## Edge Cases

- Destructive actions are not added in PM-002.
- The app never runs force push.
- The app never discards local changes.
