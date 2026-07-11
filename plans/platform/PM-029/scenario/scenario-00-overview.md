# Scenarios: PM-029 Overview

## Scenario List

| #   | Title                                  | Description                                                      |
|-----|----------------------------------------|------------------------------------------------------------------|
| 0   | Existing Runtime Verification          | Workspace continues to run Smoke and Critical without automation |
| 1   | Configure Automation Repository        | User connects an external Cypress or Playwright test repository  |
| 2   | Link Tests To A Card                   | User reviews discovered specs and saves explicit selected specs  |
| 3   | Run Card Automation Tests              | User starts selected tests from the item verification harness    |
| 4   | Missing Or Invalid Automation Settings | UI and API block unsafe or incomplete automation runs            |

---

# Scenario 0: Existing Runtime Verification

## Goal

Keep current Smoke and Critical verification behavior unchanged for workspaces that do not configure automation tests.

## Starting State

| #   | State             | Summary                                                                       |
|-----|-------------------|-------------------------------------------------------------------------------|
| 1   | Runtime exists    | Workspace has `runtime.commands.verify.smoke` and optional `critical` command |
| 2   | Automation absent | Workspace runtime has no automation repository configured                     |
| 3   | User on item      | Item workspace displays the existing verification harness                     |

## Execution Flow

```text
User clicks Run smoke verify
  -> frontend posts profile smoke
  -> API creates a verification job
  -> verification service runs prepare, up, health, verify, down
  -> frontend shows job steps and artifacts
```

## Expected Result

| Area      | Expected Behavior                                         |
|-----------|-----------------------------------------------------------|
| Buttons   | `Run smoke verify` and `Run critical verify` stay visible |
| Commands  | Smoke and Critical use existing runtime command selection |
| Metadata  | No card-level test metadata is required                   |
| Artifacts | Existing runtime and verify logs remain available         |

---

# Scenario 1: Configure Automation Repository

## Goal

Allow a workspace to define where automation tests live and how to run selected specs.

## Starting State

| #   | State            | Summary                                                               |
|-----|------------------|-----------------------------------------------------------------------|
| 1   | Runtime enabled  | Workspace already has startup, teardown, health, and verify commands  |
| 2   | Test repo exists | Example path: `/Users/kdvu/Documents/0. CC/1. Discovery/testing`      |
| 3   | Cypress scripts  | Test repo supports `npx cypress run --spec` and environment selection |

## Execution Flow

```text
User opens workspace integration settings
  -> user enables Automation tests
  -> user selects repository path
  -> user selects runner Cypress
  -> user sets default environment local
  -> user reviews command template and artifact paths
  -> frontend saves runtime config
  -> backend normalizes and validates automation settings
```

## Expected Result

| Area          | Expected Behavior                                                 |
|---------------|-------------------------------------------------------------------|
| Persistence   | Automation settings are stored with workspace runtime config      |
| Validation    | Missing or non-directory repository path is rejected when enabled |
| Defaults      | Cypress default command and artifact paths are available          |
| Compatibility | Existing runtime fields still save and load unchanged             |

---

# Scenario 2: Link Tests To A Card

## Goal

Make the card's automation coverage explicit while using discovery to reduce manual entry.

## Starting State

| #   | State           | Summary                                                           |
|-----|-----------------|-------------------------------------------------------------------|
| 1   | Current item    | Item identifier is available, such as `DI-390`                    |
| 2   | Test repo plans | Automation repo has `plans/DI-390/test-plan.md`                   |
| 3   | Spec references | Test docs mention paths such as `cypress/e2e/02-Create-Offer/...` |

## Execution Flow

```text
User opens item verification harness
  -> frontend requests discovered specs for item identifier
  -> backend scans matching automation plan docs for spec references
  -> frontend shows suggestions
  -> user accepts, removes, or manually adds specs
  -> frontend saves selected specs to item metadata
```

## Expected Result

| Area        | Expected Behavior                                                     |
|-------------|-----------------------------------------------------------------------|
| Discovery   | Matching ticket docs and spec path references appear as suggestions   |
| Selection   | User-approved specs become the run list                               |
| Persistence | Selected specs survive reload because they are saved in item metadata |
| Safety      | Paths outside the automation repo are rejected                        |

---

# Scenario 3: Run Card Automation Tests

## Goal

Run selected Cypress or Playwright specs from the current card after the workspace runtime is healthy.

## Starting State

| #   | State                 | Summary                                                   |
|-----|-----------------------|-----------------------------------------------------------|
| 1   | Runtime configured    | Workspace can prepare, start, health check, and tear down |
| 2   | Automation configured | Automation repository and command template are valid      |
| 3   | Specs selected        | Current item has one or more selected specs               |

## Execution Flow

```text
User clicks Run automation tests
  -> frontend posts automation mode, selected specs, and environment
  -> API creates a verification job
  -> verification service runs prepare, up, health
  -> verification service renders the automation command
  -> command runs in the automation repository
  -> artifacts are collected from automation artifact paths
  -> runtime teardown runs
  -> frontend polls and renders final status
```

## Expected Result

| Area      | Expected Behavior                                                     |
|-----------|-----------------------------------------------------------------------|
| Step list | Job shows runtime steps and an automation test step                   |
| Logs      | Automation output is written to a dedicated test log                  |
| Artifacts | Reports, screenshots, videos, and logs appear in existing artifact UI |
| Failure   | Test command failure marks the job as `test_failure`                  |
| Teardown  | Runtime teardown is attempted after pass or fail                      |

---

# Scenario 4: Missing Or Invalid Automation Settings

## Goal

Prevent confusing or unsafe automation runs.

## Edge Cases

| Case                          | Expected Behavior                                             |
|-------------------------------|---------------------------------------------------------------|
| No automation repo configured | `Run automation tests` is disabled with a clear note          |
| No selected specs             | User is prompted to select or accept discovered specs first   |
| Spec path escapes repo        | API rejects the run and does not execute a command            |
| Command template missing      | API reports invalid automation configuration                  |
| Runtime startup fails         | Automation step is skipped and failure remains `boot_failure` |
| Test command fails            | Job status becomes failed with `test_failure`                 |
