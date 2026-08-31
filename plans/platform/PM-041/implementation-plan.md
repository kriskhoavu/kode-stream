# Implementation Plan: PM-041 - Selectable Header Backdrop

## Overview

Add a `headerBackdrop` preference to `AppSettings`, generalise the neon-specific band contract into a
variant-agnostic one, add the Lattice variant behind it, and expose the choice in Settings. Lattice becomes the
default; Neon stays available; None turns the band off and restores the topbar's own chrome.

## Execution Flow

```text
F1 Preference and variant type ─┬─> F2 Band contract generalisation ─> F3 Lattice variant ─┐
                                │                                                          ├─> F5 Cross-variant pass
                                └─> F4 Settings control ───────────────────────────────────┘
```

F1 is pure: the variant union, the preference field and its normalization, with no UI. F2 is the rename and the
dispatcher — no visual change, and the existing neon field must look identical after it. F3 adds Lattice behind
the dispatcher F2 built. F4 needs only the variant type from F1, so it runs in parallel with F2 and F3. F5 is the
join: both themes, both variants, reduced motion, keyboard focus. Critical path is F1 → F2 → F3 → F5. No external
gates.

## Phases Summary

| Phase | Name                          | Track    | Status   |
|-------|-------------------------------|----------|----------|
| F1    | Preference and variant type   | Frontend | Complete |
| F2    | Band contract generalisation  | Frontend | Complete |
| F3    | Lattice variant               | Frontend | Complete |
| F4    | Settings control              | Frontend | Complete |
| F5    | Cross-variant verification    | Frontend | Complete |
| F6    | Monochrome lattice palette    | Frontend | Reverted |
| F7    | Flat ground under the lattice | Frontend | Complete |
| F8    | One band height               | Frontend | Complete |
| F9    | Topbar popups above content   | Frontend | Complete |
| F10   | Lit lattice junctions         | Frontend | Complete |
| F11   | Nav edge on the panel border  | Frontend | Complete |
| F12   | Band height to 240px          | Frontend | Complete |
| F13   | One orange for both themes    | Frontend | Complete |
| F14   | Brighter constellation        | Frontend | Complete |
| F15   | Natural field, real junctions | Frontend | Complete |

## Terminology Lock

All code, CSS classes, custom properties and TS types must use:

- `BackdropVariant` with members `neon`, `lattice`, `none`
- `headerBackdrop` (not `backdropTheme`, not `bandStyle`)
- `backdropRoutes` (not `neonBandRoutes`)
- `.header-band`, `.backdrop-{variant}`, `.header-backdrop` (not `.neon-band`, `.neon-backdrop`)
  (`.header-band-{route}` existed from F2 until F8 dropped it)
- `--band-height` (not `--neon-band-height`)
- `NeonBackdrop`, `LatticeBackdrop`, `HeaderBackdrop`

`--neon-hue` keeps its name; it is genuinely specific to the Neon variant.

> Lock this in before writing any code.

## Frontend Phases

### Phase F1: Preference And Variant Type

Pure and testable with no UI. The fallback behaviour is the part that matters: every existing install reaches this
code with the field absent.

The default stays `neon` here and flips in F3. Defaulting to `lattice` before the variant exists would leave the
band blank for two phases, which is not a state any phase should be able to ship in.

**Deliverables:**

