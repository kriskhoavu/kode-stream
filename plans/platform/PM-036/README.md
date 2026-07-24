# PM-036: E2E Quality Panels

## Overview

Make agent-driven browser runbooks visible and executable from the existing Quality experience. A plan shows its local E2E coverage first and then linked canonical coverage; a Knowledge page shows the same quality view only for an E2E wiki journey. Each view reads the latest Markdown result and launches the existing AI session dialog in a constrained E2E mode.

## Related Plans

| Plan                          | Relationship          | Reused Decision                                                                      |
|-------------------------------|-----------------------|--------------------------------------------------------------------------------------|
| [PM-020](../PM-020/README.md) | AI session foundation | Reuse provider, terminal, and embedded-session launch behavior.                      |
| [PM-022](../PM-022/README.md) | Knowledge foundation  | Reuse indexed wiki pages, frontmatter, and Knowledge Reader composition.             |
| [PM-029](../PM-029/README.md) | Quality foundation    | Keep runtime verification and repository automation separate from runbook execution. |

## Glossary

| Term              | Meaning                                                                         | Maps To                                |
|-------------------|---------------------------------------------------------------------------------|----------------------------------------|
| E2E runbook       | Provider-neutral Markdown browser journey.                                      | `automation/` and `docs/e2e-testing/`  |
| Local runbook     | Ticket-local working runbook owned by the selected plan.                        | `plans/{service}/{ticket}/automation/` |
| Canonical journey | Reusable E2E wiki page linked through `sourceRef`.                              | `docs/e2e-testing/**`                  |
| Latest result     | The most recent skill-written outcome for a runbook owner.                      | `automation/results/latest.md`         |
| E2E session mode  | AI launch configuration that locks the runbook context and `e2e-testing` skill. | AI session dialog                      |

## Components

| Layer    | Component          | Purpose                                                                    |
|----------|--------------------|----------------------------------------------------------------------------|
| Backend  | E2E runbook reader | Discover eligible Markdown runbooks and parse their latest results safely. |
| Backend  | AI launch service  | Launch a workspace-context E2E session without requiring an item ID.       |
| API      | E2E read routes    | Return plan or wiki runbook summaries to the UI.                           |
| Frontend | E2E Quality panel  | Reusable runbook cards, result state, and E2E launch action.               |
| Frontend | AI Session dialog  | Render constrained E2E mode while retaining provider and terminal choices. |
## Data Flow

Selected plan or E2E Knowledge page → E2E runbook reader → local runbooks and `sourceRef`-matched canonical journeys → result reader → shared E2E Quality panel → constrained AI Session dialog → agent executes `e2e-testing` and writes the latest result.

## Design Decisions

| Decision                                | Alternatives Considered                            | Rationale                                                                                            |
|-----------------------------------------|----------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Reuse the AI Session dialog             | Direct in-app browser executor; separate E2E modal | The skill owns Playwright MCP execution, runtime prompts, and write confirmations.                   |
| Read Markdown latest results            | New verification-job database                      | `automation/results/latest.md` is the documented source of truth and needs no duplicate persistence. |
| Match canonical coverage by `sourceRef` | Ticket-title search; semantic search               | Source references are deterministic and avoid showing unrelated journeys.                            |
| Show E2E only for E2E wiki pages        | Quality panel for every Knowledge page             | Keeps Knowledge focused and prevents a run action without a browser journey.                         |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
