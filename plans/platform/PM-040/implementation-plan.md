# Implementation Plan: PM-040 - Detected Wiki Taxonomy

## Overview

Add taxonomy detection and an optional settings override to `internal/knowledge`, carry bucket, area, and tier on the
page model, and replace the hardcoded `e2e-testing` prefix match with role-based bucket resolution. Then consume the
taxonomy in both Knowledge views, which group by the raw domain path today and break on a wiki more than two
directories deep.

## Execution Flow

```text
B1 Detection and classification ──┐
                                  ├─> B3 Model and indexer wiring ─> B4 Role-based journey lookup
B2 Settings override reader ──────┘         │
                                            └─> B5 Graph payload ─> F1 Graph grouping ──┐
                                                                                        ├─> F3 Hierarchy styling
                                                                    F2 Pages grouping ──┘
```

B1 and B2 are pure, independent, and testable with no wiring. B3 is the join: the first phase touching shared indexer
code, needing both the detector and the override reader. B4 is the only backend behaviour change. B5 exposes the
taxonomy on the graph payload and is the sole backend prerequisite for the frontend work. F1 and F2 are independent
of each other and can run in parallel once B5 lands; F2 needs no backend change beyond B3. F3 restyles what both
produce, so it lands last. The critical path is B1 → B3 → B5 → F1 → F3. No external gates.

## Phases Summary

| Phase | Name                          | Track    | Status   |
|-------|-------------------------------|----------|----------|
| B1    | Detection and classification  | Backend  | Complete |
| B2    | Settings override reader      | Backend  | Complete |
| B3    | Model and indexer wiring      | Backend  | Complete |
| B4    | Role-based journey lookup     | Backend  | Complete |
| B5    | Taxonomy on the graph payload | Backend  | Complete |
| F1    | Graph grouping by taxonomy    | Frontend | Complete |
| F2    | Pages grouping by taxonomy    | Frontend | Complete |
| F3    | Hierarchy styling pass        | Frontend | Complete |
| F4    | Tier colour and row density   | Frontend | Complete |
| F5    | Palette reduction             | Frontend | Complete |

## Backend Phases

### Phase B1: Detection And Classification

Tests first: the three structural shapes from the README table are the acceptance criteria, and the within-bucket rule
must be shown to reject the global-recurrence alternative.

**Deliverables:**

- [x] `internal/knowledge/taxonomy.go` — `Taxonomy` type, tier detection, path classification.
- [x] Table tests for tier under bucket with no area (`platform/concepts`).
- [x] Table tests for area nested in area (`domains/master-data/article/reference`).
- [x] Table tests for a bucket with no tier at all (`e2e-testing/offer`).
- [x] Test that a root-level page yields empty bucket, area, and tier.
- [x] Regression test that `offer` and `master-data`, which recur across buckets, are areas and not tiers.
- [x] Test that a globally-learned tier is recognised where it appears only once in its bucket.
- [x] Test that an empty page set yields an empty tier set without panicking.

**Verification:** `go test ./internal/knowledge`

**Commit:** `PM-040: Add wiki taxonomy detection and classification`

---

### Phase B2: Settings Override Reader

Mirrors `internal/workspace/scanner/settings.go` in structure and in warning behaviour. Every defect degrades to
detection rather than failing the scan.

**Deliverables:**

- [x] `internal/knowledge/taxonomy_settings.go` — read and validate `knowledge-settings.yaml` with `KnownFields(true)`.
- [x] Absent file returns no override and no warning.
- [x] Test that `taxonomy.tiers` replaces rather than merges with a detected set.
- [x] Test that a declared bucket role is returned.
- [x] Warning tests for bad YAML, `version` other than `1`, unknown field, unknown role, duplicate bucket path.
- [x] Test that every defect yields `invalid_metadata` and never an error return.

**Verification:** `go test ./internal/knowledge`

**Commit:** `PM-040: Add knowledge taxonomy settings override`

---

### Phase B3: Model And Indexer Wiring

The join phase. First contact with shared indexer code, so the existing detector tests are the regression net.

**Deliverables:**

- [x] `internal/knowledge/models.go` — `Bucket`, `Area`, `Tier` on `KnowledgePage`, omitted when empty.
- [x] `internal/knowledge/detector.go` — read settings, detect, apply override, classify, after `ResolveRelationships`.
- [x] Settings warnings joined into the existing wiki warning list.
- [x] Detector test over a fixture tree asserting classification of every page.
- [x] Detector test asserting a malformed settings file still indexes every page and adds a warning.
- [x] Test that `Domain` is unchanged for every page in the fixture.

**Verification:** `go test ./internal/knowledge ./internal/server/api`

**Commit:** `PM-040: Classify knowledge pages by bucket, area, and tier`

---

### Phase B4: Role-Based Journey Lookup

The only behavioural change. The compatibility default is what keeps existing Wiki Roots working, so it needs an
explicit test rather than being assumed.

**Deliverables:**

- [x] `internal/knowledge/knowledge_service.go` — resolve the journeys bucket by role in `E2ERunbooksForSources`.
- [x] Same resolution in `E2ERunbook`; both literal prefix matches removed.
- [x] Test that with no settings file the default `e2e-testing` bucket resolves, preserving PM-036 behaviour.
- [x] Test that a declared `journeys` bucket replaces the default.
- [x] Test that a sibling bucket such as `e2e-testing-archive` no longer matches, proving exact over prefix.
- [x] Test that a declared bucket absent from the tree returns no journeys and warns.

**Verification:** `go test ./internal/knowledge ./internal/server/api`

**Commit:** `PM-040: Resolve E2E journeys by bucket role`

---

---

### Phase B5: Taxonomy On The Graph Payload

