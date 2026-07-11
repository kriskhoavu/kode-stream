# PM-029: External Automation Verification Runner

PM-029 adds first-class card-linked automation test execution to Kode Stream. A workspace can point to an external automation repository, discover candidate Cypress or Playwright specs for the current item, persist explicit selected specs, and run those tests from the item workspace without replacing the existing runtime smoke and critical verification profiles.

## Related Plans

| Item                          | Relationship             | Key Context                                                                            |
|-------------------------------|--------------------------|----------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Verification baseline    | Established local verification and browser acceptance as part of the platform workflow |
| [PM-015](../PM-015/README.md) | Architecture conventions | Calls for clearer backend ownership and frontend feature-controller boundaries         |
| [PM-028](../PM-028/README.md) | Current item surface     | Keeps item detail and embedded Explorer as the main place for item-specific actions    |

## Glossary

| Term                         | Meaning                                                                                 | Code                                |
|------------------------------|-----------------------------------------------------------------------------------------|-------------------------------------|
| Runtime Verification Profile | Existing workspace-level `smoke`, `critical`, or `full` command                         | `VerifyProfile`                     |
| Automation Repository        | Separate local repository that owns Cypress or Playwright tests                         | `WorkspaceRuntimeConfig.automation` |
| Automation Runner            | Configured runner type and command template for the automation repository               | `AutomationRunnerConfig`            |
| Selected Spec                | Test file or spec folder explicitly linked to one item                                  | `plan.yaml` verification metadata   |
| Discovered Spec              | Candidate spec inferred from the automation repo plan docs or matching ticket structure | Test discovery service              |
| Automation Verification Job  | Verification job that runs selected specs after runtime startup and health checks       | `verification.Job`                  |
| Rendered Command             | Final shell command after template variables are substituted                            | Verification job metadata           |

## Components

| Layer      | Component                    | Purpose                                                                  |
|------------|------------------------------|--------------------------------------------------------------------------|
| Backend    | Runtime config normalization | Store and validate optional automation repository settings               |
| Backend    | Verification service         | Run automation commands inside the external repo and collect artifacts   |
| Backend    | Item metadata writer         | Persist selected specs for the current item in `plan.yaml`               |
| Backend    | Test discovery service       | Suggest specs from ticket-matching plan docs and spec path references    |
| Controller | Verification API             | Accept automation run input and return selected spec/job metadata        |
| Frontend   | Workspace runtime settings   | Configure automation repo, runner, environment, command, and artifacts   |
| Frontend   | Item verification harness    | Show runtime buttons, selected specs, suggestions, and automation action |

## Smoke, Critical, And Automation Roles

Current `Smoke verify` and `Critical verify` remain workspace-level runtime verification profiles. They are useful for fast generic checks:

- App or container boots correctly.
- Health endpoints pass.
- Core smoke command runs.
- Critical non-card-specific checks run.
- Fallback verification runs when no card tests are linked.

The new automation runner does not replace them in v1. It sits beside them:

| Action               | Scope        | Runs                                                                |
|----------------------|--------------|---------------------------------------------------------------------|
| Run smoke verify     | Workspace    | Existing `runtime.commands.verify.smoke` command                    |
| Run critical verify  | Workspace    | Existing `runtime.commands.verify.critical`, falling back to smoke  |
| Run automation tests | Current item | Selected Cypress or Playwright specs from the automation repository |

Longer term, Smoke and Critical can become profiles that optionally include automation specs. PM-029 keeps them separate to avoid confusing runtime health with feature automation coverage and to preserve backward compatibility for every workspace already using those buttons.

## Data Flow

```text
User opens item workspace
  -> frontend loads workspace runtime and item metadata
  -> frontend requests automation spec suggestions for this item
  -> user accepts or edits selected specs
  -> backend saves selected specs into item metadata
  -> user clicks Run automation tests
  -> verification service prepares runtime workspace
  -> runtime starts and health checks pass
  -> automation command runs in external automation repository
  -> verification artifacts are collected
  -> runtime teardown runs
  -> frontend polls and renders job status, steps, logs, and artifacts
```

## Design Decisions

| Decision                                               | Alternatives Considered                                 | Rationale                                                                            |
|--------------------------------------------------------|---------------------------------------------------------|--------------------------------------------------------------------------------------|
| Keep Smoke and Critical as runtime profiles            | Replace them with automation profiles                   | Existing workspaces rely on these commands for generic environment checks            |
| Add automation as a separate item action               | Add automation behind the existing smoke button         | Users need to see whether they are running environment verification or feature tests |
| Persist selected specs explicitly                      | Rely only on automatic discovery                        | Discovery is helpful but should not silently change what a card runs                 |
| Discover specs from ticket docs and spec path mentions | Require every spec to contain structured metadata first | Discovery testing repos already mention `cypress/e2e/...` paths in plan documents    |
| Run automation after `prepare`, `up`, and `health`     | Let the automation repo own full setup                  | Current workspace runtime already knows how to start the app under test              |
| Use command templates instead of hard-coding Cypress   | Add only Cypress-specific command generation            | Playwright migration should reuse the same integration model                         |
| Validate selected specs inside the automation repo     | Trust raw user-entered paths                            | Verification commands must not become arbitrary filesystem execution shortcuts       |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
