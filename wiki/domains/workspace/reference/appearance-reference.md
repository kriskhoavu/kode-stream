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

[live] Valid variants are `lattice`, `neon` and `none`. The default is `lattice`, and an invalid stored value becomes the default. One browser-local preference covers Workstream, Canvas and Knowledge. Evidence: `web/src/components/HeaderBackdrop.tsx:25`, `web/src/features/settings/appSettings.ts:8`.

## Geometry And Motion
<!-- chunkId: workspace-appearance-reference-geometry-motion -->
<!-- keywords: field geometry, seeded walk, animation, assistive technology -->

[live] The lattice uses a 1200 × 240 field, a 24-pixel fine grid, a 96-pixel coarse grid, 12 seeded walks and one group animation. Reduced-motion CSS stops the animation. The SVG is hidden from assistive technology. Evidence: `web/src/components/LatticeBackdrop.tsx:24`.

## Not supported
<!-- chunkId: workspace-appearance-reference-not-supported -->
<!-- keywords: synchronized preference, route override, custom geometry -->

[live] Server-synchronized preferences, per-route overrides and custom lattice geometry. Evidence: `web/src/components/HeaderBackdrop.tsx:100`.

## Known conflicts
<!-- chunkId: workspace-appearance-reference-known-conflicts -->
<!-- keywords: visual brief, dimensions, walk parameters -->

- "The visual brief's dimensions apply." The source values above are the authority.

## Related pages

See [[workspace-appearance]].
