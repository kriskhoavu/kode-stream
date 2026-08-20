# Implementation Plan: PM-040 - Detected Wiki Taxonomy

## Overview

Add taxonomy detection and an optional settings override to `internal/knowledge`, carry bucket, area, and tier on the
page model, and replace the hardcoded `e2e-testing` prefix match with role-based bucket resolution. Backend only.

## Execution Flow

```text
B1 Detection and classification ──┐
                                  ├─> B3 Model and indexer wiring ─> B4 Role-based journey lookup
B2 Settings override reader ──────┘
```

B1 and B2 are pure, independent, and testable with no wiring, so they can be built in either order or in parallel. B3
is the join: it is the first phase that touches shared indexer code, and it needs both the detector from B1 and the
override reader from B2 to apply a final taxonomy. B4 is the only behavioural change in the ticket and depends on B3
having populated `bucket`. The critical path is B1 → B3 → B4; B2 is the shorter branch. No external gates.

## Phases Summary

| Phase | Name                         | Track   | Status |
|-------|------------------------------|---------|--------|
| B1    | Detection and classification | Backend |        |
| B2    | Settings override reader     | Backend |        |
| B3    | Model and indexer wiring     | Backend |        |
| B4    | Role-based journey lookup    | Backend |        |

## Backend Phases

### Phase B1: Detection And Classification

Tests first: the three structural shapes from the README table are the acceptance criteria, and the within-bucket rule
must be shown to reject the global-recurrence alternative.

**Deliverables:**

- [ ] `internal/knowledge/taxonomy.go` — `Taxonomy` type, tier detection, path classification.
- [ ] Table tests for tier under bucket with no area (`platform/concepts`).
- [ ] Table tests for area nested in area (`domains/master-data/article/reference`).
- [ ] Table tests for a bucket with no tier at all (`e2e-testing/offer`).
- [ ] Test that a root-level page yields empty bucket, area, and tier.
- [ ] Regression test that `offer` and `master-data`, which recur across buckets, are areas and not tiers.
- [ ] Test that a globally-learned tier is recognised where it appears only once in its bucket.
- [ ] Test that an empty page set yields an empty tier set without panicking.

**Verification:** `go test ./internal/knowledge`

**Commit:** `PM-040: Add wiki taxonomy detection and classification`

---

### Phase B2: Settings Override Reader

Mirrors `internal/workspace/scanner/settings.go` in structure and in warning behaviour. Every defect degrades to
detection rather than failing the scan.

**Deliverables:**

- [ ] `internal/knowledge/taxonomy_settings.go` — read and validate `knowledge-settings.yaml` with `KnownFields(true)`.
- [ ] Absent file returns no override and no warning.
- [ ] Test that `taxonomy.tiers` replaces rather than merges with a detected set.
- [ ] Test that a declared bucket role is returned.
- [ ] Warning tests for bad YAML, `version` other than `1`, unknown field, unknown role, duplicate bucket path.
- [ ] Test that every defect yields `invalid_metadata` and never an error return.

**Verification:** `go test ./internal/knowledge`

**Commit:** `PM-040: Add knowledge taxonomy settings override`

---

### Phase B3: Model And Indexer Wiring

The join phase. First contact with shared indexer code, so the existing detector tests are the regression net.

**Deliverables:**

- [ ] `internal/knowledge/models.go` — `Bucket`, `Area`, `Tier` on `KnowledgePage`, omitted when empty.
- [ ] `internal/knowledge/detector.go` — read settings, detect, apply override, classify, after `ResolveRelationships`.
- [ ] Settings warnings joined into the existing wiki warning list.
- [ ] Detector test over a fixture tree asserting classification of every page.
- [ ] Detector test asserting a malformed settings file still indexes every page and adds a warning.
- [ ] Test that `Domain` is unchanged for every page in the fixture.

**Verification:** `go test ./internal/knowledge ./internal/server/api`

**Commit:** `PM-040: Classify knowledge pages by bucket, area, and tier`

---

### Phase B4: Role-Based Journey Lookup

The only behavioural change. The compatibility default is what keeps existing Wiki Roots working, so it needs an
explicit test rather than being assumed.

**Deliverables:**

- [ ] `internal/knowledge/knowledge_service.go` — resolve the journeys bucket by role in `E2ERunbooksForSources`.
- [ ] Same resolution in `E2ERunbook`; both literal prefix matches removed.
- [ ] Test that with no settings file the default `e2e-testing` bucket resolves, preserving PM-036 behaviour.
- [ ] Test that a declared `journeys` bucket replaces the default.
- [ ] Test that a sibling bucket such as `e2e-testing-archive` no longer matches, proving exact over prefix.
- [ ] Test that a declared bucket absent from the tree returns no journeys and warns.

**Verification:** `go test ./internal/knowledge ./internal/server/api`

**Commit:** `PM-040: Resolve E2E journeys by bucket role`

---

## Post-Implementation Checklist

- [ ] Full backend suite: `go test ./...`
- [ ] Frontend untouched but confirmed green: `npm run typecheck && npm test`
- [ ] Update `docs/architecture/ARCHITECTURE.md` where it describes Knowledge wiki detection.
- [ ] Confirm no literal remains: `grep -rn '"e2e-testing"' internal/`
- [ ] Update PM-040 documents if naming drifted during implementation.
- [ ] Re-render `brief.html` if the plan changed.
- [ ] Keep phase commits separate.

## Testing Strategy

Table-driven unit tests for detection and classification, since the rule is a pure function of path shape and the
interesting cases are enumerable. Settings tests follow the existing scanner settings tests, asserting warnings rather
than errors. One detector test over a fixture tree proves the wiring, and the B4 service tests pin the compatibility
default, which is the single highest-risk assumption in the ticket.

The reference Wiki Root shapes are reproduced as fixtures rather than read from disk, so the tests stay independent of
any particular workspace.

## Migration Notes

None. The Knowledge index is app-owned and rebuilt by rescan, so the new fields populate on the next scan. An index
written before PM-040 yields empty taxonomy fields, and the journey lookup falls back to the default bucket, which
matches pre-PM-040 behaviour. No persisted schema, route, or JSON contract is broken; the added fields are omitted
when empty.
