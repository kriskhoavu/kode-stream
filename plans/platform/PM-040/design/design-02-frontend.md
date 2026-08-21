# Frontend Design: PM-040 Taxonomy-Grouped Knowledge Views

## Overview

Both Knowledge views group by the raw `domain` path, which is a directory path of unbounded depth. The graph's
positioner handles exactly two levels of it, and the browser tree gives every level identical styling. The
restructured wiki is three and four levels deep, so the graph drops most nodes and the tree becomes unreadable.

Both views change to group by `bucket` then `area`, with `tier` demoted from a level to a page attribute. That caps
grouping at two levels for any wiki depth, which is the shape both views were built for.

## The Defect

`domainSectionPositions` assigns coordinates to top-level domains and their direct children only. A domain nested
deeper is created as a node but never positioned, so it falls to the `{ x: 0, y: 0 }` default along with its pages.

| Structure                | Rendered nodes | Stacked at origin |
|--------------------------|----------------|-------------------|
| Old wiki, at most 2 deep | 9              | 0                 |
| New wiki, 3 to 4 deep    | 19             | 10                |

Ten of nineteen nodes at one point is what produces the sparse canvas and the long crossing edges: the visible
nodes are the only ones with real positions, and the long edges terminate in the pile.

In the browser tree the same depth causes a different failure. A tier directory has no `README.md` or `index.md`
anywhere in the corpus, so `findLandingPage` returns nothing and the tier name renders as a plain label instead of a
button. The only control that opens it is the 26px chevron.

## Grouping Model

| Level  | Source          | Role in the view                                   |
|--------|-----------------|----------------------------------------------------|
| Bucket | `page.bucket`   | Top grouping; one section per bucket               |
| Area   | `page.area`     | Second grouping, slash-joined and shown as one row |
| Tier   | `page.tier`     | Partition of an area's pages; never a container    |
| Page   | the page itself | Leaf                                               |

A nested area such as `master-data/article` stays a single row labelled `Master Data / Article` rather than two
levels. Depth in the wiki no longer adds depth in the view.

Pages with no bucket, such as the corpus index, group under a single leading section. Pages with a bucket but no
area attach directly to the bucket, which both views already support.

## Visual Direction

The tree and the graph currently describe the same structure in two unrelated visual languages. The graph already
marks role by colour — purple for a domain, terracotta for a root, grey for a leaf. The tree adopts that language so
one vocabulary covers both views.

The failure to fix is not that the styling is plain. It is that indentation is the only signal, so a reader cannot
tell a bucket from a tier without counting rails. Each level therefore gets a distinct weight, case, and colour.

### Role treatments

| Role   | Colour                              | Type                                   | Marker         |
|--------|-------------------------------------|----------------------------------------|----------------|
| Bucket | `--blue`                            | 12px, 600, uppercase, 0.08em tracking  | Library icon   |
| Area   | `--button-accent` mixed to `--text` | 13px, 600, sentence case               | Book icon      |
| Tier   | `--muted`                           | 10.5px, 600, uppercase, 0.1em tracking | None; the rule |
| Page   | `--text`                            | 13px, 450, sentence case               | None; the row  |

Bucket uses `--blue` rather than a new `--purple`: the graph already marks a bucket-level node with `--blue`, and
introducing a third colour would have broken the single-vocabulary rule this section sets out.

Bare `--button-accent` measured 4.14:1 on light and 4.25:1 on dark against the list background, below AA for 13px
text in **both** themes. Mixing 85% of the accent with `--text` reaches 4.97 and 4.98 while keeping the accent
identity.

Bucket is the only uppercase heading and area the only sentence-case heading at weight 600, so the two grouping
levels are distinguishable at a glance without reading them. Tier borrows the existing page-type badge treatment,
because a tier and a page type are the same kind of fact.

### Page type, not tier, carries the colour

Tier looks like the obvious thing to colour, and it is the wrong choice. Measured on the reference corpus, the
`reference` tier is REFERENCE for 27 of 27 pages, so a tier colour would restate in a second channel what the badge
on every row beneath it already says. The `concepts` tier is where the variety actually lives — 16 CONCEPT, 13
HOW_TO and 1 DECISION — and that variation had no visual signal at all.

