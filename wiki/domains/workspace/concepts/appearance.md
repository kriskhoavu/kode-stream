---
slug: workspace-appearance
title: Workspace Appearance
pageType: CONCEPT
topics: workspace, appearance, backdrop, accessibility, settings
summary: Workspace routes share a decorative header band whose treatment is user-selectable and motion-aware.
sourceRef: |
  PM-041 (plan retired) | Appearance model
  PM-041 (plan retired) | Frontend behavior
  PM-041 (plan retired) | Delivery
  PM-041 (plan retired) | Visual brief
  PM-041 (plan retired) | Validation approach
  PM-041 (plan retired) | Historical validation
lastVerified: 2026-09-12
---

## Shared Header Band
<!-- chunkId: workspace-appearance-shared-header-band -->
<!-- keywords: header backdrop, workspace routes, lattice, neon -->

[live] Workstream, Canvas and Knowledge share one decorative header band: `lattice` (default), `neon` or `none`. An invalid stored value falls back to `lattice`. The choice is browser-local and changes presentation only. Evidence: `web/src/components/HeaderBackdrop.tsx:25`.

## Lattice Behavior
<!-- chunkId: workspace-appearance-lattice-behavior -->
<!-- keywords: deterministic svg, junction, reduced motion -->

[live] The lattice is a deterministic SVG band, 240 pixels high, built once per module load and animated as one group. Reduced motion stops the animation. Evidence: `web/src/components/LatticeBackdrop.tsx:24`, `web/src/components/lattice-backdrop.css:134`.

## Not supported
<!-- chunkId: workspace-appearance-not-supported -->
<!-- keywords: per route variant, synchronization, density control -->

[live] Per-route variants, synchronization across browsers, and lattice color or density controls. Evidence: `web/src/features/settings/appSettings.ts:8`.

## Known conflicts
<!-- chunkId: workspace-appearance-known-conflicts -->
<!-- keywords: media emulation, radio keys, runtime check -->

[open] Has reduced motion been checked with browser media emulation, and have the radio keys been exercised? Only the styles were inspected.

## Related pages

See [[workspace-appearance-reference]].
