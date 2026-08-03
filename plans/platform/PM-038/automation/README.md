# UI Automation: PM-038

## Applicability

Browser validation is required because PM-038 changes branch context, review navigation, guarded checkout, Canvas
selection, and explicit repository writes.

## Runtime Inputs

| Input          | Source                                                                    |
|----------------|---------------------------------------------------------------------------|
| Base URL       | Supplied when executing                                                   |
| Authentication | Supplied when executing; do not commit secrets                            |
| Test workspace | Isolated disposable Git repository with two branches and structured plans |

## Playbooks

| Playbook                                                | Covers                                                         | Status                   |
|---------------------------------------------------------|----------------------------------------------------------------|--------------------------|
| [Scenario 1](scenario-01-review-and-activate-branch.md) | Review, stale-route isolation, import, checkout, and alignment | Passed; R9 delta not run |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Ticket-local playbooks are working sources. `wiki-enrich` synthesizes this delta into the canonical cross-domain Git
and workspace journey.
