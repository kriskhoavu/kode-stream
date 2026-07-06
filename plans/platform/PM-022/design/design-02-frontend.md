# Frontend Design: Structured Knowledge Wiki

## Overview

The frontend adds a top-level Knowledge route and feature module. It keeps selection in the URL, server data in feature hooks, and temporary UI state locally. Browse, Read, and Graph are views over the same backend index. Markdown rendering and editing remain owned by existing shared features.

## Route Contract

```ts
type KnowledgeLocation = {
  workspaceId?: string;
  root?: string;
  slug?: string;
  view?: 'browse' | 'read' | 'graph';
};

type Route =
  | ExistingRoutes
  | { name: 'knowledge'; location?: KnowledgeLocation };
```

`/knowledge` opens the first available Wiki. Query parameters preserve workspace, root, slug, and view. Invalid selections fall back to the nearest valid level and show a non-blocking notice. Browser Back and Forward restore selection.

## Information Architecture

```text
Knowledge
├── Workspace and Wiki selector
├── Actions: Rescan, Sync, Enrich
├── Browse
│   └── Domain tree and page summaries
├── Read
│   ├── Metadata and sanitized Markdown
│   ├── Outgoing links
│   ├── Backlinks
│   └── Open in Explorer
└── Graph
    ├── Search and domain filters
    ├── Interactive nodes and directed edges
    └── Selected-page details
```

Desktop uses a persistent domain/page rail with the active view in the main panel. Mobile uses a view switcher and collapsible navigation; Knowledge is included in bottom navigation without removing Settings access from the profile menu.

## Feature Structure

```text
web/src/features/knowledge/
├── api.ts
├── types.ts
├── useKnowledgeController.ts
├── KnowledgeBrowser.tsx
├── KnowledgeReader.tsx
├── KnowledgeGraph.tsx
├── KnowledgeWarnings.tsx
├── knowledgeGraph.ts
└── knowledge.css

web/src/pages/KnowledgePage.tsx
```

## State Ownership

| State                                   | Owner                         | Persistence             |
|-----------------------------------------|-------------------------------|-------------------------|
| Workspace, root, page slug, active view | Application route             | URL                     |
| Wikis, pages, detail, graph, warnings   | `useKnowledgeController`      | Request cache in memory |
| Domain expansion                        | `KnowledgeBrowser`            | Local storage by Wiki   |
| Graph viewport and filters              | `KnowledgeGraph`              | Component memory        |
| Action busy/result state                | `useKnowledgeController`      | Component memory        |
| Enrich executable and arguments         | Existing workspace form state | Workspace registry      |

Stale responses are ignored when workspace, root, or slug changes. Rescan, Sync, and Enrich invalidate Wiki list, page list, detail, graph, workspace state, Git status, and activity queries as applicable.

## Browse View

- Render folders as domains and leaf entries as pages.
- Default-expand ancestors of the selected page.
- Show title and compact page type; show summary in the page list or detail panel.
- Support title, slug, topic, role, and summary filtering inside the selected Wiki.
- Provide keyboard movement, expansion, selection, and activation consistent with Explorer.
- Display warning count at Wiki and page levels without blocking navigation.

## Reader View

- Load content only for the selected slug.
- Render through `ContentViewer` in read-only Markdown mode.
- Intercept resolved internal Wiki links and navigate through the Knowledge route.
- Show metadata, source references, outgoing links, and backlinks in a responsive side panel.
- “Open in Explorer” navigates to `/explorer` with workspace ID, relative path, and source mode.
- Keep external and unresolved links visibly distinct; unresolved links open no route.

## Graph View

Add one focused graph dependency rather than hand-building zoom and focus management. Wrap it behind `KnowledgeGraph` and `knowledgeGraph.ts` so layout and package-specific models do not leak into API types.

- Group or color nodes by domain using theme tokens.
- Size nodes within a small bounded range using inbound link count.
- Highlight selected node, direct neighbors, and connecting edges.
- Search by title or slug and filter by domain or page type.
- Selecting a node updates the route and reader selection.
- Provide zoom controls, fit view, reset, accessible node labels, and keyboard activation.
- Show total counts and truncation notice returned by the backend.
- Lazy-load the graph package so Kanban, Explorer, and initial Knowledge Browse do not absorb its bundle cost.

## Actions

| Action | Confirmation                                                       | Success                                                        |
|--------|--------------------------------------------------------------------|----------------------------------------------------------------|
| Rescan | None                                                               | Reload current Wiki and show page/warning counts               |
| Sync   | Only when existing Git status reports a dirty working tree         | Reload all workspace Wikis and show pull plus scan result      |
| Enrich | Always; show executable, arguments, working directory, and warning | Show bounded output, reload all workspace Wikis and Git status |

Enrich is disabled when configuration is absent and links to the selected workspace's Knowledge settings. Buttons remain disabled while an action for that workspace is running. Errors preserve current content and display existing recovery-hint patterns.

## Workspace Settings

Add a Knowledge section to workspace details:

- Enable automatic Wiki detection, defaulting to enabled.
- Enrichment executable text input.
- Repeatable literal argument inputs with add, remove, and reorder controls.
- Explain that no shell expansion occurs and execution starts at the workspace root.
- Show detected Wiki roots after save and scan.

Do not accept environment-variable values or secrets in this form.

## Navigation

- Add a `Knowledge` item with a book/network-appropriate Lucide icon to desktop navigation.
- Add Knowledge to mobile bottom navigation and keep Settings reachable from the profile menu.
- Extend route parsing, path generation, saved-route validation, and route tests.
- Extend global search later; PM-022 provides Wiki-local filtering only.

## Empty And Error States

| State                 | Presentation                                                       |
|-----------------------|--------------------------------------------------------------------|
| No workspaces         | Link to Add Workspace                                              |
| No detected Wikis     | Explain `index.md` plus front matter contract and link to Explorer |
| Empty valid Wiki      | Explain that no valid pages were indexed and show warnings         |
| Selected page removed | Return to Browse and show a rescan-change notice                   |
| Index unavailable     | Preserve navigation shell and offer Rescan                         |
| Graph truncated       | Show rendered/total counts and suggest domain filtering            |
| Enrich not configured | Disabled action with link to workspace Knowledge settings          |
| Action failure        | Keep current data and show operation log plus recovery hint        |

## Accessibility And Responsive Behavior

- Use semantic buttons, headings, navigation landmarks, and status regions.
- Announce loading and action completion through a polite live region.
- Preserve visible focus in domain tree, page list, graph controls, and nodes.
- Do not rely only on graph color for domain or selection state.
- Provide a list-based relationship view alongside graph data for screen readers.
- Collapse the metadata panel below content on narrow screens.

## Verification

- Router tests for default, fully selected, malformed, and encoded locations.
- Controller tests for selection fallback, stale requests, cache invalidation, and action results.
- Browse tests for hierarchy, filtering, warnings, keyboard navigation, and empty states.
- Reader tests for metadata, internal link interception, backlinks, external links, and Explorer navigation.
- Graph tests for model adaptation, selection, filters, truncation, and accessible fallback list.
- Settings tests for defaults, argument editing, save payloads, and disabled Enrich behavior.
- App shell tests for desktop and mobile navigation.
- Production build check to record lazy graph chunk size.