- [x] `components/HeaderBackdrop.tsx` — export `BackdropVariant` and the variant list.
- [x] `features/settings/appSettings.ts` — `headerBackdrop` on `AppSettings`, defaulting to `neon`.
- [x] `normalizeAppSettings` validates the variant against the list and falls back to the default.
- [x] Test that absent settings yield the built-in default.
- [x] Test that an unknown string such as `"aurora"` falls back to the default.
- [x] Test that a non-string value falls back without throwing.
- [x] Test that each of the three valid variants round-trips through normalization.
- [x] Test that adding the field leaves `visibleWorkstreamStatuses` untouched.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/settings`

**Commit:** `PM-041: Add the header backdrop preference`

---

### Phase F2: Band Contract Generalisation

The rename plus the dispatcher. No visual change: Neon must look pixel-identical before and after, which is what
makes this phase safe to review separately from F3.

**Deliverables:**

- [x] Rename `NeonHeaderBackdrop.tsx` to `NeonBackdrop.tsx`, exporting `NeonBackdrop`.
- [x] Move `backdropRoutes` and the band box out of the neon module into `HeaderBackdrop.tsx` and
      `header-backdrop.css`.
- [x] `HeaderBackdrop` renders the variant it is given and nothing for `none`.
- [x] `neon-backdrop.css` keeps only neon styling; `.neon-backdrop-icon` becomes `.neon-icon`.
- [x] `app-shell.css` — `.neon-band` to `.header-band`, per-route classes and `--band-height` renamed.
- [x] `App.tsx` — compute `showBand` once from the variant and the route, driving both class list and render.
- [x] Update the stale neon comment in `features/canvas/canvas.css`.
- [x] Retarget the existing neon tests at the new names; keep every assertion.
- [x] Test that `HeaderBackdrop` renders nothing for `none`.
- [x] Test that `backdropRoutes` is still exactly `workstream`, `canvas`, `knowledge`.
- [x] Confirm no `neon-band` or `neonBandRoutes` reference remains outside the Neon variant itself.

**Verification:** `npm run typecheck && npm test -- --run web/src/components web/src/App.test.tsx`

**Commit:** `PM-041: Generalise the header band contract`

---

### Phase F3: Lattice Variant

The new treatment. Geometry is generated once at module load from a seeded generator; the geometric invariants are
the acceptance criteria, since they are what make the grid and the network read as one field.

**Deliverables:**

- [x] `components/LatticeBackdrop.tsx` — seeded walk generator, evaluated once at module load.
- [x] Ten walks, one seeded per vertical band, seven steps each, one to four cells per step.
- [x] Nodes deduplicated by coordinate; coarse-intersection nodes marked for the pulse.
- [x] `components/lattice-backdrop.css` — grid gradients, network strokes, sweep, pulse, dark-theme mixes.
- [x] Grid pan and network drift share the 60s period; sweep and pulse share the 14s period.
- [x] Reduced motion stops all four animations and pins the sweep to its start transform.
- [x] Test that the layer is `aria-hidden` and renders edges and nodes.
- [x] Test that every node coordinate is a multiple of 24.
- [x] Test that every edge is orthogonal or exactly 45°.
- [x] Test that no edge has zero length.
- [x] Test that two renders produce identical geometry.
- [x] Test that at least one node carries the pulse class.
- [x] A diagonal step is shortened to fit the field, never clamped per axis, which would bend its angle.
- [x] Flip `defaultBackdropVariant` to `lattice`, now that the variant it names exists.
- [x] Update the F1 default test, which asserts the constant rather than a literal.

**Verification:** `npm run typecheck && npm test -- --run web/src/components`

**Commit:** `PM-041: Add the lattice header backdrop`

---

### Phase F4: Settings Control

Depends only on F1. The copy carries real weight here, because Settings is not a backdrop route and the control
therefore has no live preview to explain itself.

**Deliverables:**

- [x] `pages/SettingsPage.tsx` — an Appearance section with a radio group over the three variants.
- [x] Each option names what it paints in one line.
- [x] Section copy states that the backdrop appears on Workstream, Workbench and Knowledge.
- [x] Radio group styling reuses `settings-section` and `settings-toggle-row`; add rules only where the radio
      differs from the existing checkbox rows.
- [x] Test that three options render with the current one selected.
- [x] Test that selecting an option calls `onChange` with that variant and preserves the other settings.
- [x] Keyboard: native radios sharing one `name`, so the group is one tab stop and arrows move within it.
      Arrow navigation could not be exercised through the browser automation layer, which does not trigger the
      native default action; the pre-existing Storage option group behaves identically under it.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/SettingsPage.test.tsx`

**Commit:** `PM-041: Add the header backdrop setting`

---

### Phase F5: Cross-Variant Verification

The join. Six combinations that no unit test covers: two variants across two themes, plus off, plus reduced
motion.

**Deliverables:**

