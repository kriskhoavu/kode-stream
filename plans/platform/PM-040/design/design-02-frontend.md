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

| Role   | Colour            | Type                                   | Marker         |
|--------|-------------------|----------------------------------------|----------------|
| Bucket | `--purple`        | 12px, 600, uppercase, 0.08em tracking  | Library icon   |
| Area   | `--button-accent` | 13px, 600, sentence case               | Book icon      |
| Tier   | `--muted`         | 10.5px, 600, uppercase, 0.1em tracking | None; the rule |
| Page   | `--text`          | 13px, 450, sentence case               | None; the row  |

Bucket is the only uppercase heading and area the only sentence-case heading at weight 600, so the two grouping
levels are distinguishable at a glance without reading them. Tier borrows the existing page-type badge treatment,
because a tier and a page type are the same kind of fact.

### The tier rule

Tier is rendered as a short uppercase label followed by a hairline running to the edge of the list, not as a row
with a disclosure control. This is the one deliberate device on the page, and it earns its place by encoding
something true: a tier partitions an area's pages, it does not contain them. It also removes the defect directly —
there is no folder left to fail to open.

### Indentation

Only bucket and area draw an indentation rail. Tier sits flush inside its area. Maximum indent is therefore two
rails whatever the wiki's depth, against four and rising today.

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

The whole header row toggles its section, so expansion never depends on a landing page existing. Where a landing
page does exist it keeps a separate control that opens it, so opening a section and reading its index stay
distinct actions. Tier labels are not interactive.

## Graph Positioning

Sections are laid out per bucket, then per area within the bucket, reusing the existing section geometry. Because
grouping is capped at two levels, every node receives a position by construction rather than by the positioner
happening to recognise its depth.

Tier moves onto the node itself as a badge beside the page type. The graph's domain filter becomes a bucket filter,
with a separate tier filter, so the twenty-value flat list of path strings becomes two short lists.

## Accessibility and Quality Floor

Colour is never the only signal: each role also differs in weight, case, and marker. Header rows stay real buttons
with `aria-expanded`, keyboard arrow navigation is unchanged, and the tier rule is decorative with its label read as
a group heading. Both themes are covered by using existing tokens rather than new colour values.

## Design Decisions

| Decision                               | Rationale                                                                                     |
|----------------------------------------|-----------------------------------------------------------------------------------------------|
| Group by bucket and area, not `domain` | Caps grouping at two levels for any wiki depth, which is the shape both views already assume. |
| Tier as attribute, not container       | Tiers have no landing page and carry no subject meaning; as a level they only add depth.      |
| Nested area as one compound row        | Keeps indentation flat and matches the slash-joined area the backend already produces.        |
| Reuse the graph's role colours         | One vocabulary across both views; no new tokens, so both themes stay correct for free.        |
| Whole header row toggles               | Removes the dependency on a `README.md` that no tier directory has.                           |
| Keep `domain` in the payload           | Still used for breadcrumbs and existing tests; grouping simply stops reading it.              |
