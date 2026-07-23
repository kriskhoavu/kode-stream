# Scenario 01: Run E2E Coverage from Quality

## Goal

Validate that a user can identify the selected plan’s browser coverage, hand the correct runbook to an AI agent, and inspect the recorded outcome without mixing it with local automation-runner results.

## Preconditions and Runtime Inputs

| Input          | Source                                                         |
|----------------|----------------------------------------------------------------|
| Base URL       | Supplied when executing in a non-production environment.       |
| Authentication | Supplied when executing as a non-secret account reference.     |
| Test data      | Supplied when executing; use isolated data for any write path. |

## Plan Quality Coverage

1. Open a plan marked `plan.e2e-runbook: true` and select **Quality**. Wait until the E2E section loads.
2. Confirm that the selected plan’s local runbook cards appear before canonical journey cards.
3. Open a local runbook card. Confirm its title, source, path, and latest result state are visible.
4. Select **Run E2E test**. Confirm the AI Session dialog identifies the selected runbook and requires the `e2e-testing` skill while leaving provider and terminal choices available.
5. Launch the agent. If base URL, authentication reference, or test data is missing, confirm the agent requests it before execution.

## Knowledge Coverage

1. Open a canonical page under `docs/e2e-testing/`. Wait until the Knowledge Reader finishes loading.
2. Confirm the same E2E Quality section appears for that journey.
3. Open a non-E2E Knowledge page. Confirm the E2E Quality section is absent.

## Result States

1. When no `automation/results/latest.md` file exists, confirm the card reads **Not run**.
2. When a latest result is present, confirm status, provider, environment, failed step when applicable, and evidence references match the file.
3. If the `e2e-testing` capability is unavailable, confirm launch is disabled with configuration guidance.

## Write Boundary

The Quality panel does not execute browser actions itself. The launched agent follows the runbook’s write classification and requests confirmation before create, update, delete, export, send, or other external effects.
