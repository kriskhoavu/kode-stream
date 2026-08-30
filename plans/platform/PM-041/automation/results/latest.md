# UI Automation Result: PM-041

Status: Passed

Run against the local dev server on 2026-08-30 at 1280x800, driving the browser directly rather than a scripted
playbook. Console clean throughout; no errors on any route under any variant.

## Variant switching

| Check                                              | Result                                                   |
|----------------------------------------------------|----------------------------------------------------------|
| Settings offers three radios, current one selected | Pass — `Lattice`, `Neon`, `None` under `Header backdrop` |
| Choosing a variant writes the preference           | Pass — persisted and survives a reload                   |
| Lattice paints on a backdrop route                 | Pass — 60 edges, 67 nodes, 11 accent junctions           |
| Neon still paints after the rename                 | Pass — 29 icons, unchanged band heights                  |

## Backdrop off

| Check                | Before (`lattice`)                             | After (`none`)              |
|----------------------|------------------------------------------------|-----------------------------|
| Shell class          | `app-shell header-band header-band-workstream` | `app-shell`                 |
| Backdrop element     | present                                        | absent                      |
| Topbar background    | `rgba(0, 0, 0, 0)`                             | `rgb(255, 255, 255) / 0.94` |
| Topbar bottom border | transparent                                    | `rgb(220, 228, 240)`        |
| Topbar blur          | `none`                                         | `blur(16px)`                |
| Left nav `z-index`   | `0`                                            | `auto`                      |

Every property the band class suppresses comes back. This is the check the plan was written around.

## Theme matrix

Both variants reviewed in light and dark on Workstream, Workbench and Knowledge. The page title, global search
and filter rows stay legible over both fields in both themes; the bottom fade mask clears the content below.

## Reduced motion

The media block ships with `animation-name: none` on the grid, network, pulse and sweep, and pins the sweep to
`translateX(-60%)`. Applying those declarations directly confirms the frozen field still reads as structure with
no bright diagonal stranded across the band.

The media query itself was not emulated — this harness cannot set `prefers-reduced-motion` — so the rule was
verified by inspecting the served stylesheet and applying its declarations, not by triggering it.

## Off-route unchanged

Settings under `lattice`: shell class is bare `app-shell`, no backdrop element, topbar keeps its own fill and
blur, left nav `z-index: auto`. Identical to its appearance under `none`.

## Keyboard

App chrome sits at `z-index: 2` above the backdrop, so focus indicators are never obscured by either field. The
variant radios are native inputs sharing one `name`, which is one tab stop with arrow navigation within it.
Arrow navigation could not be exercised through this harness, which does not trigger the native default action;
the pre-existing Storage option group behaves the same way under it.
