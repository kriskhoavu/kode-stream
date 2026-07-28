# UI Playbook 1: Run E2E Coverage from Quality

## Goal

> Validate that a user can find plan and canonical browser coverage, inspect its latest result and evidence, and open a constrained AI handoff with the correct immutable runbook and result destination.

## Preconditions

- The final PM-036 build is running locally.
- Kode Stream has a local workspace containing this PM-036 plan.
- Kode Stream also has a Discovery workspace containing an E2E-enabled plan and its matching canonical
  `wiki/e2e-testing/` journey; DI-365 is suitable test data.
- The selected AI provider is enabled. Its discovered capabilities either include `e2e-testing` for the ready-state
  assertion or intentionally omit it for the blocked-state assertion.
- No credentials or production data are required for this Kode Stream navigation journey.

## Runtime Inputs

| Input          | Required Value                                                          |
|----------------|-------------------------------------------------------------------------|
| Base URL       | Supplied at run time for the local PM-036 build                         |
| Authentication | None for the local app                                                  |
| Test data      | PM-036 plan plus a Discovery plan/wiki pair with canonical E2E coverage |

## Steps

| #   | Agent Action                                                                                                                                    | Wait Condition                                                                         | Expected Result                                                                                                                                       | Write Class |
|-----|-------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Navigate to the Kode Stream work item for PM-036 and open **Quality**.                                                                          | Runtime verification, repository automation, and **E2E Test** sections finish loading. | Existing verification controls remain visible and the local PM-036 runbook appears in the E2E section.                                                | Read-only   |
| 2   | Inspect the E2E coverage groups.                                                                                                                | Runbook group counts and cards are visible.                                            | **Plan runbooks** appears before **Canonical wiki journeys** whenever both groups are present. A canonical-load diagnostic does not hide local cards. | Read-only   |
| 3   | Inspect the PM-036 runbook card before any result has been recorded.                                                                            | The card status text is visible.                                                       | The card reports **Not run** and does not infer a pass or failure.                                                                                    | Read-only   |
| 4   | Select **Run E2E test** on the PM-036 runbook.                                                                                                  | The **Run E2E test** dialog is visible and provider capabilities finish loading.       | The runbook path and prompt are immutable. The prompt names `plans/platform/PM-036/automation/results/latest.md` as the result destination.           | Read-only   |
| 5   | Inspect the required-skill state without selecting **Open session**.                                                                            | The capability status is visible.                                                      | A provider with `e2e-testing` shows a ready message and enables **Open session**; a provider without it shows configuration guidance and disables it. | Read-only   |
| 6   | Close the dialog, switch to the configured Discovery workspace, open its E2E-enabled plan, and return to **Quality**.                           | Local and canonical runbook discovery completes.                                       | Local plan runbooks are listed first and the matching canonical journey is listed second.                                                             | Read-only   |
| 7   | In **Knowledge**, open that canonical page under `wiki/e2e-testing/`.                                                                           | The Knowledge Reader and side panel finish loading.                                    | The same E2E Quality section appears for the canonical journey.                                                                                       | Read-only   |
| 8   | Open a Knowledge page outside `wiki/e2e-testing/`.                                                                                              | The non-E2E page finishes loading.                                                     | No E2E Quality section is shown.                                                                                                                      | Read-only   |
| 9   | If the selected test-data result lists evidence, open one evidence reference from its runbook card. Otherwise record this assertion as not run. | The guarded workspace file read finishes.                                              | A supported evidence file opens in the shared preview; an unavailable file reports an inline error without hiding coverage.                           | Read-only   |

## Evidence

- Capture the PM-036 Quality panel showing the local runbook and its current status.
- Capture the constrained dialog showing the immutable prompt and required-skill readiness or blocked guidance.
- Capture the Discovery plan with local-before-canonical ordering.
- Capture the canonical Knowledge page with its E2E Quality side panel.
- Capture an evidence preview when the runtime test data contains a safe evidence reference.

## Cleanup

- Return to the original Kode Stream workspace and close any open AI launch or evidence-preview dialogs.
- Do not launch an AI session, change provider settings, or modify result fixtures during this read-only journey.