- [x] Lattice and Neon reviewed in light and dark on all three backdrop routes.
- [x] Confirm the page title, global search and filter chips stay legible over both variants.
- [x] Confirm None restores the topbar fill, blur and bottom border, and the left nav stacking.
- [x] Confirm no route outside `backdropRoutes` changed under any variant.
- [x] Confirm reduced motion freezes both variants with the sweep at its start.
- [x] Confirm keyboard focus rings remain visible over both variants.
- [x] Record the pass in `automation/results/latest.md`.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Verify both backdrops across themes and motion settings`

---

### Phase F6: Monochrome Lattice Palette — Reverted by F7

Review pass, and a misdiagnosis. The lattice read as orange, and `--blue` being `#f9b98c` in the dark palette
looked like a sufficient explanation, so the whole field was redrawn in `--text`.

It was the wrong layer. The orange came from `.main-content`'s corner glows behind the band, which read from the
same token; the lattice's own blue was working, including in light theme where F6 discarded it for no reason.
Left recorded rather than deleted, because the token trap it names is real and the next reader should see both
the trap and the mistake it caused.

**Deliverables:**

- [x] Draw the whole field from `--text`, the one token that stays neutral ink in both themes.
- [x] Coarse junctions keep their emphasis through size and full ink rather than colour.
- [x] Separate dark-theme mixes for grid, strokes, nodes and pulse: ink on a near-black ground carries more
      weight per percent than ink on white.
- [x] Confirm every computed lattice colour is equal-RGB neutral in dark and the app's own ink in light.
- [x] Re-check both themes on all three backdrop routes.

**Verification:** `npm run typecheck && npm test -- --run web/src/components`

**Commit:** `PM-041: Draw the lattice in neutral ink`

---

### Phase F7: Flat Ground Under The Lattice

Reverses F6. The orange was never in the lattice: it is `.main-content`'s two corner glows, which read from
`--blue` and so go peach in dark theme. F6 recoloured the wrong layer, and lost the blue that was working.

The glows suit Neon, whose icons are themselves soft points of colour — the wash reads as more of the same. The
lattice is the opposite kind of mark: thin, even and structural. An uneven colour wash behind it makes the grid
look unevenly lit rather than lit at all.

The shell already knew the route; it now names the variant too, so the page beneath the band can respond to the
treatment above it.

**Deliverables:**

- [x] Restore the lattice palette from F3 — `--blue` for the field, `--button-accent` for coarse junctions.
- [x] `bandClasses` replaces `paintsBand`, returning the full class string including `backdrop-{variant}`.
- [x] `App.tsx` derives both the class list and the render from that one string.
- [x] `.app-shell.backdrop-lattice .main-content` drops to flat `--bg`.
- [x] Neon and every non-band route keep the glows untouched.
- [x] Test that the band class names both the route and the variant, and is empty where nothing paints.
- [x] Confirm in both themes that the lattice sits on flat ground and Neon still has its wash.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Give the lattice flat ground`

---

### Phase F8: One Band Height

The band ran 300 to 430px depending on route, inherited from the neon field's need to clear each page's content.
It read as too tall, and the per-route variation read as arbitrary once anyone noticed it.

Both go. One `200px` band on every backdrop route, which reaches the filter row and stops. The mask fades the
lower third to nothing at any height, so the extra was paint area with nothing visible in it.

**Deliverables:**

- [x] `--band-height: 200px` set once on `.app-shell.header-band`. F12 later raised it to 240px.
- [x] Remove the `header-band-knowledge` and `header-band-canvas` overrides.
- [x] `bandClasses` stops emitting `header-band-{route}`; nothing consumed it once the overrides were gone.
- [x] Update the band class test to assert the variant and the absence of the route.
- [x] Confirm the same 200px band on Workstream, Workbench and Knowledge, both variants, both themes.
- [x] Correct the design document's Colour section, which still described the reverted F6 palette because an
      F7 edit silently failed to match after table reformatting.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Settle the band at one height`

---

### Phase F9: Topbar Popups Above Content

Bug fix, reported against the workspace switcher: opening it drew the menu behind the page and the page took its
clicks.

The band gives the topbar `position: relative; z-index: 2`, which makes it a stacking context. Every popup
inside it is then capped at the topbar's own rank no matter what it asks for — the workspace menu asks for 35 and
gets 2. Page content is also 2 and comes later in the DOM, so it wins both the paint and the hit test.