The graph node carries `domain` but not the taxonomy, so the frontend cannot group by it. One field trio, mirroring
what `KnowledgePage` already holds.

**Deliverables:**

- [x] `internal/knowledge/models.go` — `Bucket`, `Area`, and `Tier` on `KnowledgeGraphNode`, omitted when empty.
- [x] `internal/knowledge/relationships.go` — copy the three fields when building each graph node.
- [x] Test that a graph built from classified pages carries the taxonomy on every node.
- [x] Test that `Domain` is still present and unchanged on the graph node.

**Verification:** `go test ./internal/knowledge ./internal/server/api`

**Commit:** `PM-040: Expose taxonomy on knowledge graph nodes`

---

### Phase F1: Graph Grouping By Taxonomy

Replaces domain-path grouping with bucket then area, capping grouping at two levels for any wiki depth. This is the
phase that fixes the reported defect.

**Deliverables:**

- [x] `web/src/features/knowledge/graphModel.ts` — group and position by bucket then area.
- [x] Nested areas render as one compound row rather than one level per segment.
- [x] Tier moves onto the node as a badge beside the page type.
- [x] Bucket filter and separate tier filter replace the flat domain dropdown.
- [x] Regression test asserting no node lands at the origin for a four-level tree.
- [x] Test covering a bucket with no area, and pages with no bucket at all.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-040: Group the knowledge graph by bucket and area`

---

### Phase F2: Pages Grouping By Taxonomy

Same grouping model in the browser tree, and the fix for tier folders that cannot be opened.

**Deliverables:**

- [x] `web/src/features/knowledge/KnowledgeBrowser.tsx` — build the tree from bucket and area.
- [x] Render tier as a labelled partition inside its area, not a collapsible node.
- [x] The whole header row toggles its section, so expansion never depends on a landing page.
- [x] Keep a distinct control for opening a landing page where one exists.
- [x] Test that a tier with no `README.md` is reachable without a landing page.
- [x] Test that selecting a deep page expands its bucket and area.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-040: Group knowledge pages by bucket, area, and tier`

---

### Phase F3: Hierarchy Styling Pass

Indentation, weight, and colour per role, so depth is not the only signal. See `design/design-02-frontend.md`.

**Deliverables:**

- [x] `web/src/features/knowledge/knowledge.css` — per-role weight, case, and colour from existing tokens.
- [x] Bucket and area draw indentation rails; tier sits flush, capping indent at two rails.
- [x] Tier rendered as an uppercase label with a hairline to the list edge.
- [x] Distinct markers for bucket and area; none for tier or page.
- [x] Graph node role styling aligned to the same vocabulary.
- [x] Verify both themes and keyboard focus in the browser pane.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-040: Restyle the knowledge hierarchy by role`

---

### Phase F4: Tier Colour And Row Density

Follow-up after review. A first pass coloured the page type instead; review corrected it to the tier, which is the
grouping a reader actually scans.

**Deliverables:**

- [x] Colour the `concepts` and `reference` tier labels and the graph tier badges.
- [x] Leave any other tier muted rather than assigning a colour nobody chose.
- [x] Stop rendering the page type on rows inside a tier.
- [x] Keep the page type on rows with no tier, where it is the only classification.
- [x] Tests for the tier modifier classes, the suppression, and the untiered exception.
- [x] Contrast measured in both themes; the green needed an 80% mix to clear AA.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-040: Colour the tier and drop the repeated page type`

---

### Phase F5: Palette Reduction

Second review pass: the hierarchy was legible but carried too much colour.

**Deliverables:**

- [x] Bucket heading moves from indigo to the accent orange, mixed toward `--text` for AA.
- [x] Area heading drops the accent and becomes plain `--text`, still bold.
- [x] Page titles are no longer bold.
- [x] Remaining blue retired from hover, the active landing-page icon, and the selected-row wash.
- [x] Contrast re-measured in both themes; bucket 4.97 and 4.98, everything else unchanged or better.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/knowledge`

**Commit:** `PM-040: Quieten the tree palette`

## Post-Implementation Checklist

- [x] Full backend suite: `go test ./...`
- [x] Frontend untouched but confirmed green: `npm run typecheck && npm test`
- [x] Update `docs/architecture/ARCHITECTURE.md` where it describes Knowledge wiki detection.
- [x] Confirm no literal remains: only the named `DefaultJourneysBucket` constant
- [x] Update PM-040 documents if naming drifted during implementation.
- [x] Re-render `brief.html` if the plan changed.
- [x] Keep phase commits separate.

## Testing Strategy

Table-driven unit tests for detection and classification, since the rule is a pure function of path shape and the
interesting cases are enumerable. Settings tests follow the existing scanner settings tests, asserting warnings rather
than errors. One detector test over a fixture tree proves the wiring, and the B4 service tests pin the compatibility
default, which is the single highest-risk assumption in the ticket.

The reference Wiki Root shapes are reproduced as fixtures rather than read from disk, so the tests stay independent of
any particular workspace.

## Migration Notes

None, but not for the reason first assumed. The Knowledge index is app-owned and rebuilt by rescan, so the new
fields populate on the next scan. An index written before PM-040 carries no `bucket` at all, so matching on bucket
alone would have returned no journeys until a rescan. Journey resolution therefore falls back to the page path when
`bucket` is empty, which is what actually preserves pre-PM-040 behaviour.

The existing full-stack test `TestE2ERunbookReadRoutesReturnLocalAndCanonicalCoverage` writes its index directly
with `Domain` set and no `Bucket`, so it is exactly this case; removing the fallback makes it fail with "No
canonical E2E journey is linked to this plan." No persisted schema, route, or JSON contract is broken, and the
added fields are omitted when empty.
