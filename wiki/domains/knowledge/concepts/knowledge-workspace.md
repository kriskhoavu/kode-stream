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
<!-- keywords: workspace wiki, markdown, metadata, index -->

[live] PM-022 Kode Stream detects structured Markdown under configured workspace sources and builds a derived Knowledge index; it does not replace the repository files. Evidence: `internal/knowledge/detector.go:50`, `internal/knowledge/store.go:56`.

Each page contributes a stable slug, title, type, topics, summary, source references, freshness, body, and relationships.
Warnings preserve partially usable pages when one file or link does not conform.

## Browse, Read, And Follow Relationships
<!-- chunkId: knowledge-workspace-browse -->
<!-- keywords: knowledge reader, backlinks, graph, wikilink -->

[live] PM-022 the Knowledge surface lists pages, renders a selected page through the shared content viewer, resolves wikilinks and backlinks, and exposes the same relationships as a graph. Evidence: `internal/knowledge/relationships.go:10`, `internal/knowledge/relationships.go:78`, `web/src/features/knowledge/KnowledgeReader.tsx:24`, `web/src/features/knowledge/KnowledgeGraph.tsx:141`.

PM-040's taxonomy controls how the corpus is grouped for navigation; it is layered over this core page and relationship
model. See [[knowledge-wiki-taxonomy]].

## Refresh Is Explicit
<!-- chunkId: knowledge-workspace-refresh -->
<!-- keywords: rescan, sync, enrich, confirmation -->

[live] PM-022 users can rescan the current wiki, synchronize the workspace checkout, or run configured enrichment before rebuilding the index. Evidence: `internal/knowledge/knowledge_service.go:85`, `internal/knowledge/knowledge_service.go:126`, `internal/knowledge/knowledge_service.go:186`.

Sync and enrich are mutations, so they use workspace locking, audit their result, and do not replace the prior usable
index until detection succeeds.

## Not supported
<!-- chunkId: knowledge-workspace-not-supported -->
<!-- keywords: arbitrary directory, silent mutation, conversation memory -->

[live] PM-022 Knowledge does not index an arbitrary traversal path or run enrichment without workspace configuration and confirmation. Evidence: `internal/knowledge/knowledge_service_test.go:108`, `internal/knowledge/knowledge_service_test.go:152`.

## Known conflicts
<!-- chunkId: knowledge-workspace-known-conflicts -->
<!-- keywords: old metadata, roles, source count, taxonomy -->

[legacy] PM-022 documents an older metadata shape containing fields such as `roles` and `sourceCount`. The current seven-field wiki standard is normative, while the parser retains limited compatibility data for older corpora.

## Related pages

See [[knowledge-workspace-reference]], [[knowledge-wiki-taxonomy]], and [[workspace-branch-context]].
