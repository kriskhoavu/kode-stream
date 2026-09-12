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
<!-- keywords: detector, page limit, file limit, partial page -->

[live] PM-022 detection bounds files, file bytes, total bytes, and pages; the default page ceiling is 5,000 and oversized files can remain visible as partial pages with warnings. Evidence: `internal/knowledge/detector.go:23`, `internal/knowledge/detector.go:209`, `internal/knowledge/detector_test.go:57`.

Index publication uses temporary-file and directory synchronization before replacement. A failed rebuild leaves the
last complete index available rather than publishing a partial replacement.

## Page And Graph Contract
<!-- chunkId: knowledge-workspace-reference-page-graph -->
<!-- keywords: page detail, backlinks, graph nodes, graph edges -->

[live] PM-022 relationship resolution produces deterministic backlinks and graph edges from valid wikilinks, with graph responses bounded to 2,000 nodes and 10,000 edges. Evidence: `internal/knowledge/relationships.go:10`, `internal/knowledge/knowledge_service.go:33`.

| API operation | Purpose                                                    |
|---------------|------------------------------------------------------------|
| List wikis    | Discover Knowledge roots for a workspace                   |
| List pages    | Retrieve page summaries and warnings for one root          |
| Read page     | Retrieve guarded Markdown and metadata by slug             |
| Read graph    | Retrieve bounded nodes and resolved relationships          |
| Rescan        | Re-read one wiki from the current checkout                 |
| Sync          | Pull through guarded Git behavior, then rebuild            |
| Enrich        | Run configured enrichment after confirmation, then rebuild |

The current Gin manifest exposes page, graph, rescan, sync, and enrich routes. Evidence:
`internal/server/api/gin_routes.go:172`, `internal/server/api/gin_routes.go:175`, `internal/server/api/gin_routes.go:178`.

## Refresh Safety
<!-- chunkId: knowledge-workspace-reference-refresh -->
<!-- keywords: workspace lock, audit, timeout, output limit -->

[live] PM-022 sync and enrich hold the workspace mutation boundary, record audit outcomes, and rescan only after their mutation succeeds; enrichment has a five-minute default timeout and bounded output. Evidence: `internal/knowledge/knowledge_service.go:36`, `internal/knowledge/knowledge_service.go:294`, `internal/knowledge/knowledge_service.go:306`.

## Not supported
<!-- chunkId: knowledge-workspace-reference-not-supported -->
<!-- keywords: unsafe root, unconfigured enrich, partial replacement -->

[live] PM-022 unsafe wiki roots, unconfigured enrichment, and failed detection cannot replace the indexed corpus. Evidence: `internal/knowledge/knowledge_service_test.go:108`, `internal/knowledge/knowledge_service_test.go:120`, `internal/knowledge/knowledge_service_test.go:152`.

## Known conflicts
<!-- chunkId: knowledge-workspace-reference-known-conflicts -->
<!-- keywords: metadata schema, taxonomy, legacy field -->

[legacy] PM-040 supersedes PM-022's flat domain navigation and optional legacy metadata fields. Core discovery, page, link, graph, and action contracts remain current.

## Related pages

See [[knowledge-workspace]], [[knowledge-wiki-taxonomy-reference]], and [[platform-storage-reference]].
