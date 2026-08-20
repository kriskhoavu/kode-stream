# Figure Grammar

Read this before writing any SVG. Copy geometry from `assets/starter.html` rather than inventing it.

SVG text does not wrap. A label that is too long for its card overflows silently — nothing errors,
nothing reflows, the page just looks wrong. Almost every rule below exists to prevent that.

## Choosing figures

Three figures minimum, five maximum. A figure earns its place only if it shows something a sentence
cannot. Before drawing, write down what each figure **proves**. If the answer is "it shows the
components", that is a table, not a figure.

| Type | Use when the point is | Starter figure |
|------|-----------------------|----------------|
| Layered topology | who talks to whom, and what must never talk to what | Figure 1 |
| Before / after split | this is exactly what changes | Figure 2 |
| Left-to-right pipeline | inputs go in, this comes out, a gate checks it | Figure 3 |
| Numbered flow | these steps happen in this order, and this one can refuse | Figure 4 |

Do not draw two figures of the same type on one page. If two things want the same type, they are
probably one figure.

## Canvas

```
<svg viewBox="0 0 1020 {H}" role="img" aria-label="{one sentence}">
```

- Width is always `1020`. Height is set to the lowest element's bottom edge + 30.
- Content lives in `x ∈ [30, 990]`. Nothing crosses those lines.
- `role="img"` and a real `aria-label` are required — one sentence saying what the figure shows,
  not "diagram".
- The `.frame` wrapper gives the SVG `min-width: 640px` and scrolls horizontally on narrow screens,
  so the figure never has to be responsive. Do not add media queries inside SVG.

## Grid

Column stops, 20px gutters, all inside `[30, 990]`:

| Layout | x stops | width |
|--------|---------|-------|
| Full | 30 | 960 |
| Half | 30, 520 | 470 |
| Third | 30, 357, 684 | 306 |
| Quarter | 30, 275, 520, 765 | 225 |

Vertical rhythm is 52px between row baselines. Rows may be taller; keep the gaps on the grid.

## Boxes

```
<rect x="30" y="140" width="470" height="104" rx="4"
      fill="var(--accent-a-tint)" stroke="var(--accent-a)" stroke-width="2"/>
```

- `rx="4"` always, matching the CSS `border-radius`.
- `stroke-width="2"` for a primary container, `1.5` for a leaf card, default `1` for a box nested
  inside a container.
- Accent fill is always the `-tint`; the stroke is the accent itself. Never fill with the accent.
- Neutral / external / not-ours: `fill="var(--card)" stroke="var(--rule)"`.
- `stroke-dasharray="5 4"` means **hypothetical, proposed, or not built yet**. Use it consistently
  or not at all; a dashed border that means nothing is noise.
- A container's inner boxes start 18px in from its left edge and 42px below its top.

## Text

- `fill="currentColor"` for anything that is just text. `.frame svg` sets `color: var(--ink-2)`,
  so this tracks the theme automatically.
- Accent-coloured text (`fill="var(--accent-a)"`) is for one labelled property per box at most.
  More than that and the accents stop meaning anything.
- Secondary lines get `opacity="0.75"` to `"0.85"`. Never below 0.7 — it fails contrast in dark mode.
- Sizes: card title `13`–`14.5` at `font-weight="600"`; body `11`–`11.5`; edge labels `11`;
  section eyebrows inside a figure `11` at `600` with `letter-spacing="1.1"`.
- Baselines: first line 26px below the box top, then 20–22px apart.
- Centred text uses `text-anchor="middle"` at the box's horizontal centre. Right-aligned corner tags
  use `text-anchor="end"` at the box's right edge minus 18.

### Character budgets

The number of characters that fit, by column width and text size. **Count before you type.**

| Column | 11.5px regular | 13.5px semibold |
|--------|----------------|-----------------|
| Quarter (225) | 28 | 22 |
| Third (306) | 40 | 32 |
| Half (470) | 64 | 50 |
| Full (960) | 140 | 110 |

Same convention as `plans/platform/dap-005/diagrams/README.md`, which hit the same problem first.
When a label will not fit, shorten the label — do not shrink the font below 10.5px.

## Arrows

Every figure carries its own `<defs>`. **Marker ids are global to the page**, so prefix them with
the figure number or figures will steal each other's arrowheads:

```
<defs>
  <marker id="f1-a" viewBox="0 0 10 10" refX="9" refY="5"
          markerWidth="7" markerHeight="7" orient="auto-start-reverse">
    <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--accent-a)"/>
  </marker>
</defs>
```

An arrowhead's `fill` cannot inherit from the line, so define one marker per colour you use:
`f1-a` (accent-a), `f1-b` (accent-b), `f1-n` (`currentColor`).

- **Direct edge** — `<line>` with `stroke-width="1.6"`, `marker-end="url(#f1-a)"`.
- **Routed edge** — `<path>` with right-angle segments only: `d="M 500 508 L 512 508 L 512 226 L 514 226"`.
  No curves.
- **Side channel / asynchronous** — add `stroke-dasharray="5 4"` and drop to `stroke-width="1.4"`.
- **Fan-in** — horizontal stubs from each source onto one vertical spine, then a single arrow.
- **Fan-out** — down from the source, along a horizontal bar, then down into each branch.
- Leave a 6px gap between an arrowhead and the box it points at, so the head does not sit on the
  stroke.
- Edge labels sit 8px above the line, `x` + 12, `font-size="11"`, `fill="currentColor"`.

## Colour has to mean something

Only three accents exist, and their meaning is declared once per page in the legend. Inside a
figure, use them for exactly that meaning. A box coloured because it looked better is a bug.

Never write a hex value inside an `<svg>`. `check_brief.py` rejects it, because a hardcoded colour
survives the theme switch and turns invisible in dark mode.

## Before you call a figure done

- [ ] Every marker id is unique across the whole page.
- [ ] Every `<text>` is within its box's character budget.
- [ ] No hex values anywhere in the SVG — tokens only.
- [ ] `role="img"` and an `aria-label` that a person could read instead of seeing the figure.
- [ ] `viewBox` height = lowest element's bottom edge + 30.
- [ ] Every accent used appears in the page legend.
- [ ] `python3 scripts/check_brief.py <file>` is clean.
- [ ] You opened the page and looked at it in both light and dark. The overflow check is an
      estimate; your eyes are the actual test.
