# Implementation Plan: PM-001 - Plan Manager Read-Only MVP

## Overview

Implement a local read-only Plan Manager app.

The MVP registers local Git repositories, scans one or more configured plan or documentation roots, renders plans and docs on a Kanban board, and opens a read-only workspace for plan files. It follows `specs/requirement.md` for behavior and `specs/design.png` for the visual baseline.

## Terminology Lock

All code, API fields, and UI labels must use:

- `Repository`
- `RepositoryConfig`
- `Plan`
- `PlanSummary`
- `PlanDetail`
- `PlanDocument`
- `PlanStatus`
- `Plan Directory`
- `Structured Plan Root`
- `Freestyle Docs Root`
- `Scan`
- `Workspace`

Avoid:

- `Project` for registered Git repositories.
- `Task` for plans.
- `Sync` for read-only scan unless Git fetch is added later.

## Implementation Clarifications

- Support at least 100 repositories, 10,000 plans, and 100,000 files through cached metadata.
- Board and list views must use cached plan summaries.
- File content must load only when a user opens a plan file.
- Manual Scan rebuilds derived metadata for one repository.
- A repository can have multiple plan directories, such as `plans`, `docs`, or `docs/plans`.
- A structured plan root uses the `service/ticket` folder shape and usually contains `plan.yaml`.
- A structured plan folder that is missing `plan.yaml` still appears as a normal plan card with inferred metadata.
- A freestyle docs root is indexed as a docs item even when it has Markdown files but no plan folder structure.
- A bad plan creates a scan warning and must not fail the whole scan.
- Keep backend boundaries between `RepositoryRegistry`, `GitAdapter`, `PlanScanner`, `PlanIndex`, `FileAccess`, and `PlanAPI`.
- HTTP handlers must not read arbitrary filesystem paths directly.
- File reads must stay inside configured plan directories.
- PM-001 must not expose Git or file write operations.
- Repository create, edit, delete, path browse, and path reveal actions write only app registry or cache data, not managed repositories.
- Kanban filters are multi-select: options are OR-matched within a filter group and AND-matched across groups.
- UI parity means matching the proposal layout, density, navigation, and mobile behavior. Pixel-perfect matching is not required.

## Backend Phases

### Phase B1: App Skeleton And Repository Registry

**Deliverables:**

- [x] Go module and `cmd/plan-manager` entrypoint.
- [x] `plan-manager serve` command.
- [x] Local config path in OS user data directory.
- [x] Repository registration model and validation.
- [x] Repository list and create API endpoints.
- [x] Repository update and delete API endpoints.
- [x] Native directory selection and path reveal API endpoints.

**Verification:** `go test ./...`

**Draft Commit:**
```text
PM-001: Add local app skeleton and repository registry

- Add Go CLI entrypoint for Plan Manager
- Add repository config storage
- Add repository validation API
```

---

### Phase B2: Read-Only Plan Scanner

**Deliverables:**

- [x] Git adapter for read-only commands.
- [x] Plan scanner for configured plan directories.
- [x] Multiple plan directories per repository.
- [x] `plan.yaml` parser.
- [x] Fallback parser for folder and README metadata.
- [x] Fallback discovery for structured folders without `plan.yaml`.
- [x] Freestyle docs discovery for docs roots.
- [x] Status normalization.
- [x] Scan result warnings.

**Verification:** `go test ./...`

**Draft Commit:**
```text
PM-001: Add read-only plan scanner

- Add metadata-driven plan parsing
- Add fallback plan discovery
- Add status normalization and scan warnings
```

---

### Phase B3: Plan API And Cache

**Deliverables:**

- [x] Plan index cache.
- [x] Plan list API with repository, branch, status, and text filters.
- [x] Plan detail API.
- [x] File tree API.
- [x] File content API.
- [x] Read-only diff API.
- [x] Repository cache deletion when a repository is removed.
- [x] Empty docs root handling in detail APIs.

**Verification:** `go test ./...`

**Draft Commit:**
```text
PM-001: Add plan index and read APIs

- Cache plan summaries and document metadata
- Serve plan details and files
- Serve read-only Git diffs
```

---

## Frontend Phases

### Phase F1: Frontend App Shell And API Client

**Deliverables:**

