# UI Automation: PM-036

## Applicability

Browser validation is required because this feature changes how users locate and execute browser runbooks from plan and Knowledge surfaces.

## Runtime Inputs

| Input          | Source                                         |
|----------------|------------------------------------------------|
| Base URL       | Supplied when executing                        |
| Authentication | Supplied when executing; do not commit secrets |
| Test data      | Supplied when executing                        |

## Playbooks

| Playbook                                                                       | Covers                                                                           | Status |
|--------------------------------------------------------------------------------|----------------------------------------------------------------------------------|--------|
| [Scenario 01 — Run E2E coverage from Quality](scenario-01-run-e2e-coverage.md) | Plan-local priority, Knowledge execution, result states, and AI session handoff. | Draft  |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Ticket-local playbooks are working sources. This plan changes reusable browser-journey coverage, so `plan.e2e-runbook` is true. After implementation, use `wiki-enrich` to synthesize the durable journey under `docs/e2e-testing/`.
