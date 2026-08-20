# Planning Document Templates

Use these templates when generating planning documents in `plans/{service}/{ticket-id}/`.
For the full workflow, see [SKILL.md](SKILL.md).

---

## plan.yaml Template

```yaml
plan:
  status: draft
  wiki_enriched: false
  e2e-runbook: false
automation-test:
  - path: ""
```

Keep this file minimal. Plan Manager infers scope and identifier from the directory path, title from the `README.md`
heading, and documents from conventional Markdown paths. Set `path` to `automation/README.md`; this is a
provider-neutral browser-playbook hub, not a Cypress or Playwright spec. For a non-UI plan, the hub records why browser
validation is not applicable. Add `owner`, `tags`, or a title override only when needed.

Set `plan.e2e-runbook` to `true` only when this ticket creates or changes reusable browser coverage. It can be true for a
backend plan when an API change affects an existing user journey.

---

## Automation Hub Template

````markdown
# UI Automation: {Ticket ID}

## Applicability

Browser validation is required because {user-visible workflow}, or `Not applicable` because {reason}.

## Runtime Inputs

| Input | Source |
|-------|--------|
| Base URL | Supplied when executing |
| Authentication | Supplied when executing; do not commit secrets |
| Test data | Supplied when executing |

## Playbooks

| Playbook | Covers | Status |
|----------|--------|--------|
| [Scenario 1](scenario-01-{workflow}.md) | {happy path and relevant states} | Draft |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Ticket-local playbooks are working sources. When `plan.e2e-runbook: true`, enrich the canonical journey under
`wiki/e2e-testing/` with `wiki-enrich`.
````

---

## README.md Template

````markdown
# {Ticket ID}: {Feature Name}

One-paragraph description of what this feature does and why.

## Glossary

| Term | Meaning | Code |
|------|---------|------|

## Data Flow

> Compact inline tree or arrow notation — no wide ASCII boxes

## Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|

## Documents

- [Design](design/design-01-backend.md)
- [Scenarios](scenario/scenario-00-overview.md)
- [Implementation Plan](implementation-plan.md)
- [Visual Brief](brief.html)
````

The `Visual Brief` line is present on every plan — step 6 of the Plan track renders it. It is
produced by the `feature-brief` skill and is a rendering of these documents, not a source of truth.
Omit the line only for a plan predating the brief that has not been backfilled yet.

---

## Scenario Template

````markdown
# Scenario {N}: {Title}

## Goal

> One sentence describing what the user is trying to achieve.

## Starting State

| #   | Title | Summary |
|-----|-------|---------|

## Visual State (Before)

> ASCII tree / graph / relationship before execution

## Execution Flows

### Flow {N}.1: {Flow Name}

```text 
User Action 
    ↓ 
Frontend Validation 
    ↓ 
API Request 
    ↓ 
Application Service 
    ↓ 
Domain Logic 
    ↓ 
Persistence 
    ↓ 
API Response 
```

## Visual State (After)

> ASCII tree / graph / relationship after execution 
````

---

## Backend Design Document

````markdown
# Backend Design: {Component Name}

## Overview

> Brief description of the backend design for this feature, including any new entities, database schema changes, and API contract.

## Data Model

### Entity: {EntityName}

| Field | Type | Purpose |
|-------|------|---------|

### Database Schema

```sql
CREATE TABLE ...
```

## API Contract

| Method | Endpoint | Request | Response |
|--------|----------|---------|----------|

## Design Decisions

| Decision | Rationale |
|----------|-----------|
````
## Frontend Design: {Component Name}

````markdown
## Overview

> Brief description of the frontend design for this feature, including any new components, hooks, or state management.

## Data Model

### Entity: {EntityName}

| Field | Type | Purpose |
|-------|------|---------|

## State Management

| Store | Responsibility |
|-------|----------------|

## Design Decisions

| Decision | Rationale |
|----------|-----------|
````

---

## Implementation Plan Template

````markdown
# Implementation Plan: {Ticket ID} - {Feature Name}