- [x] React/Vite app setup.
- [x] API client types for repositories, plans, files, and scans.
- [x] App shell with top bar, left nav, repository tabs, search, and theme toggle.
- [x] Repository registration screen.
- [x] Repository edit and delete controls.
- [x] Native path browse, reveal, and drag-and-drop path support.
- [x] Plan directory chips in the repository form.

**Verification:** `npm run typecheck && npm test`

**Draft Commit:**
```text
PM-001: Add frontend shell and API client

- Add React app structure
- Add API client types
- Add repository registration UI
```

---

### Phase F2: Kanban Board

**Deliverables:**

- [x] Scalable board toolbar with repository, branch, status, author, and text filters.
- [x] Multi-select filter popovers with OR matching within each facet.
- [x] Selected filter chips and clear actions.
- [x] Filter chevrons and outside-click dismissal.
- [x] Five Kanban columns.
- [x] Plan cards with title, service, branch, author, tags, and updated time.
- [x] Empty, loading, and error states.
- [x] Desktop layout matching `specs/design.png`.

**Verification:** `npm run typecheck && npm test`

**Playwright MCP:** Verify board rendering and filters on desktop.

**Draft Commit:**
```text
PM-001: Add read-only Kanban board

- Add status columns and plan cards
- Add board filters
- Match the desktop board design
```

---

### Phase F3: Plan Workspace

**Deliverables:**

- [x] Workspace route.
- [x] Workspace header.
- [x] Directory-first natural-sorted file tree.
- [x] File and directory icons in the file tree.
- [x] Collapsible and resizable file explorer panel.
- [x] Raw Markdown tab.
- [x] Markdown preview tab.
- [x] Metadata sidebar.
- [x] Collapsible and resizable plan info panel.
- [x] Docs root metadata callouts.
- [x] Read-only diff tab.

**Verification:** `npm run typecheck && npm test`

**Playwright MCP:** Open `PM-001` and verify file tree, raw Markdown, preview, metadata, and diff.

**Draft Commit:**
```text
PM-001: Add read-only plan workspace

- Add plan detail layout
- Add file tree and Markdown preview
- Add metadata and diff panels
```

---

### Phase F4: Responsive Styling And Visual Parity

**Deliverables:**

- [x] Mobile board layout matching `specs/design.png`.
- [x] Responsive workspace layout.
- [x] Responsive repository management layout for large repository lists.
- [x] Responsive Kanban filter controls for large option sets.
- [x] Light and dark theme behavior.
- [x] Disabled or hidden write actions for v1.
- [ ] Screenshot verification artifacts from Playwright MCP.

**Verification:** `npm run typecheck && npm test`

**Playwright MCP:** Capture desktop and mobile screenshots and compare to `specs/design.png`.

**Draft Commit:**
```text
PM-001: Add responsive visual parity

- Match desktop and mobile proposal layouts
- Add theme behavior
- Verify the UI with Playwright MCP screenshots
```

---

## DevOps Phases

### Phase C1: Embedded Build And Local Binary

**Deliverables:**

- [x] Frontend production build.
- [x] Go binary embedding frontend assets.
- [x] Configurable localhost port.
- [x] Startup output with local URL.
- [ ] App smoke test against the built binary.

**Verification:** `npm run build && go build ./cmd/plan-manager`

**Draft Commit:**
```text
PM-001: Add embedded local app build

- Build frontend assets
- Embed assets in the Go binary
- Serve the app from localhost
```

---

### Phase C2: Verification And Release Preparation

**Deliverables:**

- [x] Document local build commands.
- [x] Add Playwright MCP acceptance checklist.
- [x] Add release notes for future Homebrew packaging.
- [ ] Confirm managed repositories stay unchanged after scan.

**Verification:** `go test ./... && npm run typecheck && npm test && npm run build`

**Playwright MCP:** Run full acceptance flow from repository registration to mobile screenshot.

**Draft Commit:**
```text
PM-001: Add verification and release preparation

- Document local verification commands
- Add Playwright MCP acceptance checklist
- Prepare Homebrew release notes
```

---

## Post-Implementation Checklist

- [x] Update `plans/platform/PM-001/` docs to reflect final naming.
- [x] Confirm `specs/design.png` remains the visual baseline.
- [x] Confirm no Git write operations are available in v1.
- [ ] Confirm Playwright MCP screenshots were captured for desktop and mobile.
- [ ] Create the follow-up plan `PM-002: Plan Editing And Git Operations`.
- [ ] PR description references this plan.
