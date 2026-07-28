# Scenarios: PM-036 Overview

## Scenario List

| #   | Title                              | Description                                               |
|-----|------------------------------------|-----------------------------------------------------------|
| 1   | Run selected plan coverage         | Start a constrained E2E AI session from the current plan. |
| 2   | Inspect canonical wiki coverage    | Read and run an E2E journey from Knowledge.               |
| 3   | Handle missing coverage or results | Explain the next action without a false pass/fail result. |

---

# Shared Starting State

## Starting State

- The selected item is a plan under `plans/{service}/{ticket}`.
- A plan may have `plan.e2e-runbook: true` and Markdown playbooks under `automation/`.
- The configured Knowledge wiki may contain canonical pages under `wiki/e2e-testing/`.
- No browser provider, base URL, account reference, or test data is stored in the app.

## Visual State (Before)

The existing Quality panel contains runtime verification and repository automation. Knowledge renders the selected wiki page without Quality controls.

## Flow 1: Run selected plan coverage

1. The user opens the current plan’s **Quality** tab.
2. The E2E section lists ticket-local runbooks first, followed by canonical journeys whose `sourceRef` matches the plan’s automation files.
3. The user selects a runbook and chooses **Run E2E test**.
4. The AI Session dialog opens in E2E mode with the runbook path, generated instruction, and `e2e-testing` skill fixed.
5. The user chooses the available provider and terminal surface, then launches the session.
6. The agent requests any missing runtime inputs, follows the runbook, and writes the latest result.
7. Refreshing the panel displays the reported status and available evidence references.

## Flow 2: Inspect canonical wiki coverage

1. The user opens a page under `wiki/e2e-testing/` in Knowledge.
2. The Knowledge Reader renders the same E2E section for that page only.
3. The user reviews the latest result or launches the constrained E2E session with the wiki page as workspace context.

## Flow 3: Missing coverage or result

1. A plan without an eligible local runbook or matching canonical `sourceRef` shows **No E2E coverage**.
2. A runbook without a readable latest result shows **Not run** rather than an error or pass.
3. An unavailable `e2e-testing` capability disables launch and explains how to configure the AI provider.

## Acceptance Criteria

- The selected plan’s local coverage appears before canonical coverage.
- Runtime verification and Cypress/Playwright repository automation retain their existing controls and semantics.
- Only E2E wiki pages expose the Knowledge Quality section.
- E2E launch never persists secrets or runtime inputs in the plan or app configuration.
