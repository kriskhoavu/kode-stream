# Scenarios: PM-001 Overview

## Scenario List

| #   | Title               | Description                                                               |
|-----|---------------------|---------------------------------------------------------------------------|
| 0   | Empty app           | The app starts with no repositories registered                            |
| 1   | Register repository | The developer registers this repository with one or more plan directories |
| 2   | Scan content        | The app indexes structured plans and freestyle docs roots                 |
| 3   | Browse board        | The developer views and filters plans grouped by Kanban status            |
| 4   | Open workspace      | The developer opens a plan or docs root and reads its documents           |
| 5   | Use mobile board    | The developer views the board on a narrow viewport                        |
| 6   | Manage repository   | The developer edits or removes a registered repository                    |

---

# Scenario 0: Empty App

## Starting State

- The app has no registered repositories.
- The backend has no cached plan index.
- The frontend shows the app shell and an empty board state.

## Available Actions

| Action         | Description                     | Flow                                                          |
|----------------|---------------------------------|---------------------------------------------------------------|
| Add Repository | Register a local Git repository | User enters name, path, baseline branch, and plan directories |
| Browse Path    | Select a local path             | User opens the native directory picker                        |

## Expected Result

- The app does not scan automatically.
- The app does not run Git commands before a repository is registered.
- The UI still follows the shell in `specs/design.png`.

---

# Scenario 1: Register Repository

## Starting State

- The developer runs `plan-manager serve`.
- The browser opens the local app.
- This repository exists on disk.

## Flow

1. Developer opens Repositories.
2. Developer enters:
   - Name: `Plan Manager`
   - Path: current repository path
   - Baseline branch: `main`
   - Plan directories: `plans`, optionally `docs`
3. Backend validates:
   - `.git` exists.
   - `main` exists.
   - each configured plan directory exists.
4. Backend stores `RepositoryConfig` in the user data directory.

## Expected Result

- The repository appears in the repository list.
- The app shows a manual Scan action.
- Plan directories appear as badges in the repository form.
- The user can reveal the local path in the platform file manager.
- The repository working tree is not changed.

---

# Scenario 2: Scan Content

## Flow

1. Developer clicks Scan.
2. Backend reads local Git metadata.
3. Backend scans configured plan directories.
4. Backend parses `plan.yaml` when present.
5. Backend creates normal plan cards with inferred metadata when structured folders are missing `plan.yaml`.
6. Backend creates docs cards when a docs root contains Markdown without plan structure.
7. Backend writes only to the Plan Manager app cache.

## Expected Result

- Plans from `plans/api`, `plans/webapp`, `plans/gateway`, and `plans/platform` appear.
- `PM-001` appears under `platform`.
- `DI-202602` and `DI-430` appear in `In Progress`.
- Completed plans appear in `Done`.
- Docs roots appear as docs cards when configured.
- Plans without `plan.yaml` appear as normal plan cards.
- Unknown statuses map to `Draft`.

## Edge Cases

- Invalid YAML creates a scan warning and does not stop the scan.
- Missing README still creates a minimal plan card.
- Deleted folders disappear after the next scan.
- Duplicate ticket IDs stay unique by repository, branch, service, and path.

---

# Scenario 3: Browse Board

## Flow

1. Developer opens Kanban.
2. Frontend loads plan summaries.
3. Frontend renders columns:
   - Ideas
   - Draft
   - In Progress
   - Review
   - Done
4. Developer filters by repository, branch, status, and text.
5. Developer selects multiple statuses or authors in one filter.

## Expected Result

- Board layout follows `specs/design.png`.
- Cards show title, repository or service, branch, author when known, and updated time.
- Filters update the visible cards without a full page reload.
- Multiple selections in one filter are OR-matched.
- Different filter groups are AND-matched.
- Filter menus close when the user clicks outside them.
- Empty columns keep their header and count.

---

# Scenario 4: Open Workspace

## Flow

1. Developer opens a plan card.
2. Frontend loads plan detail.
3. Frontend renders:
   - Workspace header.
   - File tree.
   - Raw Markdown tab.
   - Preview tab.
   - Metadata sidebar.
   - Read-only diff tab.
4. Developer collapses, expands, and resizes the file explorer or plan info panel.

## Expected Result

- File tree rows use directory and file icons.
- File tree sorting is directory-first and natural alphabetical.
- Markdown tables, checklists, images, and Mermaid blocks render in preview.
- Docs roots show the appropriate metadata badge or callout.
- Raw Markdown is read-only in v1.
- Commit, pull, save, and new-plan actions are hidden or disabled in v1.
- The design stays close to the workspace section in `specs/design.png`.

---

# Scenario 5: Use Mobile Board

## Flow

1. Playwright MCP opens the app at a mobile viewport.
2. Developer views the board.
3. Developer opens a column and a card.

## Expected Result

- Mobile layout follows the right-side mobile mockup in `specs/design.png`.
- Cards are readable without horizontal scrolling.
- Bottom navigation is usable.
- Plan details remain reachable.

---

# Scenario 6: Manage Repository

## Flow

1. Developer opens Repositories.
2. Developer edits repository name, baseline branch, or plan directories.
3. Developer saves changes.
4. Developer deletes a disposable repository entry.

## Expected Result

- Editing updates the app-local registry.
- Deleting removes the repository from the UI.
- Cached plans for the deleted repository disappear from the board.
- No files in the managed repository are changed.
