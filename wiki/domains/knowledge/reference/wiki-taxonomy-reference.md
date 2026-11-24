---
slug: knowledge-wiki-taxonomy-reference
title: Wiki Taxonomy Reference
pageType: REFERENCE
topics: knowledge, taxonomy, settings, journeys, warnings
summary: Reference for taxonomy inference, optional settings, warnings, and journey identification.
sourceRef: |
  PM-040 (plan retired) | Classification rules
  PM-040 (plan retired) | Delivery and compatibility
  PM-040 (plan retired) | Validation approach
  PM-040 (plan retired) | Historical validation
lastVerified: 2026-09-12
---

## Classification Algorithm
<!-- chunkId: knowledge-wiki-taxonomy-reference-classification -->
<!-- keywords: directory segments, first tier, retained area, root page -->

[live] The first directory is the bucket. Only the first recognized tier is used. Every other directory joins the area. Root pages have no classification. Evidence: `internal/knowledge/taxonomy.go:54`.

Taxonomy runs after the whole page set loads, because inference compares paths.

## Settings Contract
<!-- chunkId: knowledge-wiki-taxonomy-reference-settings-contract -->
<!-- keywords: knowledge settings, schema version, warning fallback -->

[live] Optional `knowledge-settings.yaml` at the wiki root supports schema version 1, a tier list, and bucket declarations with the `journeys` role. An unknown field or version warns and falls back to detection. Evidence: `internal/knowledge/taxonomy_settings.go:11`.

- An unknown role is dropped.
- A duplicate bucket path warns, and the last one wins.
- A declared bucket missing from the tree warns but does not block indexing.

## Journey Identification
<!-- chunkId: knowledge-wiki-taxonomy-reference-journey-identification -->
<!-- keywords: journey bucket, e2e testing, compatibility fallback -->

[live] Journeys live in the bucket with the `journeys` role, which defaults to `e2e-testing`. An index entry without a bucket falls back to its exact domain or a `bucket/` prefix. Evidence: `internal/knowledge/taxonomy.go:98`.

## Not supported
<!-- chunkId: knowledge-wiki-taxonomy-reference-not-supported -->
<!-- keywords: custom roles, explicit empty tiers, durable classification -->

[live] Custom roles, an explicit empty-tier override and stored inferred classification. Evidence: `internal/knowledge/taxonomy_settings.go:18`.

## Known conflicts
<!-- chunkId: knowledge-wiki-taxonomy-reference-known-conflicts -->
<!-- keywords: override semantics, empty list, inference -->

- "Any tier list in settings replaces inference." Only a non-empty list does. Evidence: `internal/knowledge/taxonomy_settings.go:79`.

## Related pages

See [[knowledge-wiki-taxonomy]] and [[knowledge-index]].
