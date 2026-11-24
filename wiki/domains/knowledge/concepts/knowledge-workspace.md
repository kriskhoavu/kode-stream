---
slug: knowledge-workspace
title: Workspace Knowledge
pageType: CONCEPT
topics: knowledge, structured wiki, reader, relationships, graph
summary: Kode Stream discovers structured Markdown wikis in a workspace and turns their metadata and links into browsable pages and a relationship graph.
sourceRef: |
  PM-022 (plan retired) | Structured Knowledge model
lastVerified: 2026-09-12
---

# Workspace Knowledge

## Repository Markdown Remains The Source
<!-- chunkId: knowledge-workspace-source -->
<!-- keywords: workspace wiki, metadata, derived index, warnings -->

[live] Kode Stream finds structured Markdown under configured sources and builds a derived Knowledge index. The repository files stay the source. Evidence: `internal/knowledge/detector.go:49`, `internal/knowledge/store.go:58`.

Each page gives a slug, title, type, topics, summary, source references, freshness, body and links. A file or link that does not conform raises a warning, and the rest of the page stays usable.

## Browse, Read, And Follow Relationships
<!-- chunkId: knowledge-workspace-browse -->
<!-- keywords: knowledge reader, backlinks, graph, wikilink -->

[live] Knowledge lists pages, renders one in the shared viewer, resolves wikilinks and backlinks, and draws the same links as a graph. Evidence: `internal/knowledge/relationships.go:10`, `web/src/features/knowledge/KnowledgeGraph.tsx:125`.

[[knowledge-wiki-taxonomy]] decides how pages are grouped for navigation.

## Refresh Is Explicit
<!-- chunkId: knowledge-workspace-refresh -->
<!-- keywords: rescan, sync, enrich, workspace lock -->

[live] A person can rescan the wiki, sync the checkout or run configured enrichment, then rebuild the index. Evidence: `internal/knowledge/knowledge_actions.go:15`, `internal/knowledge/knowledge_actions.go:116`.

Sync and enrich take the workspace lock and are audited. The old index stays until detection succeeds.

## Not supported
<!-- chunkId: knowledge-workspace-not-supported -->
<!-- keywords: arbitrary directory, silent mutation, conversation memory -->

[live] Indexing a traversal path, and enrichment without workspace configuration and confirmation. Evidence: `internal/knowledge/knowledge_service_test.go:108`.

## Known conflicts
<!-- chunkId: knowledge-workspace-known-conflicts -->
<!-- keywords: roles field, sourceCount, metadata schema -->

- "Pages carry `roles` and `sourceCount`." The seven-field standard replaces them. The parser still reads a little of the older shape.

## Related pages

See [[knowledge-workspace-reference]] and [[workspace-branch-context]].