Predates PM-041: `dfe3e7d` introduced the rule with the original neon band. It reproduces under Neon and Lattice
alike, on all three backdrop routes, and never under `none`, which applies no band class and so no stacking
context.

The topbar moves to 3. The two are separate grid rows and never overlap as blocks, so outranking page content
costs nothing.

**Deliverables:**

- [x] `.app-shell.header-band .topbar` z-index 2 to 3.
- [x] Rewrite the shell's stacking comment to record the order and the stacking-context trap behind it.
- [x] Verify by hit test, not by eye: `elementFromPoint` over the open menu must land inside the menu.
- [x] Confirm on Workstream, Workbench and Knowledge, under both variants and under `none`.
- [x] Confirm the profile menu, which shares the trap, and the search dialog, which does not.
- [x] Confirm the topbar keeps its transparent fill under a band — raising the rank must not restore its chrome.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Lift the topbar above page content`

---

### Phase F10: Lit Lattice Junctions

Review pass. The nodes were flat fills, so the field read as drawn rather than lit and the sweep had nothing to
catch.

**Deliverables:**

- [x] Group the nodes in one `<g>` and hang a single halo filter on it, rather than one filter per circle.
- [x] Coarse junctions carry a second, brighter halo that swells and fades on the sweep's own 14s period.
- [x] Wider halo in dark theme, where a glow carries further than it does on white.
- [x] Reduced motion pins the junction halo open instead of freezing it mid-breath.
- [x] Coarse node radius 3.2 to 3.4, so the brighter node reads as a node and not just as light.

**Verification:** `npm run typecheck && npm test -- --run web/src/components`

**Commit:** `PM-041: Light the lattice junctions`

---

### Phase F11: Nav Edge On The Panel Border

The left nav drew its right edge from a darkened `--nav`, which is invisible against a near-black nav in dark
theme. With the band crossing between nav and page, the two merged into one another.

It now uses `--line`, the border every other panel and card in the app already reads from. The nav stays
full-bleed and square: a radius only reads as one on an inset panel, and insetting the nav is a layout change
this ticket has no reason to make.

**Deliverables:**

- [x] `.left-nav` `border-right` reads from `--line`.
- [x] Confirm the edge is visible in both themes and that the nav keeps its full-height, flush-to-edge geometry.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Rule the nav edge with the panel border`

---

### Phase F12: Band Height To 240px

Follow-up to F8, which had cut the band from 300–430px to 200px. That went slightly too far: the fade landed
through the filter row rather than below it.

240px, still one height for every route, and a whole ten 24px lattice cells so the grid ends on a rule.

**Deliverables:**

- [x] `--band-height` 200px to 240px, and the `header-backdrop.css` fallback with it.
- [x] Confirm on Workstream, Workbench and Knowledge under both variants.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Reach the band to 240px`

---

### Phase F13: One Orange For Both Themes

Reported: the network and its dots were purple in light theme, and too heavy in both.

Purple because the field read from `--blue`, which is `#4f46e5` indigo in light and `#f9b98c` peach in dark — the
lattice changed hue with the theme and only looked orange on one side. F6 had misread the same token from the
other direction and been reverted for it; this is the same trap, found from light theme instead of dark.

`--orange` is orange on both sides, and its dark value is identical to the peach the lattice already used, so
dark keeps its exact hue and only light changes. Every layer drops in weight at the same time.

Junctions move to `--orange` too. They had been `--button-accent`, whose dark `#b95f2e` sits darker than the
network it was meant to accent.

**Deliverables:**

- [x] Every lattice layer reads `--orange`; no `--blue` or `--button-accent` left in the variant.
- [x] Lighter mixes throughout: edges 34 to 24 percent light and 46 to 30 dark, nodes 62 to 44 and 78 to 52.
- [x] Scope the dark node rule `:not(.lattice-pulse)`. A junction carries both classes and the themed selector
      outranks `.lattice-pulse`, so full-ink junctions had never applied in dark theme since F3.
