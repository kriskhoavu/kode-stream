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

[live] Classification assigns the first directory as bucket, consumes only the first recognized tier, and joins every other directory segment into the area; root pages have no classification. Evidence: `internal/knowledge/taxonomy.go:63`, `internal/knowledge/taxonomy.go:83`.

Taxonomy is applied after the complete page set is loaded because inference depends on relationships across paths.

## Settings Contract
<!-- chunkId: knowledge-wiki-taxonomy-reference-settings-contract -->
<!-- keywords: knowledge settings, schema version, strict fields, warning fallback -->

[live] Optional `knowledge-settings.yaml` supports schema version 1, a tier list, and bucket declarations with the `journeys` role; unknown fields or invalid versions produce warnings and fall back to detected taxonomy. Evidence: `internal/knowledge/taxonomy_settings.go:11`, `internal/knowledge/taxonomy_settings.go:44`.

Unknown roles are dropped. Duplicate valid bucket paths warn and keep the last declaration. A declared bucket absent from the tree warns without blocking indexing.

## Journey Identification
<!-- chunkId: knowledge-wiki-taxonomy-reference-journey-identification -->
<!-- keywords: journey bucket, e2e testing, exact segment, compatibility fallback -->

[live] Journey lookup uses the bucket assigned the `journeys` role and defaults to the exact `e2e-testing` segment; older indexes fall back to exact domain or `bucket/` prefix matching. Evidence: `internal/knowledge/taxonomy.go:116`, `internal/knowledge/taxonomy.go:140`.

## Historical Validation
<!-- chunkId: knowledge-wiki-taxonomy-reference-historical-validation -->
<!-- keywords: browser validation, contrast measurement, historical evidence -->

[legacy] The retired PM-040 evidence recorded browser grouping and graph checks plus sampled light and dark contrast values, but it is historical and not a fresh runtime verification. Source: `PM-040 (plan retired) | Automation result`.

## Not supported
<!-- chunkId: knowledge-wiki-taxonomy-reference-not-supported -->
<!-- keywords: custom roles, explicit empty tiers, durable classification -->

[live] Custom role values, an explicit empty-tier override, and durable inferred classification are not supported. Evidence: `internal/knowledge/taxonomy_settings.go:21`, `internal/knowledge/taxonomy_settings.go:95`.

## Known conflicts
<!-- chunkId: knowledge-wiki-taxonomy-reference-known-conflicts -->
<!-- keywords: override semantics, empty list, plan intent -->

[live] The retired plan described any present tier list as replacement, while the implementation replaces inference only when that list is non-empty. Evidence: `internal/knowledge/taxonomy_settings.go:95`.

## Related pages

See [[knowledge-wiki-taxonomy]], [[knowledge-index]], and [[e2e-testing-index]].
