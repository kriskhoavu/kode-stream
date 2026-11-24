---
slug: knowledge-workspace-reference
title: Workspace Knowledge Reference
pageType: REFERENCE
topics: knowledge API, scan limits, metadata parser, graph limits, refresh actions
summary: Reference for Knowledge discovery, metadata parsing, relationship resolution, query APIs, and guarded refresh actions.
sourceRef: |
  PM-022 (plan retired) | Backend and frontend Knowledge contracts
lastVerified: 2026-09-12
---

# Workspace Knowledge Reference

## Index Contract
<!-- chunkId: knowledge-workspace-reference-index -->
<!-- keywords: detector, page limit, partial page, atomic replace -->

[live] Detection bounds files, file bytes, total bytes and pages. The default page limit is 5,000. An oversized file can stay visible as a partial page with a warning. Evidence: `internal/knowledge/detector.go:22`.

The index is written to a temporary file and synced before replacement. A failed rebuild keeps the last complete index.

## Page And Graph Contract
<!-- chunkId: knowledge-workspace-reference-page-graph -->
<!-- keywords: backlinks, graph nodes, graph edges, routes -->

[live] Valid wikilinks produce deterministic backlinks and graph edges. A graph response holds at most 2,000 nodes and 10,000 edges. Evidence: `internal/knowledge/knowledge_service.go:28`.

| API operation | Purpose                                                    |
|---------------|------------------------------------------------------------|
| List wikis    | Discover Knowledge roots for a workspace                   |
| List pages    | Page summaries and warnings for one root                   |
| Read page     | Guarded Markdown and metadata by slug                      |
| Read graph    | Bounded nodes and resolved links                           |
| Rescan        | Re-read one wiki from the checkout                         |
| Sync          | Pull through guarded Git, then rebuild                     |
| Enrich        | Run configured enrichment after confirmation, then rebuild |

The routes are in `internal/server/api/gin_routes.go`.

## Refresh Safety
<!-- chunkId: knowledge-workspace-reference-refresh -->
<!-- keywords: workspace lock, audit, timeout, output limit -->

[live] Sync and enrich hold the workspace lock, are audited and rescan only after they succeed. Enrichment times out after five minutes by default, with bounded output. Evidence: `internal/knowledge/knowledge_service.go:31`.

## Not supported
<!-- chunkId: knowledge-workspace-reference-not-supported -->
<!-- keywords: unsafe root, unconfigured enrich, partial replacement -->

[live] An unsafe wiki root, unconfigured enrichment, or failed detection cannot replace the index. Evidence: `internal/knowledge/knowledge_service_test.go:120`.

## Known conflicts
<!-- chunkId: knowledge-workspace-reference-known-conflicts -->
<!-- keywords: flat domain navigation, legacy field, grouping -->

- "Knowledge groups pages by a flat domain." It groups by bucket and area. See [[knowledge-wiki-taxonomy-reference]].

## Related pages

See [[knowledge-workspace]] and [[platform-storage-reference]].