| Page type | Treatment                    | Why                                           |
|-----------|------------------------------|-----------------------------------------------|
| HOW_TO    | `--green` mixed to `--text`  | Actionable; the thing a reader hunts for      |
| REFERENCE | `--purple` mixed to `--text` | Lookup material                               |
| CONCEPT   | `--muted`, unchanged         | The default; marking it would mark everything |
| DECISION  | `--muted`, unchanged         | One page in the corpus; not worth a colour    |

Colour therefore means one thing per axis: level for a heading, kind for a badge. Tier labels stay muted, which is
what keeps the two axes from competing.

`--green` needed a heavier mix toward `--text` than `--purple`: at 88% it measured 4.32:1 on light, just under AA,
and 80% reaches 4.90. `--orange` and `--warning` were rejected outright because both resolve to `#f9b98c` in dark
mode, which is exactly the bucket heading colour.

### The tier rule

Tier is rendered as a short uppercase label followed by a hairline running to the edge of the list, not as a row
with a disclosure control. This is the one deliberate device on the page, and it earns its place by encoding
something true: a tier partitions an area's pages, it does not contain them. It also removes the defect directly —
there is no folder left to fail to open.

### Indentation

Only bucket and area draw an indentation rail. Tier sits flush inside its area. Because tier is not a node and a
nested area is one compound row, nesting is a single rail whatever the wiki's depth — better than the two the
grouping model allows, and against four and rising today.

```text
DOMAINS                                        ▾   bucket
│
│  Offer                                       ▾   area
│  │
│  │  CONCEPTS ─────────────────────────────       tier rule
│  │  Offer Approval                  HOW-TO
│  │  Offer Creation                  HOW-TO
│  │
│  │  REFERENCE ────────────────────────────
│  │  Action Permission Model       REFERENCE
│  │
│  Master Data / Article                       ▸   nested area, one row
```

### Interaction

A section with no landing page gets a title button that toggles it, replacing the inert label that caused the
defect. Where a landing page does exist the established interaction is kept — the title opens the index, the
chevron toggles — because that contract is already learned and only the missing-landing-page case was broken. Tier
labels are not interactive.

## Graph Positioning

Sections are laid out per bucket, then per area within the bucket, reusing the existing section geometry. Because
grouping is capped at two levels, every node receives a position by construction rather than by the positioner
happening to recognise its depth.

Tier moves onto the node itself as a badge beside the page type. The graph's domain filter becomes a bucket filter,
with a separate tier filter, so the twenty-value flat list of path strings becomes two short lists.

## Accessibility and Quality Floor

Colour is never the only signal: each role also differs in weight, case, and marker. Header rows stay real buttons
with `aria-expanded`, keyboard arrow navigation is unchanged, and the tier rule is decorative with its label read as
a group heading.

Measured against the list background, every role clears AA in both themes: bucket 6.01 light and 10.60 dark, area
4.97 and 4.98, tier 4.55 and 7.13, page title 16.97 and above. The page-type marks clear it too: HOW_TO 4.91 and
9.27, REFERENCE 7.75 and 7.17, unmarked 4.55 and 7.13.

In light mode the REFERENCE violet sits in the same family as the indigo bucket heading. They stay separable by
size, weight, and position rather than hue alone, which is the weakest pairing in the scheme and worth revisiting if
a better token appears.

## Design Decisions

| Decision                               | Rationale                                                                                     |
|----------------------------------------|-----------------------------------------------------------------------------------------------|
| Group by bucket and area, not `domain` | Caps grouping at two levels for any wiki depth, which is the shape both views already assume. |
| Tier as attribute, not container       | Tiers have no landing page and carry no subject meaning; as a level they only add depth.      |
| Nested area as one compound row        | Keeps indentation flat and matches the slash-joined area the backend already produces.        |
| Reuse the graph's role colours         | One vocabulary across both views; no new tokens, so both themes stay correct for free.        |
| Colour page type, not tier             | Tier restates the badge beside it 27/27 times; page type is what varies inside a group.       |
| Mark only two of four page types       | CONCEPT is the default and DECISION is a single page; marking all four would read as noise.   |
| Whole header row toggles               | Removes the dependency on a `README.md` that no tier directory has.                           |
| Keep `domain` in the payload           | Still used for breadcrumbs and existing tests; grouping simply stops reading it.              |
