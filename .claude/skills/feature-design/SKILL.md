---
name: feature-design
description: Design and implement Discovery features from a ticket ID. Use when planning, scaffolding, or implementing a ticket that needs plans/{service}/{ticket-id}/ documents, minimal plan.yaml metadata, service ownership, track-based phases, verification, and phase commits.
---

# Feature Design

Use these resources:

- [Templates](references/FORMS.md)
- [Guidelines](references/REFERENCE.md)
- [Scaffold script](scripts/init_plan.sh)
- [Visual brief rendering](../feature-brief/SKILL.md)
- [Markdown formatting](../md-formatting/SKILL.md)
- [E2E browser validation](../e2e-testing/SKILL.md)
- [E2E wiki enrichment](../wiki-enrich/SKILL.md)

## Rules

- Read existing code first.
- Use short, clear English.
- Reuse existing names and patterns.
- Ask only when service ownership or requirements are ambiguous.
- Keep `plan.yaml` minimal.
- Implement one phase at a time.
- Verify before every commit.
- Commit a phase before starting the next phase.

## Plan a Feature

### 1. Collect Inputs

Require ticket ID, short description, owning service, and tracks. Tracks are `backend`, `frontend`, `devops`, or a comma-separated combination.

Supported services: `api`, `api-worker`, `article`, `customer`, `customer-worker`, `user`, `gateway`, `aggregate`, `translation`, `mail`, `salesapi`, `esb`, `commons`, `webapp`, `platform`.

Infer the service from affected modules. Confirm only when uncertain.

### 2. Explore

Read relevant code. Identify existing patterns, data stores, APIs, frontend state, tests, and verification commands. Do not plan from assumptions.

### 3. Check Related Plans

Search `plans/` for related tickets.

When one exists:

- Read its `README.md` and relevant designs.
- Reuse its terminology and decisions.
- Add a concise `Related Plans` section to the new README.

### 4. Scaffold

```bash
# Default: backend + frontend
bash .claude/skills/feature-design/scripts/init_plan.sh {ticket-id} {description} {service}

# Selected tracks
bash .claude/skills/feature-design/scripts/init_plan.sh {ticket-id} {description} {service} backend
bash .claude/skills/feature-design/scripts/init_plan.sh {ticket-id} {description} {service} backend,frontend,devops
```

The script creates:

```text
plans/{service}/{ticket-id}/
├── plan.yaml
├── README.md
├── scenario/scenario-00-overview.md
├── design/design-0N-{track}.md
├── automation/README.md
└── implementation-plan.md
```

`plan.yaml` starts with `draft` status. Plan Manager infers the rest.

### 5. Fill Documents

Use `references/FORMS.md`.

- Lock terminology in `README.md`.
- Clarify domain, API, frontend, and cross-cutting requirements.
- Add scenarios for happy paths and edge cases.
- Write only designs for selected tracks.
- Put `## Execution Flow` immediately after the implementation-plan overview. Show phase dependencies, parallel work,
  joins, and external gates in a compact fenced `text` diagram. Follow it with a short explanation of the critical
  path, safe parallel work, and true blockers. A linear plan still needs a one-line flow.
- Keep the execution-flow diagram, phase summary, prerequisites, and statuses consistent. Do not invent dependencies
  merely to order unrelated work, and do not hide application prerequisites inside a DevOps phase.
- Split implementation into ordered phases.
- Add deliverables, verification, and a draft commit to every phase.
- Set `plan.e2e-runbook: true` only when the plan creates or changes reusable user-journey coverage, including a backend change that affects a UI workflow. Leave it `false` when existing coverage is sufficient.
- Keep ticket-local browser playbooks in `automation/` as working sources. When `plan.e2e-runbook` is true, use `e2e-testing` to update them and `wiki-enrich` to synthesize the durable journey runbook.
- Run `/md-formatting` after editing tables or diagrams.

#### Code Block Policy

Only these are allowed in plan documents as fenced code blocks:

| Allowed                      | Example                                         |
|------------------------------|-------------------------------------------------|
| SQL scripts/changesets       | `CREATE TABLE`, `ALTER TABLE`, `CREATE INDEX`   |
| Class / interface names only | `class LineItemService` — no fields or methods  |
| Algorithms and pseudocode    | step-by-step logic, decision trees              |
| Dependency flow diagrams     | phase ordering, parallel branches, joins, gates |
| Formulas                     | ranking = (prev + next) / 2                     |

**Not allowed:** Java/TypeScript class bodies with fields or methods, Liquibase YAML changesets, JSON/YAML configuration, framework annotations, or any code that duplicates what belongs in the source files. Use tables or prose instead to describe fields, request/response shapes, and configuration.

### 6. Render the Brief

Every plan gets a visual brief. Once the documents are filled and `/md-formatting` has run, invoke
`feature-brief` for this ticket. It reads the documents you just wrote and produces
`plans/{service}/{ticket-id}/brief.html`, then add the `Visual Brief` line to the README's
`## Documents` list.

Do this before implementation starts. The brief is where figure-shaped gaps in a plan show up — a
flow you cannot draw is usually a flow that is not yet decided — and finding those while the plan is
still draft is the point.

The markdown stays the source of truth. When a phase changes the plan during implementation,
re-render the affected sections rather than editing `brief.html` by hand.

## Implement a Feature

Before starting, read `implementation-plan.md`, `git status`, and recent commits. Resume from the first incomplete phase.

For each phase:

1. Announce the phase and deliverables.
2. Read existing patterns in the affected module.
3. Implement only that phase.
4. Run focused tests and the phase verification command.
5. Stop if verification fails.
6. Update the phase status and affected planning documents.
7. Commit with the ticket ID and draft commit text.
8. Report the commit SHA before continuing.

Use these baseline checks when applicable:

```bash
# Backend
./gradlew :{service}:app:build --console=plain

# Frontend
cd webapp && npx tsc --noEmit && npx jest --passWithNoTests
```

## Finish

- Confirm implementation matches the plan.
- Update final terminology and file references.
- Run relevant full tests.
- Run `/md-formatting`.
- Re-render `brief.html` with `feature-brief` when implementation changed the plan.
- Keep phase commits separate.
- When `plan.e2e-runbook` is true, update the ticket-local playbook, run `wiki-enrich`, and verify that the reusable E2E wiki runbook covers the changed flow before handoff. Execute it when runtime inputs are available.

Use `feature-testing` for code-based test design and coverage work. Use `e2e-testing` for browser-agent validation after implementation.
