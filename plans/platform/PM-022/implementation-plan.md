# Implementation Plan: PM-022 - Structured Knowledge Wiki

## Overview

Implement Knowledge as a separate indexed read model over compatible Markdown Wiki roots. Complete and commit each phase before starting the next. Preserve existing API contracts and keep Wiki pages separate from Kanban items.

## Phases Summary

| Phase | Name                                   | Status   |
|-------|----------------------------------------|----------|
| B1    | Knowledge domain and parser            | Complete |
| B2    | Detection and persisted index          | Complete |
| B3    | Query APIs and page access             | Complete |
| B4    | Rescan, Sync, Enrich, and audit        | Complete |
| F1    | Routes, types, and API client          | Complete |
| F2    | Knowledge controller and Browse view   | Complete |
| F3    | Reader and Explorer integration        | Complete |
| F4    | Graph, settings, and responsive polish | Complete |
| V1    | Full verification and documentation    | Complete |
| UI1   | Knowledge layout repair                | Complete |
| UI2   | Integrated Wiki reading experience     | Complete |
| UI3   | Interactive domain landing pages       | Complete |
| UI4   | Hierarchical Wiki navigation           | Complete |
| UI5   | Dark-mode navigation simplification    | Complete |
| UI6   | Collapsible Wiki domains               | Complete |
| UI7   | Root domain ordering                   | Complete |
| UI8   | Root-only default expansion            | Complete |
| UI9   | Landing-domain child alignment         | Complete |
| UI10  | Uniform domain alignment               | Complete |
| UI11  | Consistent domain icons                | Complete |
| UI12  | Reader-to-tree synchronization         | Complete |

## Backend Phases

### Phase B1: Knowledge Domain And Parser

**Deliverables:**

- [x] Add Knowledge Wiki, page, link, warning, graph, and action result models.
- [x] Parse required and optional front matter with string/list normalization.
- [x] Extract Wiki links and relative Markdown links without treating code as links.
- [x] Resolve links, backlinks, duplicate slugs, and deterministic graph edges.
- [x] Add focused parser and relationship tests using the Discovery Wiki format as fixtures.

**Verification:** `go test ./internal/knowledge/...`

**Commit:** `PM-022: Add Knowledge page parser and relationships`

### Phase B2: Detection And Persisted Index

**Deliverables:**

- [x] Resolve `knowledge-index.yaml` through application config.
- [x] Detect Wiki roots only from registered sources with `index.md` and valid pages.
- [x] Enforce workspace, source, symlink, ignore, file, byte, page, link, and time budgets.
- [x] Persist metadata and relationships with atomic replacement by workspace and root.
- [x] Preserve previous index entries after failed scans and remove stale roots only after successful workspace detection.
- [x] Add registry fields for optional Knowledge settings while preserving existing YAML.

**Verification:** `go test ./internal/knowledge/... ./internal/config/... ./internal/registry/...`

**Commit:** `PM-022: Add Knowledge detection and index persistence`

### Phase B3: Query APIs And Page Access

**Deliverables:**

- [x] Add the Knowledge application service and wire it in the server composition root.
- [x] Add Wiki list, page list, page detail, and graph endpoints.
- [x] Read selected Markdown through guarded workspace access and return viewer-compatible classification.
- [x] Return empty arrays, stable warning codes, deterministic ordering, and bounded graph responses.
- [x] Extend saved-route validation for `/knowledge`.
- [x] Add service and HTTP contract tests for valid, missing, malformed, and unsafe requests.

**Verification:** `go test ./internal/application/knowledge/... ./internal/api/...`

**Commit:** `PM-022: Add Knowledge query APIs`

### Phase B4: Rescan, Sync, Enrich, And Audit

**Deliverables:**

- [x] Add one-Wiki Rescan with atomic index replacement.
- [x] Compose existing guarded Git pull with post-success workspace Wiki detection and rescan.
- [x] Execute configured enrichment executable and literal arguments without a shell.
- [x] Add timeout, process cleanup, bounded output, confirmation, and failure preservation.
- [x] Record sanitized `knowledge_enrich`, `knowledge_sync`, and `knowledge_rescan` audit events.
- [x] Add tests for Git confirmation/failure and all process execution outcomes.

**Verification:** `go test ./internal/application/knowledge/... ./internal/api/... ./internal/audit/...`

**Commit:** `PM-022: Add Knowledge synchronization and enrichment`

## Frontend Phases

### Phase F1: Routes, Types, And API Client

**Deliverables:**

- [x] Add Knowledge DTOs and API client methods.
- [x] Add the `/knowledge` route with encoded workspace, root, slug, and view state.
- [x] Extend route parsing, path generation, saved-route validation assumptions, and tests.
- [x] Add desktop and mobile Knowledge navigation with lazy page loading.

