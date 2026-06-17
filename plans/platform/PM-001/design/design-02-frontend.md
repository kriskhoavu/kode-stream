# Frontend Design: PM-001

## Goals

- Match the product proposal in `specs/design.png`.
- Provide a fast Kanban board for the active workspace.
- Provide a read-only plan workspace.
- Provide repository management for register, edit, delete, scan, browse, and reveal actions.
- Support structured plans and freestyle docs roots.
- Support desktop and mobile layouts.
- Keep all write actions disabled or hidden in v1.

## UX Acceptance

- The UI should match the layout, density, navigation, and mobile behavior of `specs/design.png`.
- Pixel-perfect parity is not required for PM-001.
- The board must feel dense and operational, not like a marketing page.
- Desktop board columns should remain stable while filters and loading states change.
- Mobile cards must be readable without horizontal scrolling.
- Text must not overflow cards, filters, buttons, tabs, or side panels.
- Write actions from the design must be hidden or disabled in PM-001.

## Visual Source Of Truth

| Source                 | Required Use                                                                                            |
|------------------------|---------------------------------------------------------------------------------------------------------|
| `specs/design.png`     | Desktop shell, Kanban board, workspace layout, mobile board, spacing, density, and light/dark direction |
| `specs/requirement.md` | Feature behavior and data requirements                                                                  |

## App Structure

```text
App
  AppShell
    TopBar
    LeftNav
    WorkspaceSelector
    MainContent
  KanbanPage
    BoardToolbar
    FilterPopover
    KanbanColumn
    PlanCard
  PlanWorkspacePage
    WorkspaceHeader
    FileTree
    MarkdownRawView
    MarkdownPreview
    MetadataPanel
    DiffPanel
  RepositoriesPage
    RepositoryForm
    PlanDirectoryInput
    RepositoryList
    RepositoryActions
```

## Routes

| Route            | Purpose                                |
|------------------|----------------------------------------|
| `/`              | Redirect to Kanban                     |
| `/kanban`        | Board view                             |
| `/plans/:planId` | Plan workspace                         |
| `/repositories`  | Repository registration and management |

## UI States

| Area       | State          | Behavior                                                    |
|------------|----------------|-------------------------------------------------------------|
| Board      | Loading        | Show stable column skeletons                                |
| Board      | Empty          | Show empty-state action to add repository                   |
| Board      | Loaded         | Show five columns and counts                                |
| Board      | Filtered empty | Keep filters visible and show no-results text               |
| Workspace  | Loading        | Keep shell stable and load panels independently             |
| Workspace  | File missing   | Show file-level error                                       |
| Workspace  | Docs root      | Show docs metadata and an empty/file-focused reading state  |
| Repository | Invalid        | Show validation errors from backend                         |
| Repository | Editing        | Preserve current values and allow cancel/save               |
| Repository | Delete confirm | Require explicit delete confirmation before removing a repo |

## Board Behavior

- Render columns in this order:
  - Ideas
  - Draft
  - In Progress
  - Review
  - Done
- Use compact cards like the design.
- Show title, repository or service, branch, author when known, tags, and updated time.
- Scope the board to the active workspace selected from the left navigation.
- Use branch, status, author, and text filters.
- Allow multiple selected options in each filter group.
- Match selected options as OR within a filter group and AND across filter groups.
- Use searchable popovers for long option lists.
- Show selected filter chips below the toolbar.
- Close filter popovers when the user clicks outside them.
- Show a small down icon on each filter button.
- Do not enable drag-and-drop status moves in v1.
- Keep column widths stable on desktop.
- Use cached plan summaries from the backend.
- Do not request file contents for board cards.

## Workspace Behavior

- File tree is sorted by directory first, then filename.
- File tree uses natural alphabetical sorting, such as `design-2.md` before `design-10.md`.
- `plan.yaml` document order is ignored for the file explorer.
- File tree rows use directory and file icons.
- File tree spacing must keep filenames visually separated and readable.
- File explorer and plan info panels are collapsible.
- File explorer and plan info panels are resizable with smooth transitions.
- Raw Markdown tab is read-only.
- Preview renders:
  - headings.
  - tables.
  - checklists.
  - images with relative paths.
  - Mermaid blocks when supported.
- Diff tab shows read-only added, changed, and deleted lines.
- Docs roots show a docs-oriented metadata callout.
- Empty docs roots show an empty state instead of a blank page.
- Commit, pull, save, and new-plan actions are hidden or disabled in v1.
- Load file content only when the user opens a file.

## Repository Behavior

- Register repositories with one or more plan directories.
- Treat each registered repository as a workspace.
- Switch active workspace from the left navigation instead of mixing repositories on one board.
- Support structured roots such as `plans` and docs roots such as `docs`.
- Display selected plan directories as badges in the input area.
- Provide quick-add directory chips for common roots such as `plans` and `docs`.
- Allow native path browsing for repository paths and plan directories.
- Allow dragging a local path or file URL into the repository path field.
- Allow revealing repository paths in Finder, Windows Explorer, or the platform file manager.
- Allow editing a registered repository.
- Allow deleting a registered repository and clearing its cached plans from the UI.
- Keep the repository management layout usable with large repository lists.

## Mobile Behavior

- Use the mobile board pattern from `specs/design.png`.
- Keep cards readable in a single column.
- Use bottom navigation for Kanban, Plans, Branches, and Repos.
- Keep filters reachable without covering cards.
- Collapse large filter sets into compact controls that do not overflow.

## Design Constraints

- Use lucide icons for navigation and action buttons.
- Use cards only for plan cards and repeated items.
- Do not put cards inside cards.
- Do not use decorative gradient blobs or orbs.
- Keep text inside buttons and cards from overflowing.
- Use stable dimensions for board columns, icon buttons, and cards.
- Preserve the dense operational feel from the proposal design.

## Verification

- Run TypeScript checks.
- Run component tests for board, filters, workspace, and repository form.
- Run Playwright MCP on desktop and mobile viewports.
- Capture screenshots after UI layout changes.
- Compare screenshots against `specs/design.png` before completing each UI phase.
