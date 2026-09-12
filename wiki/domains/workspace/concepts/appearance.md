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

[live] Workstream, Canvas, and Knowledge routes share one decorative header band with `lattice`, `neon`, and `none` variants; invalid stored values fall back to lattice. Evidence: `web/src/components/HeaderBackdrop.tsx:14`, `web/src/components/HeaderBackdrop.tsx:20`.

The setting is browser-local and the backdrop is presentation-only. Navigation and page content remain above it in the stacking order.

## Lattice Behavior
<!-- chunkId: workspace-appearance-lattice-behavior -->
<!-- keywords: deterministic lattice, svg animation, junction, reduced motion -->

[live] The lattice is a deterministic SVG generated once per module load and animated as one group; reduced-motion styles stop the animation and pin the sweep. Evidence: `web/src/components/LatticeBackdrop.tsx:24`, `web/src/components/lattice-backdrop.css:134`.

The implemented band is 240 pixels high. Junction emphasis is derived from connected paths rather than separate interactive controls.

## Not supported
<!-- chunkId: workspace-appearance-not-supported -->
<!-- keywords: per route variant, cloud synchronization, density control -->

[live] Per-route variants, cross-browser setting synchronization, and user controls for lattice color or density are not supported. Evidence: `web/src/components/HeaderBackdrop.tsx:29`, `web/src/features/settings/appSettings.ts:8`.

## Known conflicts
<!-- chunkId: workspace-appearance-known-conflicts -->
<!-- keywords: historical brief, band height, reduced motion test -->

[open] Has reduced motion been verified through browser media emulation and have native radio-key interactions been exercised? The retired validation inspected styles but did not complete those runtime checks.

## Related pages

See [[workspace-appearance-reference]], [[workspace-terminal-canvas]], and [[knowledge-index]].
