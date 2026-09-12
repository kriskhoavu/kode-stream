---
slug: knowledge-wiki-taxonomy
title: Wiki Taxonomy
pageType: CONCEPT
topics: knowledge, taxonomy, buckets, tiers, areas
summary: Wiki navigation derives buckets, areas, and reusable tiers from directory structure instead of product-specific names.
sourceRef: |
  PM-040 (plan retired) | Taxonomy model
  PM-040 (plan retired) | User experience
  PM-040 (plan retired) | Navigation presentation
  PM-040 (plan retired) | Visual brief
lastVerified: 2026-09-12
---

## Navigation Model
<!-- chunkId: knowledge-wiki-taxonomy-navigation-model -->
<!-- keywords: bucket, area, tier, directory structure -->

[live] The first directory is a bucket, a repeated structural directory can become a tier, and all remaining directory segments form the area. Evidence: `internal/knowledge/taxonomy.go:20`, `internal/knowledge/taxonomy.go:63`.

This lets the same browser present domain knowledge and E2E journeys without hard-coding repository-specific folder names. The graph exposes tier as an attribute; the browser groups bucket, area, and collapsible tier partitions.

## Why Tiers Are Learned Locally
<!-- chunkId: knowledge-wiki-taxonomy-learned-locally -->
<!-- keywords: parent threshold, bucket scope, false tier, corpus -->

[live] A directory name becomes a tier only after appearing below two distinct parents within one bucket. Evidence: `internal/knowledge/taxonomy.go:8`, `internal/knowledge/taxonomy.go:27`.

Counting within a bucket prevents a subject name repeated in unrelated buckets from becoming a false global tier. Explicit settings are available when inference is too unstable for a small or changing corpus.

## Navigation Behavior
<!-- chunkId: knowledge-wiki-taxonomy-navigation-behavior -->
<!-- keywords: knowledge browser, graph view, collapsed tier, selection -->

[live] Browser and graph views share the same taxonomy projection; selecting or filtering a page opens the containing branch so the result remains visible. Evidence: `web/src/features/knowledge/taxonomy.ts:1`, `web/src/features/knowledge/KnowledgeBrowser.tsx:21`.

## Not supported
<!-- chunkId: knowledge-wiki-taxonomy-not-supported -->
<!-- keywords: content instruction, taxonomy persistence, custom role -->

[live] Retrieved page content does not instruct the classifier, inferred taxonomy is not persisted as authoritative state, and the only recognized bucket role is `journeys`. Evidence: `internal/knowledge/taxonomy_settings.go:15`, `internal/knowledge/taxonomy.go:94`.

## Known conflicts
<!-- chunkId: knowledge-wiki-taxonomy-known-conflicts -->
<!-- keywords: empty tier override, replacement semantics, plan discrepancy -->

[live] A non-empty configured tier list replaces inferred tiers, but an empty list leaves inference intact; an empty list therefore cannot explicitly clear all tiers. Evidence: `internal/knowledge/taxonomy_settings.go:85`, `internal/knowledge/taxonomy_settings.go:95`.

## Related pages

See [[knowledge-wiki-taxonomy-reference]], [[knowledge-index]], and [[e2e-testing-index]].
