# UI Automation: PM-043

## Applicability

Browser validation is required: the card cluster, the Agents tab, the agent pane and the dock hand-off are
user-visible workflows on the Workstream page.

## Runtime Inputs

| Input                | Source                                                      |
|----------------------|-------------------------------------------------------------|
| Base URL             | Supplied when executing                                     |
| One local workspace  | Isolated writable Git workspace with one indexed plan       |
| Claude Code          | Installed and signed in on the executing machine            |

## Playbooks

| Playbook                                         | Covers                                                          | Status |
|--------------------------------------------------|-----------------------------------------------------------------|--------|
| [Scenario 1](scenario-01-card-agent-journey.md) | Launch from a card, prompt, dock hand-off, Canvas, Continue here | Draft  |

## Latest Result

- [Latest execution result](results/latest.md)
