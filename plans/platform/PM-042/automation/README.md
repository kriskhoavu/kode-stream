# UI Automation: PM-042

## Applicability

Browser validation is required because Focus mode, the Workbench tabs, and the Sessions page are user-visible
workflows on the Workbench page and a new page.

## Runtime Inputs

| Input                | Source                                                      |
|----------------------|-------------------------------------------------------------|
| Base URL             | Supplied when executing                                     |
| Authentication       | Supplied when executing; do not commit secrets              |
| Two local workspaces | Isolated writable Git workspaces with one indexed plan each |
| Test-safe provider   | Supplied when executing; must write one deterministic file  |

## Playbooks

| Playbook                                           | Covers                                                     | Status |
|----------------------------------------------------|------------------------------------------------------------|--------|
| [Scenario 1](scenario-01-session-focus-journey.md) | Focus, Sessions tab, Changes with a real edit, global Stop | Draft  |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

`plan.e2e-runbook` is true. After implementation, enrich the Canvas journey under `wiki/e2e-testing/` with
`wiki-enrich`.
