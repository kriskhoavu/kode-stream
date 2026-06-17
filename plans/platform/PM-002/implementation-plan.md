# Implementation Plan: PM-002 - Plan Editing And Git Operations

## Overview

Implement safe plan authoring and guarded Git operations for the local Plan Manager app.

PM-002 builds on PM-001. It keeps existing read APIs stable and adds write APIs, editor UI, metadata editing, new plan creation, status moves, and Git controls.

## Terminology Lock

All code, API fields, and UI labels must use:

- `Repository`
- `Plan`
- `Plan Directory`
- `Plan Metadata`
- `Edit Session`
- `Dirty State`
- `Write Guard`
- `Git Operation`
- `Commit Draft`
- `Branch Operation`

Avoid:

- `Project` for registered repositories.
- `Task` for plans.
- `Sync` when the operation is specifically `Scan`, `Fetch`, `Pull`, or `Push`.
- `Auto Save` because PM-002 saves only when the user clicks Save.

## Implementation Clarifications

- PM-002 is a full authoring MVP.
- It supports Markdown editing, metadata editing, status moves, new plan creation, commit, pull, push, fetch, branch create, and branch switch.
- Write APIs must validate repository scope, plan scope, file scope, and symlink safety.
- Freestyle docs roots support Markdown editing only.
- Structured plans support Markdown and metadata editing.
- Structured plans without `plan.yaml` can get a generated `plan.yaml` when metadata or status changes.
- Commit operations stage and commit only selected plan paths.
- Pull, push, and branch switch use guard-and-confirm behavior.
- The app does not add background auto-fetch.
- The app never stores Git credentials.
- After successful content writes, the affected repository is rescanned.

## Backend Phases

### Phase B1: Write-Safe Domain And Models

**Deliverables:**

- [x] Add request and response models for file save, metadata save, status update, new plan, Git status, commit, and branch operations.
- [x] Add Git change models for dirty, staged, untracked, conflicted, ahead, behind, and upstream state.
- [x] Add validation helpers for editable status values, branch names, commit messages, service names, and ticket names.
- [x] Add tests for validation rules.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-002: Add write-safe models

- Add plan edit and Git operation request models
- Add Git status response models
- Add validation helpers for write operations
```

---

### Phase B2: File And Metadata Write Services

**Deliverables:**

- [x] Add safe file writer that resolves file IDs through the plan file tree or document list.
- [x] Add Markdown file save with expected hash support.
- [x] Add metadata writer for `plan.yaml`.
- [x] Add status update writer for Kanban moves.
- [x] Add structured plan creator with starter `README.md`, scenario folder, design folder, implementation plan, and `plan.yaml`.
- [x] Rescan the affected repository after successful writes.
- [x] Add tests for path traversal, symlink escape, docs root behavior, metadata creation, and duplicate plan creation.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-002: Add safe plan write services

- Add guarded Markdown file saves
- Add plan metadata and status writers
- Add structured plan creation
```

---

### Phase B3: Git Operation APIs

**Deliverables:**

- [x] Extend Git adapter with status, fetch, pull, push, commit, branch create, and branch switch.
- [x] Add write guards for conflicts, dirty state, divergence, and selected path scope.
- [x] Add repository Git API endpoints.
- [x] Add plan file, metadata, status, and new plan API endpoints.
- [x] Return clear operation results with updated Git status.
- [x] Add tests for guarded operation behavior and API errors.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-002: Add guarded Git APIs

- Add Git status and operation endpoints
- Add guarded commit, pull, push, and branch operations
- Add plan write API routes
```

---

### Phase B4: Index Refresh And Stale-State Integration

**Deliverables:**

- [x] Ensure successful writes update the plan index and app state version.
- [x] Ensure Git content changes rescan the affected repository.
- [x] Ensure failed writes do not mutate the index.
- [x] Keep PM-001 read APIs stable.
- [x] Add tests for app state version changes after writes.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-002: Refresh index after writes

- Rescan repositories after content changes
- Update app state version for write operations
- Preserve existing read API behavior
```

---

## Frontend Phases

### Phase F1: API Types And Client Methods

**Deliverables:**

- [x] Add frontend types for edit inputs, Git status, Git changes, Git operation results, and branch operations.
- [x] Add API client methods for file save, metadata save, status update, new plan, Git status, fetch, pull, push, commit, branch create, and branch switch.
- [x] Normalize optional response fields.
- [x] Add focused API client tests where current test setup supports them.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-002: Add frontend write API client

- Add plan edit and Git operation types
- Add API client methods for write operations
- Normalize Git and edit responses
```

---

### Phase F2: Edit State And Dirty-State Handling

**Deliverables:**

- [x] Add editor state for current content, saved content, dirty flag, saving state, and errors.
- [x] Add metadata form state and validation.
- [x] Add Git status state and operation state.
- [x] Add navigation guard for unsaved edits.
- [x] Integrate stale-content popup with edit sessions.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-002: Add edit and Git state handling

- Track unsaved file and metadata changes
- Add Git status state
- Add navigation guards for dirty edit sessions
```

---

### Phase F3: Editor, Metadata, New Plan, And Git UI

**Deliverables:**

- [x] Add Markdown editor mode to the workspace raw tab.
- [x] Keep preview rendering the current editor content.
- [x] Add metadata editor for structured plans.
- [x] Add Kanban status move controls.
- [x] Add new plan dialog.
- [x] Add Git status panel with changed-file selection and commit form.
- [x] Add fetch, pull, push, branch create, and branch switch controls.
- [x] Add confirmation dialogs for risky operations.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-002: Add plan editing UI

- Add Markdown and metadata editing
- Add new plan and status move flows
- Add Git status and operation controls
```

---

### Phase F4: Styling, Responsive Checks, And App Verification

**Deliverables:**

- [ ] Style editor, metadata, Git panel, dialogs, and new plan form to match the PM-001 shell.
- [ ] Verify desktop and mobile layouts.
- [ ] Verify no write control overlaps in narrow viewports.
- [ ] Rebuild frontend assets.
- [ ] Rebuild Go binary.
- [ ] Restart the app on port `4317`.
- [ ] Smoke test the running app.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go build -o ./bin/plan-manager ./cmd/plan-manager`

**Draft Commit:**
```text
PM-002: Polish editing workflow

- Style editing and Git operation surfaces
- Verify responsive layouts
- Rebuild and run the app
```
