---
slug: knowledge-index
title: Knowledge Navigation
pageType: REFERENCE
topics: knowledge, wiki, taxonomy, navigation
summary: Knowledge pages are classified from their directory shape for browsing and journey discovery.
sourceRef: |
  plans/platform/PM-022/README.md
  PM-040 (plan retired) | Knowledge navigation
lastVerified: 2026-09-12
---

# Knowledge Navigation

## Pages
<!-- chunkId: knowledge-index-pages -->
<!-- keywords: taxonomy concept, navigation reference, wiki structure -->

[live] PM-022 [[knowledge-workspace]] and [[knowledge-workspace-reference]] describe discovery, reading, relationships, and refresh actions; [[knowledge-wiki-taxonomy]] and [[knowledge-wiki-taxonomy-reference]] record the later classification model. Evidence: `internal/knowledge/detector.go:50`, `internal/knowledge/taxonomy.go:27`, `web/src/features/knowledge/taxonomy.ts:1`.

## Not written yet
<!-- chunkId: knowledge-index-not-written -->
<!-- keywords: freshness claims, chunk retrieval, metadata parsing -->

[open] How should claim status, `lastVerified`, and chunk metadata influence retrieval ranking? The current parser does not assign those fields ranking semantics.

## Open questions
<!-- chunkId: knowledge-index-open-questions -->
<!-- keywords: stable taxonomy, explicit tiers, corpus growth -->

[open] When should a young corpus replace inferred tiers with an explicit `knowledge-settings.yaml` override? Reclassification currently occurs on each rescan.

## Related pages

See [[workspace-index]], [[e2e-testing-index]], and [[platform-documentation-audit]].
