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

| Today                 | After              | Files                                  |
|-----------------------|--------------------|----------------------------------------|
| `neonBandRoutes`      | `backdropRoutes`   | `App.tsx`, backdrop components, tests  |
| `.neon-band`          | `.header-band`     | `app-shell.css`, `App.tsx`             |
| `.neon-band-{route}`  | dropped in F8      | `app-shell.css`, `App.tsx`             |
| `--neon-band-height`  | `--band-height`    | `app-shell.css`, `header-backdrop.css` |
| `.neon-backdrop`      | `.header-backdrop` | `header-backdrop.css`                  |
| `.neon-backdrop-icon` | `.neon-icon`       | `neon-backdrop.css`                    |

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

Worth knowing before reaching for `--blue` anywhere near this band: it is `#f9b98c` in the dark palette, a peach
rather than a blue. That is fine inside the lattice, where it reads as a warm blueprint, and not fine behind it.

### Lit, not drawn

Nodes carry a halo, or the sweep passes over flat fills and the field reads as a diagram. One `drop-shadow` on a
group wrapping every node, so the whole set costs one filter region rather than sixty. Coarse junctions add a
second, brighter halo animated on the sweep's 14s period — the light and the thing it lights share a clock.

Dark theme takes a wider halo than light: a glow carries further on a near-black ground, and on white the same
radius reads as a smudge rather than a shine.

### The ground beneath

`.main-content` paints two symmetric corner glows, also from `--blue`. Under Neon they read as more of the same,
since its icons are themselves soft points of colour. Under the lattice they do not: a thin, even, structural
mark on an uneven colour wash looks unevenly lit rather than lit.

`.app-shell.backdrop-lattice .main-content` therefore drops to flat `--bg`. That scoping is why the shell carries
a `backdrop-{variant}` class at all — the page beneath the band has to know which treatment is above it.

No new tokens. Nothing in the band ever carries text, so contrast ratios do not apply to the backdrop itself;
what matters is that the page title, search field and filter chips stay legible above it, which the bottom fade
mask and the low grid opacity both serve.

## Nav Edge

`.left-nav` draws its right edge from `--line`, the border token every panel and card in the app shares. It
previously used a darkened `--nav`, which is invisible against a near-black nav in dark theme — with the band
crossing between the nav and the page, the two merged.

The nav stays full-bleed and square-cornered. A 10px radius only reads as rounded on a panel inset from the
viewport edges, and insetting the nav is a layout change with no bearing on the backdrop.

## Band Height

One height, `240px`, for every backdrop route. `header-backdrop.css` reads it through `--band-height`, set once
on `.app-shell.header-band`.

The band used to run 300 to 430px and vary by route, so the neon icon field would clear whatever each page put
below the topbar. Two things made that unnecessary. The mask fades the band's lower third to nothing whatever its
height, so most of the extra was paint area with nothing visible in it; and a band that changes height as you
move between Workstream, Workbench and Knowledge is a difference nobody asked the UI to express.

240px is a whole ten 24px lattice cells, so the grid ends on a rule rather than mid-cell. F8 first cut the band
to 200px; F12 restored a little of that reach, which is what puts the fade below the filter row instead of
through it.

`.header-band-{route}` is gone with the per-route heights. Nothing else ever styled a band by route, and an
emitted class no rule consumes is dead weight. Bringing per-route heights back is one rule and one template
literal if a page ever genuinely needs a different band.

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
