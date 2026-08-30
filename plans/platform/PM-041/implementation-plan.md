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

| Phase | Name                         | Track    | Status   |
|-------|------------------------------|----------|----------|
| F1    | Preference and variant type  | Frontend | Complete |
| F2    | Band contract generalisation | Frontend | Complete |
| F3    | Lattice variant              | Frontend | Complete |
| F4    | Settings control             | Frontend | Complete |
| F5    | Cross-variant verification   | Frontend | Complete |

## Terminology Lock

All code, CSS classes, custom properties and TS types must use:

- `BackdropVariant` with members `neon`, `lattice`, `none`
- `headerBackdrop` (not `backdropTheme`, not `bandStyle`)
- `backdropRoutes` (not `neonBandRoutes`)
- `.header-band`, `.header-band-{route}`, `.header-backdrop` (not `.neon-band`, `.neon-backdrop`)
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
