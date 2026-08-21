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
| Bucket | `--button-accent` mixed to `--text` | 12px, 600, uppercase, 0.08em tracking  | Library icon   |
| Area   | `--text`                            | 13px, 600, sentence case               | Book icon      |
| Tier   | Per tier; see below                 | 10.5px, 600, uppercase, 0.1em tracking | None; the rule |
| Page   | `--text`                            | 13px, 450, sentence case               | None; the row  |

Only two levels are coloured — the bucket and the tier. An area reads as the level below a bucket through weight and
case, and a page title needs no emphasis on top of the three treatments above it. A first pass coloured three levels
and bolded the page titles as well; review found the result too busy, and the signal each level carries survives the
reduction intact.

Bucket is the only uppercase heading and area the only sentence-case heading at weight 600, so the two grouping
levels stay distinguishable at a glance without reading them.

Bare `--button-accent` measured 4.14:1 on light and 4.25:1 on dark against the list background, below AA in **both**
themes. Mixing 85% of the accent with `--text` reaches 4.97 and 4.98 while keeping the accent identity.

No blue remains in the tree. Hover, the active landing-page icon, and the selected-row wash all use the selection
accent that already marks the selected row's edge, rather than introducing a colour the hierarchy does not use.

### The tier carries the colour

The tier is the grouping a reader scans, so it is what gets the colour. Page type does not, and inside a tier it is
not shown at all.

| Element                 | Treatment                    | Why                                          |
|-------------------------|------------------------------|----------------------------------------------|
| Tier `concepts`         | `--green` mixed to `--text`  | Explanatory material                         |
| Tier `reference`        | `--purple` mixed to `--text` | Lookup material                              |
| Any other tier          | `--muted`, unchanged         | No colour is assigned to a tier nobody chose |
| Page type inside a tier | Not rendered                 | The label above already states it            |
| Page type with no tier  | `--muted`, unchanged         | The only classification such a row carries   |

Suppressing the page type inside a tier removes a measured redundancy: the `reference` tier is REFERENCE for 27 of
27 pages, so the badge restated its own heading on every row. It also buys back a line of density in a list that
runs to 34 pages in one area.

The cost is real and worth stating: the `concepts` tier mixes 16 CONCEPT, 13 HOW_TO and 1 DECISION, so hiding the
badge there loses a distinction the tier does not carry. That was accepted deliberately in favour of a calmer list;
the page type is still on the page itself and on the graph node.

`--green` needs a heavier mix toward `--text` than `--purple`: at 88% it measured 4.32:1 on light, so it sits at
80%. `--orange` and `--warning` were rejected outright because both resolve to `#f9b98c` in dark mode, which is
exactly the bucket heading colour.

### The tier rule

Tier is rendered as a short uppercase label followed by a hairline running to the edge of the list. This is the one
deliberate device on the page.

The label is also the tier's disclosure control, and tiers start collapsed. An earlier pass made a tier
non-interactive on the argument that it partitions an area's pages rather than containing them. Use showed the
argument was too pure: an area of 27 pages needs to be closable, and the reader's mental model is a folder whatever
the taxonomy calls it.

The original defect cannot recur through this. It happened because a tier's only control was a chevron that existed
only when a `README.md` did; here the label itself is the button and depends on nothing. Two behaviours protect the
collapsed default:

| Situation          | Behaviour                                             |
|--------------------|-------------------------------------------------------|
| A page is selected | Its bucket, area, and tier all open, so it is visible |
| A filter is active | Every tier opens, so no match hides inside a shut one |

Without the first, a page opened from a wiki link or a deep link would render inside a closed section and appear
missing.

### Indentation

Only bucket and area draw an indentation rail. Tier sits flush inside its area. Because tier is not a node and a
nested area is one compound row, nesting is a single rail whatever the wiki's depth — better than the two the
grouping model allows, and against four and rising today.

```text
DOMAINS                                        ▾   bucket
│
│  Offer                                       ▾   area
│  │
│  │  ⌄ CONCEPTS ───────────────────────────       tier rule, open
│  │  Offer Approval
│  │  Offer Creation
│  │
│  │  › REFERENCE ──────────────────────────       tier rule, shut by default
│  │
│  Master Data / Article                       ▸   nested area, one row
```

### Interaction

A section with no landing page gets a title button that toggles it, replacing the inert label that caused the
defect. Where a landing page does exist the established interaction is kept — the title opens the index, the
chevron toggles — because that contract is already learned and only the missing-landing-page case was broken. A tier
label is a toggle for its own section and nothing else.

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

Measured against the list background, every role clears AA in both themes: bucket 4.97 light and 4.98 dark, area and
page title 16.97 and 14.44, `concepts` 4.91 and 9.27, `reference` 7.75 and 7.17, an unmarked tier or page type 4.55
and 7.13.

Retiring the indigo also retired the one weak pairing in the earlier scheme, where the `reference` violet sat in the
same hue family as the bucket heading. Bucket orange, `concepts` green, and `reference` violet are three distinct
families.

Colour is still never the only signal. A tier keeps its written label, and a page type suppressed in the tree is
still shown on the page itself and on the graph node.

## Design Decisions

| Decision                                 | Rationale                                                                                     |
|------------------------------------------|-----------------------------------------------------------------------------------------------|
| Group by bucket and area, not `domain`   | Caps grouping at two levels for any wiki depth, which is the shape both views already assume. |
| Tier as attribute, not container         | Tiers have no landing page and carry no subject meaning; as a level they only add depth.      |
| Nested area as one compound row          | Keeps indentation flat and matches the slash-joined area the backend already produces.        |
| Reuse the graph's role colours           | One vocabulary across both views; no new tokens, so both themes stay correct for free.        |
| Colour the tier, not the page type       | The tier is the grouping a reader scans; colouring both would make colour mean two things.    |
| Hide the page type inside a tier         | The label above states it; for the reference tier it restated it 27 times out of 27.          |
| Keep the page type when there is no tier | Such a row has no heading above it, so the badge is its only classification.                  |
| Whole header row toggles                 | Removes the dependency on a `README.md` that no tier directory has.                           |
| Keep `domain` in the payload             | Still used for breadcrumbs and existing tests; grouping simply stops reading it.              |