**Verification:** `npm run typecheck && npm test -- --run web/src/app/router.test.ts web/src/App.test.tsx`

**Commit:** `PM-022: Add Knowledge route and client contracts`

### Phase F2: Knowledge Controller And Browse View

**Deliverables:**

- [x] Add `useKnowledgeController` with stale-request protection and action invalidation.
- [x] Add workspace and Wiki selection with deterministic fallback behavior.
- [x] Render domain hierarchy, page summaries, metadata filters, and warnings.
- [x] Add keyboard navigation, route synchronization, loading, and empty states.
- [x] Add focused controller and Browse interaction tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-022: Add Knowledge browsing experience`

### Phase F3: Reader And Explorer Integration

**Deliverables:**

- [x] Render selected Markdown through the shared Content Viewer.
- [x] Intercept resolved Wiki links and preserve safe external-link behavior.
- [x] Show metadata, source references, outgoing links, backlinks, and warnings.
- [x] Add “Open in Explorer” with workspace-relative deep-link behavior.
- [x] Test reader navigation, removed pages, unresolved links, and Explorer routing.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/WorkspaceExplorerPage.test.tsx`

**Commit:** `PM-022: Add Knowledge reader and Explorer navigation`

### Phase F4: Graph, Settings, And Responsive Polish

**Deliverables:**

- [x] Add and lazy-load an interactive graph dependency behind a feature adapter.
- [x] Render domain-aware nodes, directed edges, selection, neighbor highlighting, filters, fit, pan, and zoom.
- [x] Add accessible relationship list and graph truncation feedback.
- [x] Add workspace Knowledge settings for enablement and enrichment executable/arguments.
- [x] Add Rescan, guarded Sync, and confirmed Enrich actions with status and bounded logs.
- [x] Complete mobile layout, focus behavior, live announcements, and feature styling.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/WorkspacesPage.test.ts && npm run build`

**Commit:** `PM-022: Add Knowledge graph and actions`

## Final Verification

### Phase V1: Full Verification And Documentation

**Deliverables:**

- [x] Verify Discovery `docs` detection and representative page/link metadata manually.
- [x] Verify Rescan, Sync confirmation, and configured Enrich against a disposable Git fixture.
- [x] Verify desktop and mobile Browse, Read, Graph, warnings, and Explorer navigation through focused interaction tests and the responsive production build; the in-app browser was unavailable for visual inspection.
- [x] Run Markdown formatting and update PM-022 documents for implementation changes.
- [x] Record final test counts and graph chunk size in the README implementation status.

**Verification:**

```bash
go test ./...
npm run typecheck
npm test -- --run
npm run build
git diff --check
```

**Commit:** `PM-022: Finalize Structured Knowledge Wiki`

### Phase UI1: Knowledge Layout Repair

**Deliverables:**

- [x] Replace invalid Knowledge-only color tokens with the application theme tokens.
- [x] Apply established application button, panel, form, and active-state styling.
- [x] Remove the permanent empty live-status row and restore compact toolbar spacing.
- [x] Collapse large Wiki warning collections and bound their expanded height.
- [x] Add a focused regression test for controls, status spacing, and warning overflow.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/KnowledgePage.test.tsx web/src/features/knowledge && npm test -- --run && npm run build`

**Commit:** `PM-022: Repair Knowledge page layout`

### Phase UI2: Integrated Wiki Reading Experience

**Deliverables:**

- [x] Keep the Wiki index visible while reading page content.
- [x] Open full page content with one click instead of an intermediate summary and “Read page” action.
- [x] Replace Browse and Read modes with a single Pages workspace while retaining Graph as a separate view.
- [x] Move repository-wide scan warnings into collapsed, clearly scoped Index diagnostics.
- [x] Correct swapped detail and graph request guards that left page reads stuck on “Loading page…”.
- [x] Add regression coverage for single-click navigation, persistent index visibility, and completed page loading.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/KnowledgePage.test.tsx web/src/features/knowledge && npm run build`

**Commit:** `PM-022: Integrate Knowledge index and reader`

### Phase UI3: Interactive Domain Landing Pages

**Deliverables:**

- [x] Recognize `index.md` and `README.md` as domain landing pages.
- [x] Promote each domain heading into an accessible landing-page control.
- [x] Remove the landing page from the child list to avoid duplicate hierarchy entries.
- [x] Preserve keyboard navigation across parent and child page controls.
- [x] Add focused regression coverage for parent navigation and child rendering.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Add interactive Knowledge domain parents`