## Overview

Brief description of what will be implemented.

## Execution Flow

```text
B1 Foundation ─────┐
                   ├─> B3 Integration ─> B4 Verification
F1 Parallel work ──┘
External gate ──────────────────────────┘
```

Replace the example with the real phase IDs. Explain the critical path, which branches can run in parallel, where they
join, and which external gates genuinely block progress. Keep this description to one short paragraph. For a linear
plan, use one left-to-right line and state that no phases run in parallel.

## Phases Summary

> Keep only rows for the selected tracks. Update Status as phases complete.

| Phase | Name              | Status |
|-------|-------------------|--------|
| B1    | Domain Layer      |        |
| B2    | Client Model      |        |
| B3    | Application Layer |        |
| B4    | Index Layer       |        |
| F1    | Types & Service   |        |
| F2    | State + Hooks     |        |
| F3    | Components        |        |
| F4    | Styling + i18n    |        |

## Backend Phases

### Phase B1: Domain Layer

**Deliverables:**

- [ ] `{Entity}.java` — JPA entity with correct column names
- [ ] `{Entity}Repository.java` — Spring Data repository
- [ ] Liquibase changeset

**Verification:** `./gradlew :{service}:domain:build`

**Commit:** `{ticket-id}: Add {Entity} domain layer`

---

### Phase B2: Client Model

**Deliverables:**

- [ ] `{Feature}Model.java`
- [ ] `Create{Feature}Request.java`
- [ ] `Update{Feature}Request.java`

**Verification:** `./gradlew :{service}:app:compileJava`

**Commit:** `{ticket-id}: Add {Feature} client model DTOs`

---

### Phase B3: Application Layer

**Deliverables:**

- [ ] `{Feature}Service.java` interface with Javadoc
- [ ] `{Feature}ServiceImpl.java`
- [ ] `{Feature}Controller.java` with OpenAPI annotations
- [ ] `{Feature}ModelBuilder.java`

**Verification:** `./gradlew :{service}:app:build`

**Commit:** `{ticket-id}: Add {Feature} service and controller`

---

### Phase B4: Index / Integration Layer (if Solr-backed)

**Deliverables:**

- [ ] Update `{Domain}Index.java`
- [ ] Update `{Domain}IndexService.java`

**Verification:** `./gradlew :{service}:index:build`

**Commit:** `{ticket-id}: Update Solr index for {Feature}`

---

## Frontend Phases

> Follow `skills/frontend-feature/SKILL.md` for detailed templates.

### Phase F1: Types & Service

**Deliverables:**

- [ ] `service/{feature}/type.ts`
- [ ] `service/{feature}/index.ts`
- [ ] Registered in `serviceStore.ts` and `ApplicationService` type

**Verification:** `cd webapp && npx tsc --noEmit`

**Commit:** `{ticket-id}: Add {Feature} frontend service layer`

---

### Phase F2: State + Query Hooks

**Deliverables:**

- [ ] Zustand slice extended or new standalone store
- [ ] Query keys in `queryKeys.ts`
- [ ] `use{Feature}Query.ts` (useQuery + useMutation)

**Verification:** `cd webapp && npx tsc --noEmit`

**Commit:** `{ticket-id}: Add {Feature} state and query hooks`

---

### Phase F3: Components

**Deliverables:**

- [ ] Main component (memo-wrapped orchestrator)
- [ ] Sub-components in `component/`
- [ ] Feature hooks in `hook/`

**Verification:** `cd webapp && npx jest --testPathPattern="{Feature}" --passWithNoTests`

**Commit:** `{ticket-id}: Add {Feature} components`

---

### Phase F4: Styling + Translations

**Deliverables:**

- [ ] `{Feature}.styled.ts` with `Styled*` prefix, `CC_*` colors
- [ ] Translation keys in `localization/*.json`
- [ ] Visual parity check: normal mode ↔ feature mode (no flicker)

**Verification:** Visual review in browser. Run full jest suite.

**Commit:** `{ticket-id}: Add {Feature} styling and translations`