- [x] Verify computed fills, not appearance: junction fill must differ from node fill in both themes.
- [x] Confirm no purple remains in light theme.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Draw the lattice in one orange`

---

### Phase F14: Brighter Constellation

F13 lightened the field by dropping alpha, which made it fainter rather than brighter — the opposite of what was
wanted. Brightness on a dark ground comes from a paler ink, not a thinner one.

Introduces `--lattice-ink`, namespaced in the variant's own stylesheet: `--orange` in light, and `--orange`
lifted 34% toward white in dark. Every layer derives from it, so hue and weight tune independently.

**Deliverables:**

- [x] `--lattice-ink` defined per theme; every layer reads it, no direct `--orange` uses left in the variant.
- [x] Dark: edges 30 to 52 percent, nodes 52 to 88, halo 45 to 65 and one pixel wider.
- [x] Junction halo opens wider at the top of its breath, and its floor rises from 0.45 to 0.6 opacity.
- [x] Light gets a small lift only — edges to 30 percent, nodes to 52 — after a first pass at 38 and 66 read as
      bold and busy on white.
- [x] Verify computed inks and fills per theme, not appearance alone.
- [x] Confirm the page title, search field and filter rows stay legible over the brighter field in both themes.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Brighten the constellation`

---

### Phase F15: Natural Field, Real Junctions

Reported: the walks looked cut off as they drifted downwards, the variety was poor, and the field wanted better
highlights and more vibrant colour. The first of those turned out to be two bugs rather than styling.

**The field was hard-clipped.** The viewBox was 1200 × 360 inside a 240px band, so `slice` rendered it 384px tall
and clipped 40% of it with no fade. Every walk crossing that boundary ended mid-edge.

**The network never rode the grid.** The grid was a CSS background at device scale; the network was an SVG scaled
by `slice`. At 1280px wide the SVG scaled 1.067, putting its 24-unit spacing at 25.6px against a 24px CSS grid.
The premise only held at exactly 1200px band width.

**Deliverables:**

- [x] Move the grid into the SVG as two nested patterns, in the same group as the network, moved by one
      transform. Grid and network are now locked by construction, not by two matching animations.
- [x] Field height 360 to 240, matching the band, so `slice` has little left to crop.
- [x] Fade each walk along its length, and fade the field's top and bottom 56 units, so a crop never cuts hard.
- [x] Walk length varies from 4 to 10 steps; 12 walks rather than 10.
- [x] Junctions are nodes where three or more edges meet, or coarse-grid landmarks — 9 rather than 3, and they
      land where the drawing is dense. Radius grows with degree.
- [x] Dark ink lift 34% to 14% for saturation, with higher alphas carrying the brightness instead.
- [x] Junction core lifted toward white, so the highlight reads as light rather than a second hue.
- [x] Tests for the fades, the opacity range, the varied walk length, the junction rule and degree-based radius.
- [x] Confirm reduced motion still freezes the field with the sweep pinned off-field.

**Verification:** `npm run typecheck && npm test && npm run build`

**Commit:** `PM-041: Let the lattice fade instead of stopping`

## Post-Implementation Checklist

- [x] Full frontend suite: `npm run typecheck && npm test && npm run build`
- [x] No terminology drift: `grep -rn "neonBand\|neon-band\|NeonHeaderBackdrop" web/src`
- [x] Update PM-041 documents if naming drifted during implementation.
- [x] Re-render `brief.html` if the plan changed.
- [x] Keep phase commits separate.

## Testing Strategy

Unit tests carry the two things that break silently. First, preference normalization: every existing install
arrives with the field missing, and a fallback that throws or yields `undefined` blanks the band with no error
anyone would notice. Second, the lattice invariants — node on grid, edge orthogonal or 45°, no zero-length edge,
stable across renders. Those four properties are the difference between a lattice and a scatter of dots, and a
plausible-looking edit to the generator can break them while the component still renders something.

What unit tests cannot carry is whether the result is legible, which is why F5 is a phase rather than a checklist
item. Two variants, two themes, three routes and a reduced-motion pass is a matrix worth walking deliberately.

The Neon tests are retargeted rather than rewritten in F2. Keeping their assertions intact is the evidence that
the rename changed names and nothing else.

## Migration Notes

None required. `AppSettings` is read through `normalizeAppSettings`, which already tolerates partial stored
objects, so an install with no `headerBackdrop` picks up the default on first load and persists it on the next
write. Nothing else reads the key.

The visible change is that existing installs move from Neon to Lattice without asking. That is intended, and it is
reversible in two clicks from Settings.