### Phase UI4: Hierarchical Wiki Navigation

**Deliverables:**

- [x] Replace the landing-page chevron with a bookmark icon that does not imply collapsing.
- [x] Render page types as distinct Concept, Reference, How-to, and fallback badges.
- [x] Convert slash-separated domain labels into an indented tree with hierarchy guide lines.
- [x] Show only the local domain segment at nested levels while preserving full-path navigation labels.
- [x] Keep the hierarchy expanded and preserve arrow-key navigation across landing and page controls.
- [x] Add focused regression coverage for nested domains, landing icons, and page-type badges.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Improve Knowledge navigation hierarchy`

### Phase UI5: Dark-Mode Navigation Simplification

**Deliverables:**

- [x] Replace colored page-type badges with compact muted text labels.
- [x] Keep selected page titles on the high-contrast application text color.
- [x] Use a subtle tinted background, border, and left accent for selection.
- [x] Keep landing-page labels readable while applying accent color only to their icon.
- [x] Add focused regression coverage for simple labels and selected-page styling hooks.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Simplify Knowledge navigation styling`

### Phase UI6: Collapsible Wiki Domains

**Deliverables:**

- [x] Add independent expand and collapse controls to every domain with children.
- [x] Keep landing-page navigation separate from hierarchy toggling.
- [x] Start domains expanded and retain collapse state during the mounted session.
- [x] Temporarily reveal matching domains while filtering without discarding saved state.
- [x] Preserve page-entry arrow navigation without including toggle controls.
- [x] Add focused regression coverage for collapse, expand, filtering, and landing-page isolation.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Add collapsible Knowledge domains`

### Phase UI7: Root Domain Ordering

**Deliverables:**

- [x] Promote the top-level `root` domain ahead of all other domains.
- [x] Apply the ordering independently of API page order.
- [x] Preserve the existing relative order of all non-root domains.
- [x] Add focused regression coverage for unordered page input.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Keep root Knowledge domain first`

### Phase UI8: Root-Only Default Expansion

**Deliverables:**

- [x] Expand the top-level `root` domain by default.
- [x] Collapse every other top-level and nested domain by default.
- [x] Preserve manual expansion state during the mounted session.
- [x] Continue temporarily revealing matching domains while filtering.
- [x] Add focused regression coverage for initial root and non-root states.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Default Knowledge domains to collapsed`

### Phase UI9: Landing-Domain Child Alignment

**Deliverables:**

- [x] Identify domain groups that expose a landing-page icon.
- [x] Indent their direct page entries beyond the parent title position.
- [x] Preserve full-width hit targets within the available sidebar width.
- [x] Leave root and non-landing domain spacing unchanged.
- [x] Add focused regression coverage for the landing-domain styling hook.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Align Knowledge landing-page children`

### Phase UI10: Uniform Domain Alignment

**Deliverables:**

- [x] Align landing-domain titles with plain domain titles such as `root`.
- [x] Move the landing-page bookmark after the domain label so it does not shift content.
- [x] Remove the special landing-domain child offset.
- [x] Use the same direct-page spacing for root and landing domains.
- [x] Add focused regression coverage for label-first bookmark placement.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Normalize Knowledge domain alignment`

### Phase UI11: Consistent Domain Icons

**Deliverables:**

- [x] Render the Wiki icon to the left of every domain label, including `root`.
- [x] Keep landing-enabled domain labels clickable without changing plain-domain semantics.
- [x] Apply one consistent direct-child indentation to every domain.
- [x] Preserve nested hierarchy offsets and full-width sidebar containment.
- [x] Add focused regression coverage for root, plain, and landing-domain icons.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Standardize Knowledge domain icons`

### Phase UI12: Reader-To-Tree Synchronization

**Deliverables:**

- [x] Detect page selection changes initiated by reader references and backlinks.
- [x] Expand every ancestor domain for the selected page.
- [x] Clear filters that would hide the selected page.
- [x] Scroll the selected entry into the visible sidebar area and move keyboard focus to it.
- [x] Keep landing-page and direct-page entries addressable by slug.
- [x] Add focused regression coverage for nested external selection and focus.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/KnowledgePage.test.tsx && npm run build`

**Commit:** `PM-022: Synchronize Knowledge reader selection`

## Rollback And Compatibility

- Workspace YAML fields are additive and optional.
- The Knowledge index is app-owned cache data and can be deleted and rebuilt.
- Disabling Knowledge hides detection and actions without changing workspace files.
- Existing workspace, item, Explorer, search, Git, audit, and release contracts remain valid.
- If the graph package causes unacceptable bundle or accessibility regressions, keep Browse and Reader enabled while withholding Graph navigation until corrected.
