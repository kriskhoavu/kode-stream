# PM-041: Selectable Header Backdrop

The decorative band running from the topbar down through each page's filter rows is currently one fixed
treatment: a field of 29 neon icons, each drifting and pulsing behind its own pair of animated `drop-shadow`
filters. It was added as a single hardcoded component with no way to change or disable it.

PM-041 turns that one treatment into a chosen one. It adds a second variant, Lattice, makes Lattice the default,
keeps Neon available, and adds a third option that turns the band off entirely. The choice lives in Settings and
persists per browser like every other app preference.

The band contract itself — which routes carry it, how tall it is per route, how it stacks against the sticky left
nav — is unchanged. Only its contents become pluggable.

## Related Plans

| Ticket                        | Relationship     | Key Context                                                                    |
|-------------------------------|------------------|--------------------------------------------------------------------------------|
| [PM-040](../PM-040/README.md) | Adjacent surface | Knowledge is one of the three backdrop routes; its palette work sets the tone. |

The neon band arrived in commit `dfe3e7d` outside the plan flow, so PM-041 is the first document to describe it.

## Scope

In scope: the Lattice variant, the variant registry and preference, generalising the band contract away from
neon-specific names, the Settings control, and an off switch.

Out of scope, and deliberately so:

- Per-route variants. The choice is one setting for the whole app.
- Making the band appear on routes that do not carry it today.
- A colour or density control for either variant. Both read from existing theme tokens.
- Syncing the choice across browsers. It is a local preference, like `theme`.

## Glossary

| Term             | Meaning                                                          | Maps To (code)    |
|------------------|------------------------------------------------------------------|-------------------|
| Header Backdrop  | The decorative layer filling the band below the topbar           | `HeaderBackdrop`  |
| Backdrop Variant | Which treatment the band paints: `neon`, `lattice`, or `none`    | `BackdropVariant` |
| Band             | The masked region the backdrop occupies, sized per route         | `.header-band`    |
| Backdrop Route   | A route whose shell carries the band                             | `backdropRoutes`  |
| Neon             | Scattered icon field, per-icon hue, drift and glow pulse         | `NeonBackdrop`    |
| Lattice          | Blueprint grid with an on-grid node network, panning as one body | `LatticeBackdrop` |
| Walk             | One path of grid-aligned steps; the nodes are its turns          | `latticeWalks`    |

`Band` and `Backdrop` are distinct on purpose. The band is the geometry and the stacking context, and it does not
change in this ticket. The backdrop is what fills it, and that is the whole feature.

## Components

| Layer      | Component                          | Purpose                                                        |
|------------|------------------------------------|----------------------------------------------------------------|
| Preference | `features/settings/appSettings.ts` | Carry and normalize `headerBackdrop`                           |
| Shell      | `components/HeaderBackdrop.tsx`    | Variant type, `backdropRoutes`, dispatch to the chosen variant |
| Variant    | `components/NeonBackdrop.tsx`      | Existing icon field, moved behind the dispatcher               |
| Variant    | `components/LatticeBackdrop.tsx`   | Grid, node network, and sweep                                  |
| Geometry   | `components/header-backdrop.css`   | Band box, height variable, bottom fade mask                    |
| Style      | `components/lattice-backdrop.css`  | Grid gradients, lattice strokes, sweep and pulse timing        |
| Shell      | `App.tsx`, `styles/app-shell.css`  | Apply the band class only when a variant paints                |
| Settings   | `pages/SettingsPage.tsx`           | The control                                                    |

## Data Flow

```text
localStorage kodeStream.planManager.appSettings
    ↓
loadAppSettings → normalizeAppSettings → headerBackdrop
    ↓
App: variant === 'none' or route not in backdropRoutes?
    ├── yes → no band class, topbar keeps its own fill, nothing rendered
    └── no  → shell gets .header-band .header-band-{route}
              ↓
              HeaderBackdrop dispatches on variant
              ├── neon    → NeonBackdrop
              └── lattice → LatticeBackdrop
```

## The Lattice

The grid and the node network are one field rather than two stacked layers, and three rules are what make them
read that way.

**Nodes sit on the grid.** The network is built as ten walks stepping along the 24px grid — orthogonal or 45° only,
one to four cells per step, with a node at every turn. Every edge therefore lies on a grid line or its diagonal.

**It drifts as one body.** The whole network translates 96px over 60s, exactly matching the grid's own pan, so the
two move locked together. Per-node drift, as the standalone constellation used, tears edges away from their
endpoints.

**The sweep is the shared beat.** One diagonal light pass crosses both layers on a 14s cycle, and the node pulse
runs on the same period. Only nodes landing on a coarse 96px intersection carry that pulse, so the emphasis marks
real structure rather than arbitrary points.

The field is monochrome, drawn from `--text` throughout. Colour is what the grid and the network least need, and
the obvious token for it is a trap: `--blue` is `#f9b98c` in the dark palette, so reading from it turned the whole
lattice orange.

One walk is seeded per vertical band of the field, so coverage reaches the left nav, the page title and the right
edge instead of clumping.

## Design Decisions

| Decision                                    | Alternatives Considered                    | Rationale                                                                                                |
|---------------------------------------------|--------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Lattice becomes the default                 | Keep Neon default; no default change       | The ticket exists because Lattice is the wanted look. Neon stays one click away.                         |
| Three variants including `none`             | Two variants plus a separate on/off toggle | Off is a third answer to the same question. A separate toggle creates a meaningless off-plus-Neon state. |
| Preference on `AppSettings`                 | A bare `theme`-style string preference     | Settings owns the control, and `AppSettings` is the bag Settings already writes.                         |
| `none` drops the band class entirely        | Render the band empty                      | The band class also strips the topbar fill, blur and border. An empty band leaves the topbar unstyled.   |
| Rename to `.header-band` / `backdropRoutes` | Keep the `neon` names for all variants     | A Lattice route class reading `neon-band-canvas` is a lie the next reader has to decode.                 |
| Lattice geometry from a seeded generator    | A committed literal array, as Neon uses    | The lattice's meaning is in the walk structure; a flat list of 70 coordinates hides exactly that.        |
| Generate once at module load                | Generate per render                        | Matches Neon's stability guarantee: the field must not reshuffle when the shell re-renders.              |
| Both variants freeze under reduced motion   | Hide the backdrop under reduced motion     | Static Lattice still reads as structure; removing it changes layout weight for no accessibility gain.    |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Frontend Design](design/design-01-frontend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
- [Visual Brief](brief.html)
