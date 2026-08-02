# UI Automation: PM-037

## Applicability

Browser validation is required because PM-037 adds the reusable workspace -> plan -> terminal Workbench journey and
changes visible behavior across Local, Agent-Backed Cloud, and Agentless Remote Snapshot workspaces.

## Runtime Inputs

| Input                     | Source                                                               |
|---------------------------|----------------------------------------------------------------------|
| Base URL                  | Supplied when executing                                              |
| Authentication            | Supplied when executing; do not commit secrets                       |
| Local workspace           | Writable isolated Git workspace with at least one indexed plan       |
| AI provider               | Installed, enabled, and authenticated provider suitable for test use |
| Agent-Backed workspace    | Optional Cloud test workspace with owner Agent connection controls   |
| Remote Snapshot workspace | Optional authorized commit-pinned Cloud test workspace               |

## Playbooks

| Playbook                                                                              | Covers                                                                                                               | Status |
|---------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|--------|
| [Scenario 01 — Orchestrate a plan terminal](scenario-01-orchestrate-plan-terminal.md) | First Canvas, spatial restore, plan/session Workbench, Local persistence, offline Agent, and Remote Snapshot states. | Draft  |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Ticket-local playbooks are working sources. This plan introduces a reusable browser journey, so
`plan.e2e-runbook` is true. No existing `wiki/e2e-testing/` directory was available during planning. After
implementation, run `wiki-enrich` to synthesize the canonical journey and verify it before handoff.
