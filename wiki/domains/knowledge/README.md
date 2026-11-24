---
slug: knowledge-index
title: Knowledge Navigation
pageType: REFERENCE
topics: knowledge, wiki, taxonomy, navigation
summary: Knowledge pages are classified from their directory shape for browsing and journey discovery.
sourceRef: |
  PM-022 (plan retired) | Knowledge navigation
  PM-040 (plan retired) | Knowledge navigation
lastVerified: 2026-09-12
---

# Knowledge Navigation

## Pages
<!-- chunkId: knowledge-index-pages -->
<!-- keywords: taxonomy concept, navigation reference, wiki structure -->

[live] [[knowledge-workspace]] and [[knowledge-workspace-reference]] cover discovery, reading, relationships and refresh. [[knowledge-wiki-taxonomy]] and [[knowledge-wiki-taxonomy-reference]] cover how pages are grouped. Evidence: `internal/knowledge/detector.go:49`, `internal/knowledge/taxonomy.go:17`.

## Not written yet
<!-- chunkId: knowledge-index-not-written -->
<!-- keywords: freshness claims, chunk retrieval, ranking -->

[open] Should claim status, `lastVerified` and chunk metadata affect ranking? The parser gives them no ranking meaning.

## Open questions
<!-- chunkId: knowledge-index-open-questions -->
<!-- keywords: stable taxonomy, explicit tiers, corpus growth -->

[open] When should a young corpus fix its tiers in `knowledge-settings.yaml`? Each rescan infers them again.

## Related pages

See [[workspace-index]], [[e2e-testing-index]] and [[platform-documentation-audit]].
