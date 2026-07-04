# Implementation Plan: PM-022 - Structured Knowledge Wiki

## Overview

Implement Knowledge as a separate indexed read model over compatible Markdown Wiki roots. Complete and commit each phase before starting the next. Preserve existing API contracts and keep Wiki pages separate from Kanban items.

## Phases Summary

| Phase | Name                                   | Status   |
|-------|----------------------------------------|----------|
| B1    | Knowledge domain and parser            | Complete |
| B2    | Detection and persisted index          | Complete |
| B3    | Query APIs and page access             | Complete |
| B4    | Rescan, Sync, Enrich, and audit        | Pending  |
| F1    | Routes, types, and API client          | Pending  |
| F2    | Knowledge controller and Browse view   | Pending  |
| F3    | Reader and Explorer integration        | Pending  |
| F4    | Graph, settings, and responsive polish | Pending  |
| V1    | Full verification and documentation    | Pending  |

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

- [ ] Add one-Wiki Rescan with atomic index replacement.
- [ ] Compose existing guarded Git pull with post-success workspace Wiki detection and rescan.
- [ ] Execute configured enrichment executable and literal arguments without a shell.
- [ ] Add timeout, process cleanup, bounded output, confirmation, and failure preservation.
- [ ] Record sanitized `knowledge_enrich`, `knowledge_sync`, and `knowledge_rescan` audit events.
- [ ] Add tests for Git confirmation/failure and all process execution outcomes.

**Verification:** `go test ./internal/application/knowledge/... ./internal/api/... ./internal/audit/...`

**Commit:** `PM-022: Add Knowledge synchronization and enrichment`

## Frontend Phases

### Phase F1: Routes, Types, And API Client

**Deliverables:**

- [ ] Add Knowledge DTOs and API client methods.
- [ ] Add the `/knowledge` route with encoded workspace, root, slug, and view state.
- [ ] Extend route parsing, path generation, saved-route validation assumptions, and tests.
- [ ] Add desktop and mobile Knowledge navigation with lazy page loading.

**Verification:** `npm run typecheck && npm test -- --run web/src/app/router.test.ts web/src/App.test.tsx`

**Commit:** `PM-022: Add Knowledge route and client contracts`

### Phase F2: Knowledge Controller And Browse View

**Deliverables:**

- [ ] Add `useKnowledgeController` with stale-request protection and action invalidation.
- [ ] Add workspace and Wiki selection with deterministic fallback behavior.
- [ ] Render domain hierarchy, page summaries, metadata filters, and warnings.
- [ ] Add keyboard navigation, route synchronization, loading, and empty states.
- [ ] Add focused controller and Browse interaction tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-022: Add Knowledge browsing experience`

### Phase F3: Reader And Explorer Integration

**Deliverables:**

- [ ] Render selected Markdown through the shared Content Viewer.
- [ ] Intercept resolved Wiki links and preserve safe external-link behavior.
- [ ] Show metadata, source references, outgoing links, backlinks, and warnings.
- [ ] Add “Open in Explorer” with workspace-relative deep-link behavior.
- [ ] Test reader navigation, removed pages, unresolved links, and Explorer routing.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/WorkspaceExplorerPage.test.tsx`

**Commit:** `PM-022: Add Knowledge reader and Explorer navigation`

### Phase F4: Graph, Settings, And Responsive Polish

**Deliverables:**

- [ ] Add and lazy-load an interactive graph dependency behind a feature adapter.
- [ ] Render domain-aware nodes, directed edges, selection, neighbor highlighting, filters, fit, pan, and zoom.
- [ ] Add accessible relationship list and graph truncation feedback.
- [ ] Add workspace Knowledge settings for enablement and enrichment executable/arguments.
- [ ] Add Rescan, guarded Sync, and confirmed Enrich actions with status and bounded logs.
- [ ] Complete mobile layout, focus behavior, live announcements, and feature styling.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge web/src/pages/WorkspacesPage.test.ts && npm run build`

**Commit:** `PM-022: Add Knowledge graph and actions`

## Final Verification

### Phase V1: Full Verification And Documentation

**Deliverables:**

- [ ] Verify Discovery `docs` detection and representative page/link metadata manually.
- [ ] Verify Rescan, Sync confirmation, and configured Enrich against a disposable Git fixture.
- [ ] Verify desktop and mobile Browse, Read, Graph, warnings, and Explorer navigation.
- [ ] Run Markdown formatting and update PM-022 documents for implementation changes.
- [ ] Record final test counts and graph chunk size in the README implementation status.

**Verification:**

```bash
go test ./...
npm run typecheck
npm test -- --run
npm run build
git diff --check
```

**Commit:** `PM-022: Finalize Structured Knowledge Wiki`

## Rollback And Compatibility

- Workspace YAML fields are additive and optional.
- The Knowledge index is app-owned cache data and can be deleted and rebuilt.
- Disabling Knowledge hides detection and actions without changing workspace files.
- Existing workspace, item, Explorer, search, Git, audit, and release contracts remain valid.
- If the graph package causes unacceptable bundle or accessibility regressions, keep Browse and Reader enabled while withholding Graph navigation until corrected.
