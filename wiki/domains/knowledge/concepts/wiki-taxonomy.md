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

[live] The first directory is the bucket. A repeated structural directory can become a tier. The other directories form the area. Evidence: `internal/knowledge/taxonomy.go:17`.

So one browser shows domain pages and E2E journeys without hard-coded folder names. Both views group by bucket, then area, and show the tier as a page attribute.

## Why Tiers Are Learned Locally
<!-- chunkId: knowledge-wiki-taxonomy-learned-locally -->
<!-- keywords: parent threshold, bucket scope, false tier -->

[live] A directory name becomes a tier only under two distinct parents in one bucket. Evidence: `internal/knowledge/taxonomy.go:8`.

Counting per bucket stops a name repeated across buckets from becoming a false tier. `knowledge-settings.yaml` fixes the tiers when inference is unstable.

## Navigation Behavior
<!-- chunkId: knowledge-wiki-taxonomy-navigation-behavior -->
<!-- keywords: knowledge browser, graph view, selection, filter -->

[live] The browser and graph share one taxonomy projection. Selecting or filtering a page opens its group so the page stays visible. Evidence: `web/src/features/knowledge/KnowledgeBrowser.tsx:21`.

## Not supported
<!-- chunkId: knowledge-wiki-taxonomy-not-supported -->
<!-- keywords: content instruction, taxonomy persistence, custom role -->

[live] Page content never instructs the classifier. Inferred taxonomy is never stored as authority. The only bucket role is `journeys`. Evidence: `internal/knowledge/taxonomy_settings.go:13`.

## Known conflicts
<!-- chunkId: knowledge-wiki-taxonomy-known-conflicts -->
<!-- keywords: empty tier override, replacement semantics, settings file -->

- "An empty tier list in settings clears all tiers." It leaves inference on. Only a non-empty list replaces it. Evidence: `internal/knowledge/taxonomy_settings.go:79`.

## Related pages

See [[knowledge-wiki-taxonomy-reference]] and [[e2e-testing-index]].
