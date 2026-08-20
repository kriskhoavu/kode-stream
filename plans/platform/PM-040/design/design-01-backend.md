# Backend Design: PM-040 Detected Wiki Taxonomy

## Overview

Two new files under `internal/knowledge`: one detects and applies the taxonomy, one reads the optional override. Three
new fields on the page model. Two call sites in the Knowledge service stop matching a literal. No persisted schema
changes, no new HTTP routes, no changes under `web/`.

The Knowledge index is app-owned and rebuilt by rescan, so the new fields need no migration — the next rescan
repopulates them. A stale index read before its first rescan yields empty bucket, area, and tier. Defaulting the
bucket *name* is not enough on its own in that state, because no page has a bucket to compare against; journey
resolution falls back to the page path instead. See Role Resolution below.

## Data Model

### Page fields added to `KnowledgePage`

Serialized in the existing `knowledge-index.yaml` alongside `domain`. All three are omitted when empty.

| Field    | Type     | Purpose                                                     |
|----------|----------|-------------------------------------------------------------|
| `bucket` | `string` | First path segment; empty for a page at the Wiki Root       |
| `area`   | `string` | Slash-joined subject segments between bucket and tier       |
| `tier`   | `string` | Recognised tier segment; empty when the bucket has no tiers |

`Domain` is untouched, keeps its PM-022 meaning, and remains the field the browser tree and graph filter read.

### Taxonomy value type

| Member  | Type              | Purpose                                                 |
|---------|-------------------|---------------------------------------------------------|
| `Tiers` | set of `string`   | Recognised tier directory names for this Wiki Root      |
| `Roles` | bucket → role map | Declared meaning per bucket; empty under pure detection |

One role is defined in this ticket: `journeys`. The type permits others without further change.

## Detection Algorithm

Input is the relative path of every indexed page in one Wiki Root. Output is the tier name set.

```text
for each page path:
    segments = split(dirname(path), "/")
    if segments is empty: skip           # root-level page has no bucket
    bucket = segments[0]
    for i from 1 to len(segments) - 1:
        name   = segments[i]
        parent = join(segments[0..i-1], "/")
        record parent into candidates[bucket][name]

tiers = { name : exists bucket where count(distinct candidates[bucket][name]) >= 2 }
```

Threshold is two distinct parents within a single bucket. The set is global once learned, so a name qualifying anywhere
is treated as a tier everywhere.

## Classification

With the tier set known, a page path resolves left to right.

| Step | Rule                                                                     |
|------|--------------------------------------------------------------------------|
| 1    | Empty directory part yields empty bucket, area, and tier                 |
| 2    | First segment is the bucket                                              |
| 3    | Scan remaining segments; the first one in the tier set becomes the tier  |
| 4    | Segments before the tier join with `/` to form the area                  |
| 5    | No tier match leaves tier empty and all remaining segments form the area |

Segments after a matched tier are folded into the area rather than dropped, so a deeper-than-expected layout degrades
instead of losing information.

## Settings Override

Optional file `knowledge-settings.yaml` at the Wiki Root. Absent means pure detection. Read with `KnownFields(true)`,
matching `scanner/settings.go`.

| Key                       | Type           | Effect when present                                        |
|---------------------------|----------------|------------------------------------------------------------|
| `version`                 | integer        | Must be `1`; any other value warns and the file is ignored |
| `taxonomy.tiers`          | list of string | Replaces the detected tier set outright                    |
| `taxonomy.buckets[].path` | string         | Bucket directory name the role attaches to                 |
| `taxonomy.buckets[].role` | string         | Role for that bucket; unknown roles warn and are dropped   |

Replace rather than merge for `tiers`: a curator listing tiers explicitly is correcting detection, and a merge would
make a wrong detected tier impossible to remove.

### Validation and degradation

Every defect is a `KnowledgeWarning`, never a scan failure, consistent with PM-022 detection warnings.

| Condition                    | Code               | Result                                   |
|------------------------------|--------------------|------------------------------------------|
| File unreadable or bad YAML  | `invalid_metadata` | Warn; fall back to pure detection        |
| `version` not `1`            | `invalid_metadata` | Warn; fall back to pure detection        |
| Unknown role value           | `invalid_metadata` | Warn; drop that bucket entry only        |
| Duplicate bucket path        | `invalid_metadata` | Warn; last entry wins                    |
| Bucket path absent from tree | `invalid_metadata` | Warn; keep the role, it may appear later |

A bucket named in settings but missing from the tree warns rather than errors: the ordering between editing settings
and adding the directory is not ours to dictate.

## Integration Point

Detection needs the whole page set before it can classify any single page, so it cannot run inside `ParsePage`. It runs
in `Detector.DetectSource` after `ResolveRelationships`, at the point where the page slice is complete and warnings are
still being collected.

```text
walk and read files
    ↓
ParsePage per file
    ↓
ResolveRelationships
    ↓
read knowledge-settings.yaml → warnings
    ↓
DetectTaxonomy over page paths → apply override
    ↓
Classify each page → bucket, area, tier
```

## Role Resolution

`E2ERunbooksForSources` and `E2ERunbook` currently test `strings.HasPrefix(page.Domain, "e2e-testing")`. Both instead
select pages whose bucket is the bucket holding role `journeys`.

| Situation                            | Resolved journeys bucket                 |
|--------------------------------------|------------------------------------------|
| No settings file                     | `e2e-testing`, the compatibility default |
| Settings declare a `journeys` bucket | The declared bucket                      |
| Settings declare no `journeys` role  | `e2e-testing`, the same default          |

A page is then matched against that bucket in one of two ways. A page carrying a `bucket` is compared directly. A
page carrying none comes from an index written before PM-040, and is compared against its `domain` path instead —
either equal to the bucket or prefixed by `bucket + "/"`. Without that second branch, journey lookup returns
nothing until the first rescan, which the existing full-stack API test demonstrates.

The default preserves current behaviour for every existing Wiki Root with no new file, which is what keeps this ticket
free of a migration. The comparison also tightens from prefix to exact bucket match, so a sibling directory such as
`e2e-testing-archive` no longer matches by accident.

## API Contract

No change. `KnowledgeWiki` and `KnowledgePage` gain fields; every existing route, status, and error contract is
unchanged. Added JSON fields are additive and omitted when empty, so existing clients are unaffected.

## Design Decisions

| Decision                               | Rationale                                                                                  |
|----------------------------------------|--------------------------------------------------------------------------------------------|
| Detection after `ResolveRelationships` | The tier rule is a property of the whole page set; per-file parsing cannot see it.         |
| `tiers` replaces, roles merge          | Curators override detection to remove a wrong tier; roles have nothing to merge with.      |
| Exact bucket match, not prefix         | Prefix matching silently captures sibling directories sharing the bucket name as a prefix. |
| Path fallback for a pre-PM-040 index   | A stale index has no bucket on any page, so bucket-only matching would drop every journey. |
| Settings live at the Wiki Root         | Same placement as `workspace-settings.yaml`; travels with the wiki through Git.            |
| Fields omitted when empty              | A root-level page has no bucket; emitting empty strings would imply a bucket named "".     |
| No `web/` change                       | `Domain` still drives the tree and graph, keeping the risky surface out of this ticket.    |
