# Design System

The stylesheet is `assets/brief.css`, inlined into `<style>` by `scripts/init_brief.sh`. Do not add
CSS to a brief. If a page needs a component that is not here, it probably needs prose instead.

## The page spine

Fixed. A run fills these sections; it does not invent, reorder, or drop them.

| # | Section | Markup | Filled from |
|---|---------|--------|-------------|
| 1 | Masthead | `header.masthead` → `.meta-row` + `h1` + `.standfirst` | `plan.yaml`, `README.md` heading |
| 2 | The short version | `section.sec` → `.eyebrow` + `h2` + two `p` + `.legend` | `README.md` overview and design decisions |
| 3 | Figures 1–N | `section.sec` → `figure` → `.frame` → `svg` + `figcaption` | `scenario/`, `design-0N-*.md` |
| 4 | Delivery | `section.sec` → `.phases` → `.phase` cards | `implementation-plan.md` |
| 5 | Reference | `section.sec` → `.scroll` → `table` | glossary, components, API contract |
| 6 | Scope | `section.sec` → `ul` | scope boundaries, `CLAUDE.md` ownership rules |
| 7 | Open decisions | `section.sec` → `.note.stop` per item | unresolved items |
| 8 | Provenance | `p.foot` | source directory, date, commit |

Sections 5, 6 and 7 may be omitted when the plan genuinely has nothing for them. Sections 1, 2, 3,
4 and 8 are mandatory — `check_brief.py` enforces 1, 2, 4 and 8.

## Colour

Three meaning-carrying accents, and nothing else:

| Token | Light | Dark | Typical meaning |
|-------|-------|------|-----------------|
| `--accent-a` / `--accent-a-tint` | `#2C7A6B` | `#57AD9B` | shared, existing, safe, done |
| `--accent-b` / `--accent-b-tint` | `#9C6A18` | `#D9A344` | per-instance, new, needs attention |
| `--accent-stop` / `--accent-stop-tint` | `#9E3B33` | `#D07A6E` | blocked, refused, drifted, undecided |

The rest of the palette is structural and carries no meaning: `--paper`, `--paper-2`, `--card`,
`--ink`, `--ink-2`, `--ink-3`, `--rule`, `--rule-soft`.

**The legend in section 2 is mandatory.** It states what each accent means *on this page*. Colour
without a legend is decoration, and decoration in a technical document is a lie waiting to happen.

Dark mode is defined three times over — `@media (prefers-color-scheme: dark)`,
`:root[data-theme="dark"]`, and `:root[data-theme="light"]` — so the page is correct as a local
file, in an IDE preview, and under a published artifact's theme toggle. Never hardcode a colour.

## Typography

- Headings, eyebrows, table headers, meta rows, pills and the footer are `var(--mono)`.
- Body copy is `var(--sans)` at 16px / 1.65.
- `h1` `clamp(28px, 4.4vw, 44px)`, `h2` `clamp(21px, 2.6vw, 27px)`, `h3` 16px.
- Prose is capped at `--measure` (68ch). Only `.frame` and `.scroll` may be wider.

## Components

**`.meta-row`** — four to five `<span><strong>key</strong> value</span>` pairs. Facts only:
ticket, service, tracks, status. Not a summary.

**`.standfirst`** — one paragraph, two or three sentences, saying what the page covers. It is also
the `description` when the page is published.

**`.eyebrow`** — the label above each `h2`. Use `Figure N` for figure sections; otherwise a single
word: `Delivery`, `Reference`, `Scope`, `Open`.

**`.legend`** with `.legend-key` and `.swatch.a` / `.swatch.b` / `.swatch.stop` — see Colour above.

**`figure` → `.frame` → `svg` → `figcaption`** — always all four. `.frame` supplies the card
background and horizontal scroll; `figcaption` opens with `<b>Figure N.</b>` and states the
takeaway, not a description of what is drawn.

**`.scroll` → `table`** — one table per page, in section 5. Cells may use `td.accent-a` /
`td.accent-b` to colour-code an ownership or status column, matching the legend.

**`.phases` → `.phase`** — one card per phase. `.phase-n` holds the phase id (`B1`, `F2`, `C1`).
`.phase.b` and `.phase.stop` switch the left border and number colour; use them only when the
legend gives those accents a meaning that applies to a phase, such as per-track colour. Inside,
`.phase-head` holds the `h3` and an optional `.pill`.

**`.pill`** — `.pill.done`, `.pill.active`, `.pill.blocked`, or a bare `.pill` for a neutral tag.
Status comes from `implementation-plan.md`; do not invent it.

**`.note`** — a constraint the reader would otherwise miss. Left border `--accent-b`.
**`.note.stop`** — blocked or undecided. Left border `--accent-stop`. Section 7 uses one per
decision. Two notes in a row usually means the second should be a sentence.

**`p.foot`** — source directory, date, commit, and the reminder that the markdown is the source of
truth.

## Not in the system

No JavaScript. No web fonts. No images. No icon sets. No gradients, shadows, or animation beyond
the one link-colour transition already in the stylesheet. No fourth colour. If a page seems to need
one of these, the content is wrong, not the system.
