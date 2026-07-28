# Frontend Design: PM-036 E2E Quality Panels

## Overview

Use one reusable E2E Quality panel in the plan Quality tab and the Knowledge Reader. It presents runbook cards and result metadata, but delegates execution to the existing AI Session dialog in a constrained E2E mode.

## API and State

| Concern          | Design                                                                                                    |
|------------------|-----------------------------------------------------------------------------------------------------------|
| Plan data        | Load item E2E runbooks with the existing item detail lifecycle.                                           |
| Knowledge data   | Load the selected page’s E2E summary only when its path is under `e2e-testing/`.                          |
| Result refresh   | Explicit refresh after AI launch and normal page/item reload; no background polling.                      |
| Evidence opening | Read workspace-relative evidence through the guarded workspace file API and show it in the shared viewer. |
| AI launch target | A discriminated target supports the current item or a workspace-relative wiki runbook.                    |
| Errors           | Render an inline diagnostic inside the E2E section without hiding runtime verification or wiki content.   |

## Components and Behavior

| Component                    | Responsibility                                                                                                                                          |
|------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `E2EQualityPanel`            | Shared runbook list, result state, diagnostics, and launch button.                                                                                      |
| Plan Quality integration     | Place the E2E section beneath existing runtime and automation controls.                                                                                 |
| Knowledge Reader integration | Render the panel only for an opened E2E wiki page.                                                                                                      |
| E2E dialog mode              | Lock runbook context, generated prompt/result destination, and the discovered `e2e-testing` skill ID; permit provider, terminal, and surface selection. |

Each runbook card shows title, source badge, workspace-relative path, latest status, provider/environment when present,
failed step for failed or blocked runs, and evidence links opened through the guarded workspace file reader. Canonical
coverage diagnostics remain visible when local plan coverage is still available.

Provider capability discovery uses the selected workspace ID. E2E launch remains disabled until the provider returns a
matching `e2e-testing` descriptor; the descriptor ID, rather than a display-name alias, is sent to the launch API.

## UX States

| State              | Plan Quality                                                         | Knowledge Reader                        |
|--------------------|----------------------------------------------------------------------|-----------------------------------------|
| Coverage available | Local cards first, canonical cards second.                           | Single selected canonical journey card. |
| Not run            | Show the runbook and `Not run` latest status.                        | Same.                                   |
| No coverage        | Explain that no local runbook or linked canonical journey was found. | Not shown because the page is not E2E.  |
| Skill unavailable  | Disable launch and link to AI provider configuration guidance.       | Same.                                   |
| Launch completed   | Announce session handoff and offer refresh.                          | Same.                                   |

## Accessibility and Styling

- Use headings and labeled status text rather than color alone.
- Preserve keyboard navigation, focus management, and modal behavior from the existing AI Session dialog.
- Reuse Quality panel spacing, cards, buttons, error styling, and dark/light theme tokens.

## Verification

- Add component tests for ordering, no-result state, failure metadata, and unavailable skill.
- Add Knowledge page tests that prove non-E2E pages do not render the panel.
- Add dialog tests for immutable E2E fields and editable provider/terminal/surface controls.
