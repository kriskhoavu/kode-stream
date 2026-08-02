# UI Automation: PM-037

## Applicability

Browser validation is required because PM-037 introduces the reusable workspace -> plan -> session Canvas journey,
branch-safe terminal launch, placement restoration, and visible verification freshness.

## Runtime Inputs

| Input                      | Source                                                                                 |
|----------------------------|----------------------------------------------------------------------------------------|
| Base URL                   | Supplied when executing                                                                |
| Authentication             | Supplied when executing; do not commit secrets                                         |
| Local workspace            | Writable isolated Git workspace with at least one indexed plan and controllable branch |
| AI or terminal provider    | Installed, enabled, authenticated, and suitable for isolated test use                  |
| Verification configuration | Test-safe command that can complete deterministically                                  |
| Repository mutation        | Reversible isolated file change used to invalidate verification                        |

## Playbooks

| Playbook                                                                                      | Covers                                                                                                                      | Status |
|-----------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|--------|
| [Scenario 01 — Operate the focused Terminal Canvas](scenario-01-orchestrate-plan-terminal.md) | Arrange and restore nodes, branch-safe launch, session lifecycle, Git state, verification staleness, and keyboard recovery. | Ready  |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Ticket-local playbooks are working sources. This plan introduces a reusable browser journey, so
`plan.e2e-runbook` is true. No matching canonical journey existed in the configured `discovery-wiki` collection. PM-037
therefore introduces the canonical [[platform-terminal-canvas-journey]] journey during implementation enrichment.

Agent-Backed Cloud and Agentless Remote Snapshot sections were removed from this MVP playbook. They require separate
provider-capability delivery and future journey deltas.
