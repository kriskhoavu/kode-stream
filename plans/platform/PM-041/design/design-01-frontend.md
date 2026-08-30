# Frontend Design: PM-041

## Preference

`AppSettings` gains one field, `headerBackdrop`, holding a `BackdropVariant`.

| Aspect        | Value                                                                     |
|---------------|---------------------------------------------------------------------------|
| Type          | `'neon' \| 'lattice' \| 'none'`                                             |
| Default       | `lattice`, flipped from `neon` in F3 once the variant exists              |
| Storage       | Existing `planManager.appSettings` object, under the `kodeStream.` prefix |
| Normalization | Any value not in the variant list becomes the default                     |
| Persistence   | The existing `useAppSettings` effect; no new write path                   |

`normalizeAppSettings` already discards unknown statuses rather than failing. The variant follows the same shape:
validate against the known list, fall back to the default, never throw.

## Components

```text
components/
├── HeaderBackdrop.tsx          # BackdropVariant, backdropRoutes, dispatcher
├── HeaderBackdrop.test.tsx
├── NeonBackdrop.tsx            # renamed from NeonHeaderBackdrop.tsx
├── NeonBackdrop.test.tsx
├── LatticeBackdrop.tsx
├── LatticeBackdrop.test.tsx
├── header-backdrop.css         # band box, height variable, fade mask
├── neon-backdrop.css
└── lattice-backdrop.css
```

`HeaderBackdrop` owns the variant union and the route list, so a variant is added by touching one module rather
than three. Each variant component renders only its own contents; the band box, its height and its bottom fade
belong to `header-backdrop.css` and are shared.

## Naming Migration

| Today                 | After                  | Files                                  |
|-----------------------|------------------------|----------------------------------------|
| `neonBandRoutes`      | `backdropRoutes`       | `App.tsx`, backdrop components, tests  |
| `.neon-band`          | `.header-band`         | `app-shell.css`, `App.tsx`             |
| `.neon-band-{route}`  | `.header-band-{route}` | `app-shell.css`, `App.tsx`             |
| `--neon-band-height`  | `--band-height`        | `app-shell.css`, `header-backdrop.css` |
| `.neon-backdrop`      | `.header-backdrop`     | `header-backdrop.css`                  |
| `.neon-backdrop-icon` | `.neon-icon`           | `neon-backdrop.css`                    |

`--neon-hue`, the per-icon custom property, keeps its name. It is genuinely neon-specific.

The stale comment in `features/canvas/canvas.css:1` refers to "the neon icon band" and needs the same update.

## Shell Wiring

The shell applies the band only when a variant will actually paint:

```text
showBand = variant !== 'none' AND route.name ∈ backdropRoutes
```

This single condition drives both the class list and whether `HeaderBackdrop` renders. Splitting it — hiding the
backdrop but keeping the class — leaves the topbar transparent with no bottom border and nothing behind it.

## Lattice Geometry

Generated once at module load by a seeded linear congruential generator, so the field is identical on every run
and reviewable by changing one seed constant.

| Parameter       | Value                                         |
|-----------------|-----------------------------------------------|
| Field viewBox   | 1200 × 360                                    |
| Fine grid       | 24px                                          |
| Coarse grid     | 96px                                          |
| Walks           | 10, one seeded per vertical band of the field |
| Steps per walk  | 7                                             |
| Step directions | 8 — four orthogonal, four diagonal            |
| Step length     | 1 to 4 fine cells                             |
| Node radius     | 2.2, or 3.2 on a coarse intersection          |

Generation, per walk:

```text
start at a grid point inside this walk's vertical band
repeat 7 times:
    pick a direction from the eight allowed
    pick a length of 1 to 4 cells
    shorten the step so both axes stay in the field, keeping the angle
    skip the step if no room is left in that direction
    emit an edge, record both endpoints as nodes
    the endpoint becomes the new position
```

Nodes are deduplicated by coordinate, so walks crossing at a shared point yield one node, not two.

Shortening rather than clamping is load-bearing. Clamping x and y independently keeps a diagonal step inside the
field but leaves it at an arbitrary angle, which is precisely how the network stops riding the grid.

## Lattice Motion

| Layer   | Animation                       | Period | Notes                                          |
|---------|---------------------------------|--------|------------------------------------------------|
| Grid    | Background position 0 → 96px    | 60s    | Linear, so the pan is seamless                 |
| Network | Translate 0 → 96px on both axes | 60s    | Same period, so the two stay locked            |
| Sweep   | Diagonal gradient across        | 14s    | Ease-in-out, alternating                       |
| Pulse   | Node radius and opacity         | 14s    | Matches the sweep; staggered per node by delay |

Under `prefers-reduced-motion: reduce` all four are set to `animation: none` and the sweep is additionally pinned
to its start transform, so it does not strand a bright diagonal across the band.

## Colour

Both variants read existing theme tokens. Lattice uses `--blue` for the grid and the network, and
`--button-accent` for the pulse. Grid lines mix to 14% and 6% for the coarse and fine rules; network strokes mix
to 34% and nodes to 62%, raised to 46% and 78% under `:root[data-theme="dark"]`.

No new tokens. Nothing in the band ever carries text, so contrast ratios do not apply to the backdrop itself;
what matters is that the page title, search field and filter chips stay legible above it, which the existing
bottom fade mask and the reduced grid opacity both serve.

## Settings Control

A new section in `SettingsPage`, following the existing `settings-section` structure used by Board View.

| Aspect      | Choice                                                                                       |
|-------------|----------------------------------------------------------------------------------------------|
| Group label | `Appearance`                                                                                 |
| Heading     | `Header backdrop`                                                                            |
| Control     | Radio group — the options are mutually exclusive, unlike the status checkboxes               |
| Options     | Lattice, Neon, None, each with a one-line description of what it paints                      |
| Copy        | Names what the band shows, and states that it appears on Workstream, Workbench and Knowledge |

A radio group rather than a select: three options, all worth reading, and the existing settings rows are already
label-plus-input pairs.

## Testing

| Target            | Assertions                                                                                                                                        |
|-------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| `appSettings`     | Default is `lattice`; unknown, absent and non-string values fall back; valid persists                                                             |
| `HeaderBackdrop`  | Dispatches to the right variant; renders nothing for `none`; `backdropRoutes` unchanged                                                           |
| `NeonBackdrop`    | Existing assertions, retargeted at the renamed class                                                                                              |
| `LatticeBackdrop` | Layer is `aria-hidden`; edges and nodes present; every node on a 24px multiple; every edge orthogonal or 45°; output identical across two renders |
| `SettingsPage`    | Three options render; selecting one calls `onChange` with that variant                                                                            |
| `App`             | Band class applied for a backdrop route with a variant; absent for `none`; absent off-route                                                       |

The geometric assertions are the ones worth writing: "every node sits on the grid and every edge is orthogonal or
diagonal" is the rule that makes the two layers read as one field, and it is the property a careless edit to the
generator would break.