---

## Post-Implementation Checklist

- [ ] Update `plans/{ticket-id}/` docs to reflect any naming changes
- [ ] Update `changelog.md` with feature summary
- [ ] Check `instructions/` for any new conventions to document
- [ ] Verify no terminology drift: `grep -r "{old-term}" --include="*.java" --include="*.ts"`
- [ ] PR description references planning docs

## Testing Strategy

- Backend: Unit tests for service logic, integration tests for controller endpoints
- Frontend: Component render tests, query hook tests, interaction tests

## Migration Notes

- Liquibase changesets in `api/domain/src/main/resources/db/changelog/`
- Format: `changeset-{NN}-{description}.yaml`
````

---

## DevOps Templates

> Use these when `init_plan.sh` is run with `devops` as the third argument.

### design-01-infrastructure.md Template

````markdown
# Infrastructure Design: {Ticket ID}

## Gradle Tasks

| Task Name | Group | Modules | Purpose |
|-----------|-------|---------|---------|

## Module Groups

| Group | Modules | Gradle Task |
|-------|---------|-------------|

## Docker / Registry

| Image | Registry | Tag Pattern |
|-------|----------|-------------|

## Helm / Kubernetes (if applicable)

- HelmChart changes: TODO
- K8S resource changes (ConfigMap, Deployment, Service): TODO

## Design Decisions

| Decision | Rationale |
|----------|-----------|
````

---

### design-02-pipeline.md Template

````markdown
# Pipeline Design: {Ticket ID}

## Tag Patterns & Behavior

| Tag Pattern  | Build | Publish Internal | Publish External | Deploy |
|--------------|-------|------------------|------------------|--------|
| nightly-*    | ✅    | ✅               | ❌               | ✅     |
| release-*    | ✅    | ✅               | ❌               | ✅     |
| cc-release-* | ❌    | ❌               | ✅ (re-tag only) | ❌     |

## Pipeline Stages

```
Prepare → Build (parallel) → Deploy
            ├── Group 1
            └── Group 2
```

## Options Considered

### Option A: {Description}

TODO: Describe option A.

### Option B: {Description}

TODO: Describe option B.

## Comparison

| Criterion | Option A | Option B |
|-----------|----------|----------|

## Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
````

---

### DevOps Implementation Plan Template

````markdown
# Implementation Plan: {Ticket ID} - {Feature Name}

## Overview

Brief description of the CI/CD or infrastructure change.

## Execution Flow

```text
C1 Build artifacts ───────┐
                          ├─> C3 Deploy ─> C4 Live verification
C2 Environment inputs ────┘
External approval ──────────────────────┘
```

Replace the example with the real phases and gates. Explain what can run in parallel, the join required before deploy,
the critical path, and any external dependency that cannot be solved inside the repository. Do not call a phase
blocked when useful work in that phase can start.

## Phases Summary

| Phase | Name                   | Status | Verification                         |
|-------|------------------------|--------|--------------------------------------|
| C1    | Build / Infrastructure |        | `./gradlew tasks --group publishing` |
| C2    | Pipeline / CI          |        | Jenkins dry-run / PR build           |

## DevOps Phases

### Phase C1: Build / Infrastructure

**Deliverables:**

- [ ] Gradle task changes (`build.gradle`)
- [ ] HelmChart / K8S resource changes (if applicable)

**Verification:** `./gradlew tasks --group publishing`

**Commit:** `{ticket-id}: Add {description} Gradle tasks`

---

### Phase C2: Pipeline / CI

**Deliverables:**

- [ ] Jenkinsfile changes

**Verification:** Dry-run / lint Jenkinsfile locally

**Commit:** `{ticket-id}: Refactor Jenkinsfile for {description}`

---

## Post-Implementation Checklist

- [ ] Update `plans/{ticket-id}/` docs to reflect any naming changes
- [ ] Update `changelog.md` with summary
- [ ] Verify pipeline runs correctly on a test tag
- [ ] PR description references planning docs
````
