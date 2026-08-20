# PM-040: Detected Wiki Taxonomy

PM-040 teaches Knowledge the structural shape of a Wiki Root instead of hardcoding it. Bucket, area, and tier are
inferred from the directory layout of the pages already indexed, and an optional `knowledge-settings.yaml` names what
detection cannot infer: what a bucket *means*. The first consumer is the E2E journey lookup, which today matches the
literal string `e2e-testing` and afterwards asks for the bucket carrying the `journeys` role. No user-visible surface
changes in this ticket.

## Related Plans

| Ticket                        | Relationship          | Key Context                                                                        |
|-------------------------------|-----------------------|------------------------------------------------------------------------------------|
| [PM-022](../PM-022/README.md) | Knowledge foundation  | Defines Wiki Root, Wiki Page, and Domain. PM-040 adds structure above Domain.      |
| [PM-036](../PM-036/README.md) | Consumes the taxonomy | Canonical journeys are scoped to `wiki/e2e-testing/**`; that literal becomes role. |
| [PM-023](../PM-023/README.md) | Packaging conventions | Detection is a domain concern under `internal/knowledge`, not a transport concern. |

## Scope

In scope: taxonomy detection, the settings override, and de-hardcoding the journey bucket.

Out of scope, and deliberately so — each is its own ticket:

- Parsing `lastVerified` or claim-status front matter.
- Parsing `chunkId` / `keywords` chunk comments for retrieval.
- Grouping the Knowledge browser tree or graph filters by bucket and tier.

## Glossary

Extends the PM-022 glossary rather than replacing it. `Domain` keeps its existing meaning and its existing behaviour.

| Term        | Meaning                                                                | Maps To (code)  |
|-------------|------------------------------------------------------------------------|-----------------|
| Bucket      | First path segment under the Wiki Root                                 | `KnowledgePage` |
| Area        | One or more subject segments between bucket and tier                   | `KnowledgePage` |
| Tier        | Directory name that recurs under two or more parents inside one bucket | `KnowledgePage` |
| Bucket Role | Declared meaning of a bucket; detection cannot infer it                | `TaxonomyRole`  |
| Taxonomy    | Detected tier name set plus declared bucket roles for one Wiki Root    | `Taxonomy`      |
| Journeys    | Role marking the bucket that holds reusable E2E journeys               | `RoleJourneys`  |

## Components

| Layer   | Component                        | Purpose                                                        |
|---------|----------------------------------|----------------------------------------------------------------|
| Domain  | `knowledge/taxonomy.go`          | Detect tiers, classify a page path into bucket, area, and tier |
| Domain  | `knowledge/taxonomy_settings.go` | Read and validate `knowledge-settings.yaml`, warn on defects   |
| Domain  | `knowledge/models.go`            | Carry `bucket`, `area`, and `tier` on a page                   |
| Service | `knowledge/detector.go`          | Classify the page set once, after relationships resolve        |
| Service | `knowledge/knowledge_service.go` | Select the journeys bucket by role rather than by literal      |

## Detection Rule

A directory name is a tier when, inside at least one bucket, it appears under two or more distinct parents.

Measured against the reference Wiki Root: `reference` has five distinct parents inside `domains` and `concepts` has
three, while every subject-matter name has exactly one per bucket. The tier name set is global once learned, so a tier
is still recognised in a bucket where it appears only once.

Within-bucket counting is load-bearing. Counting parents globally instead promotes `offer` and `master-data` to tiers,
because each appears under both `domains` and `e2e-testing`.

## Data Flow

```text
Wiki Root page paths
    ↓
DetectTaxonomy — tier names by within-bucket recurrence
    ↓
knowledge-settings.yaml (optional) — replaces tiers, adds bucket roles
    ↓
Classify each page → bucket, area, tier
    ↓
Journey lookup resolves the bucket holding role journeys
```

## Structural Shapes

All three occur in the reference Wiki Root and all three must classify correctly.

| Path                                            | Bucket        | Area                  | Tier        |
|-------------------------------------------------|---------------|-----------------------|-------------|
| `domains/offer/concepts/approval.md`            | `domains`     | `offer`               | `concepts`  |
| `domains/master-data/article/reference/spec.md` | `domains`     | `master-data/article` | `reference` |
| `platform/concepts/deployment-rollback.md`      | `platform`    |                       | `concepts`  |
| `e2e-testing/offer/approval.md`                 | `e2e-testing` | `offer`               |             |
| `index.md`                                      |               |                       |             |

## Design Decisions

| Decision                                  | Alternatives Considered                    | Rationale                                                                                     |
|-------------------------------------------|--------------------------------------------|-----------------------------------------------------------------------------------------------|
| Detect tiers, declare roles               | Declare everything; detect everything      | Layout is observable and roles are not. Splitting on that line keeps zero-config working.     |
| Tier by within-bucket recurrence          | Fixed depth; global recurrence             | Depth breaks on `platform/concepts` and nested areas; global recurrence misfiles `offer`.     |
| Keep `Domain` unchanged                   | Replace `Domain` with the new fields       | Browser tree and graph read `Domain`; leaving it alone keeps this ticket free of UI changes.  |
| Role `journeys` defaults to `e2e-testing` | Require a settings file to enable journeys | Existing Wiki Roots keep working with no new file. The literal becomes a default, not a rule. |
| Malformed settings degrade to detection   | Fail the scan                              | Matches PM-022 warning behaviour; a bad config must not blank a Wiki.                         |
| Absent evidence yields no tier            | Guess a tier from a single directory       | One example is not a pattern. The override is the escape hatch.                               |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
- [Visual Brief](brief.html)
