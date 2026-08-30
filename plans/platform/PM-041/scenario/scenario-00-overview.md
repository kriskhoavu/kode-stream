# Scenarios: PM-041 Overview

## Scenario List

| #   | Title                    | Description                                                        |
|-----|--------------------------|--------------------------------------------------------------------|
| 0   | Neon only                | Default state before the feature                                   |
| 1   | Upgrade to Lattice       | An existing install opens the app after the change                 |
| 2   | Switching variants       | Choosing a different backdrop in Settings                          |
| 3   | Turning the backdrop off | Choosing None, and what happens to the topbar                      |
| 4   | Routes without a band    | Settings, terminal and other routes are unaffected by every choice |
| 5   | Reduced motion           | The chosen variant renders without animation                       |

---

# Scenario 0: Neon Only (Default State)

## Starting State

- No `headerBackdrop` value exists in `kodeStream.planManager.appSettings`.
- Workstream, Workbench and Knowledge paint the neon icon field. Every other route paints nothing.
- Settings offers no control over the band.

## Visual State (Before)

```text
┌──────────────────────────────────────────────────────────┐
│ ✦  ⚙   [ search ]        ◇        ⚡      ⬡      ✦        │  ← icons drift and pulse
│  ⬡    Workstream                     ⚙        ◇          │
│ ⚡  [All services] [In flight] [Blocked]   ✦      ⬡       │
├──────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────┐  │
│  │ PM-041 · Selectable header backdrop                │  │
```

## Available Actions

| Action | Description                  | Flow |
|--------|------------------------------|------|
| None   | The band is not configurable | —    |

## Edge Cases

- The band is present but unnamed anywhere in Settings, so a user who dislikes it has no recourse.

---

# Scenario 1: Upgrade To Lattice

## Starting State

An install that has used the app before. Its stored `AppSettings` has `visibleWorkstreamStatuses` and no
`headerBackdrop`.

## Flow

Normalization sees the missing field and fills the default, `lattice`. The next paint shows the grid and node
network instead of the icon field. Nothing else in the stored settings is touched, and the value is written back
on the same effect that already persists `AppSettings`.

## Visual State (After)

```text
┌──────────────────────────────────────────────────────────┐
│ ·──┬───·  [ search ]   ·───┬──·          ╱·               │  ← walks ride the grid
│ ┆  ·  ┆ Workstream   ┆    ●    ┆       ╱  ┆     ← ● pulses on coarse nodes
│ ·──┴─[All services] [In flight] [Blocked] ·───·           │
├──────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────┐  │
│  │ PM-041 · Selectable header backdrop                │  │
```

## Edge Cases

- Stored settings holding an unknown string such as `"aurora"` fall back to `lattice` rather than rendering nothing.
- Stored settings holding a non-string fall back the same way. Normalization never throws.

---

# Scenario 2: Switching Variants

## Starting State

Any backdrop route, with Lattice active.

## Flow

The user opens Settings, picks Neon, and returns to Workstream. The band now paints the icon field. The choice
survives a reload.

## Available Actions

| Action         | Description                      | Flow                                          |
|----------------|----------------------------------|-----------------------------------------------|
| Choose Neon    | Select the icon field            | Setting written, band repaints on next render |
| Choose Lattice | Select the grid and node network | Setting written, band repaints on next render |
| Choose None    | Turn the band off                | Band class dropped, see Scenario 3            |

## Edge Cases

- Switching while on Settings itself shows no immediate change, because Settings is not a backdrop route. The
  control must therefore describe what it does rather than relying on a live preview.
- Neither variant reshuffles its geometry on switch-away and switch-back. Both are generated once at module load.

---

# Scenario 3: Turning The Backdrop Off

## Starting State

Any backdrop route with a variant active.

## Flow

The user picks None. The shell stops applying `header-band`, so the backdrop is not rendered *and* the topbar
recovers its own background, blur and bottom border — all three of which the band class suppresses.

## Visual State (After)

```text
┌──────────────────────────────────────────────────────────┐
│    [ search ]                                            │  ← opaque, blurred, ruled off
├──────────────────────────────────────────────────────────┤
│ Workstream                                               │
│ [All services] [In flight] [Blocked]                     │
```

## Edge Cases

- The left nav must return to its normal stacking. The band class lowers it to `z-index: 0` so the backdrop can
  paint across it; with no band that override must not apply.
- `.main-content` children must not keep the `z-index: 2` lift, which exists only to sit above the backdrop.

---

# Scenario 4: Routes Without A Band

## Starting State

Settings, or any route outside `backdropRoutes`.

## Flow

No band is applied regardless of the chosen variant. The topbar renders normally. Choosing Neon or Lattice
changes nothing on these routes.

## Edge Cases

- `backdropRoutes` must stay exactly `workstream`, `canvas`, `knowledge`. The rename from `neonBandRoutes` must
  not quietly widen it.

---

# Scenario 5: Reduced Motion

## Starting State

`prefers-reduced-motion: reduce` is set at the OS level. Either variant is active.

## Flow

The backdrop paints in full but does not animate. Neon icons hold their placement with no drift or pulse. Lattice
holds its grid offset, its network position and its sweep, so the structure is visible and still.

## Edge Cases

- The sweep is a positioned gradient, not an opacity fade. Frozen mid-travel it would leave a bright diagonal
  across the band, so it must be frozen at its start position rather than simply having its animation removed.
