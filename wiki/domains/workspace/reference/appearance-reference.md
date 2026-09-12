---
slug: workspace-appearance-reference
title: Workspace Appearance Reference
pageType: REFERENCE
topics: appearance, backdrop, lattice, settings, motion
summary: Reference for backdrop variants, route coverage, persistence, and reduced-motion behavior.
sourceRef: |
  PM-041 (plan retired) | Backdrop contract
  PM-041 (plan retired) | Historical validation
lastVerified: 2026-09-12
---

## Variant Contract
<!-- chunkId: workspace-appearance-reference-variant-contract -->
<!-- keywords: lattice, neon, none, default variant -->

[live] The valid variants are `lattice`, `neon`, and `none`, the default is lattice, and invalid persisted values normalize to that default. Evidence: `web/src/components/HeaderBackdrop.tsx:14`, `web/src/components/HeaderBackdrop.tsx:20`.

## Geometry And Motion
<!-- chunkId: workspace-appearance-reference-geometry-motion -->
<!-- keywords: field geometry, seeded walk, animation, reduced motion -->

[live] The lattice uses a 1200 by 240 field, a 24-pixel fine grid, a 96-pixel coarse grid, 12 seeded walks, and one group animation. Evidence: `web/src/components/LatticeBackdrop.tsx:24`, `web/src/components/LatticeBackdrop.tsx:31`.

Reduced-motion CSS disables animation and pins the sweep. The decorative SVG is hidden from assistive technology.

## Persistence And Scope
<!-- chunkId: workspace-appearance-reference-persistence-scope -->
<!-- keywords: browser preference, route scope, header band, local setting -->

[live] One browser-local preference selects the treatment for Workstream, Canvas, and Knowledge routes. Evidence: `web/src/features/settings/appSettings.ts:8`, `web/src/components/HeaderBackdrop.tsx:29`.

## Not supported
<!-- chunkId: workspace-appearance-reference-not-supported -->
<!-- keywords: synchronized preference, route override, custom geometry -->

[live] Server-synchronized preferences, per-route overrides, and user-defined lattice geometry are not supported. Evidence: `web/src/features/settings/appSettings.ts:8`, `web/src/components/HeaderBackdrop.tsx:29`.

## Known conflicts
<!-- chunkId: workspace-appearance-reference-known-conflicts -->
<!-- keywords: historical dimensions, validation limit, native interaction -->

[legacy] Earlier visual briefs contain superseded dimensions and walk parameters; the current source values above are authoritative. Source: `PM-041 (plan retired) | Visual brief`.

## Related pages

See [[workspace-appearance]], [[workspace-terminal-canvas]], and [[knowledge-index]].
