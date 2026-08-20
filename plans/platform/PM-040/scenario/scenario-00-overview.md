# Scenarios: PM-040 Overview

PM-040 has no user-visible surface. These scenarios describe indexer behaviour and are the acceptance criteria the
tests encode.

## Scenario List

| #   | Title                    | Description                                                 |
|-----|--------------------------|-------------------------------------------------------------|
| 0   | Pure detection           | No settings file; taxonomy inferred from layout alone       |
| 1   | Declared journeys bucket | Settings rename the journeys bucket; runbook lookup follows |
| 2   | Insufficient evidence    | A young wiki with no recurring tier name                    |
| 3   | Malformed settings       | A broken settings file degrades to detection                |

---

# Scenario 0: Pure Detection

## Goal

> A curator rescans a Wiki Root that has no `knowledge-settings.yaml` and every page is classified from layout alone.

## Starting State

| #   | Title            | Summary                                                             |
|-----|------------------|---------------------------------------------------------------------|
| 0.1 | Wiki Root        | Four buckets, `concepts` and `reference` recurring inside `domains` |
| 0.2 | No settings file | `knowledge-settings.yaml` absent from the Wiki Root                 |

## Visual State (Before)

```text
wiki/
├── index.md
├── domains/offer/concepts/approval.md
├── domains/master-data/article/reference/spec.md
├── platform/concepts/deployment-rollback.md
└── e2e-testing/offer/approval.md
```

## Execution Flows

### Flow 0.1: Rescan Classifies Every Page

```text
Rescan request
    ↓
Walk and parse pages
    ↓
Resolve relationships
    ↓
Detect tiers: concepts, reference
    ↓
Classify each page
    ↓
Persist knowledge-index.yaml
```

## Visual State (After)

```text
approval.md              bucket=domains      area=offer                tier=concepts
spec.md                  bucket=domains      area=master-data/article  tier=reference
deployment-rollback.md   bucket=platform     area=                     tier=concepts
e2e approval.md          bucket=e2e-testing  area=offer                tier=
index.md                 bucket=             area=                     tier=
```

`platform/concepts` resolves as a tier despite appearing under one parent in that bucket, because the tier set is
learned globally from the evidence inside `domains`.

## Edge Cases

- A page at the Wiki Root has no bucket; all three fields stay empty.
- Segments after a matched tier fold into the area rather than being dropped.
- Journey lookup uses the default `e2e-testing` bucket, so PM-036 behaviour is unchanged.

---

# Scenario 1: Declared Journeys Bucket

## Goal

> A curator whose journeys live in a differently named bucket declares it, and the E2E runbook lookup follows.

## Starting State

| #   | Title          | Summary                                                    |
|-----|----------------|------------------------------------------------------------|
| 1.1 | Renamed bucket | Journeys live under `journeys/` rather than `e2e-testing/` |
| 1.2 | Settings file  | Declares the `journeys` role on bucket `journeys`          |

## Execution Flows

### Flow 1.1: Runbook Lookup Resolves By Role

```text
Plan requests linked E2E coverage
    ↓
Resolve journeys role → bucket "journeys"
    ↓
Select pages whose bucket matches
    ↓
Match sourceRef against the plan
    ↓
Return canonical journeys
```

## Visual State (After)

Pages under `journeys/` are returned as canonical coverage. Pages under `e2e-testing/`, if any remain, are not — the
declared role has replaced the default rather than adding to it.

## Edge Cases

- Declaring a bucket absent from the tree warns and returns no journeys.
- Declaring an unknown role warns and drops that entry, leaving the default in force.
- A bucket named `e2e-testing-archive` does not match the default, because comparison is exact rather than prefix.

---

# Scenario 2: Insufficient Evidence

## Goal

> A wiki early in its life produces no tiers rather than guessing one from a single example.

## Starting State

| #   | Title      | Summary                                                   |
|-----|------------|-----------------------------------------------------------|
| 2.1 | Young wiki | One bucket, one area, one `concepts` directory beneath it |

## Execution Flows

### Flow 2.1: Detection Declines

```text
Detect tiers → no name reaches two parents in any bucket
    ↓
Tier set is empty
    ↓
concepts classified as area, not tier
```

## Visual State (After)

```text
domains/offer/concepts/approval.md   bucket=domains  area=offer/concepts  tier=
```

As a second area gains a `concepts` directory, the next rescan promotes `concepts` to a tier and the area shortens to
`offer`. A curator wanting stability sooner declares `taxonomy.tiers` explicitly.

## Edge Cases

- Classification shifting between rescans is expected, not a defect.
- An empty Wiki Root produces an empty tier set and no warnings.

---

# Scenario 3: Malformed Settings

## Goal

> A broken settings file degrades to detection and reports why, rather than blanking the Wiki.

## Execution Flows

### Flow 3.1: Warn And Fall Back

```text
Read knowledge-settings.yaml
    ↓
Parse fails, or version is not 1, or a field is unknown
    ↓
Emit invalid_metadata warning
    ↓
Continue with pure detection
```

## Visual State (After)

Every page is still classified and readable. The warning appears in the existing Knowledge warnings surface built by
PM-022.

## Edge Cases

- An unreadable file is a warning, not a scan failure.
- An unknown field is rejected by `KnownFields(true)` and warns.
- Duplicate bucket paths warn; the last entry wins.
