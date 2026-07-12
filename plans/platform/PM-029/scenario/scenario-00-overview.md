# Scenarios: PM-029 Overview

## Scenario List

| #   | Title                           | Description                                                    |
|-----|---------------------------------|----------------------------------------------------------------|
| 0   | Existing Runtime Verification   | Smoke and Critical continue to run workspace runtime profiles  |
| 1   | Configure Automation Repository | User connects an external Cypress or Playwright repository     |
| 2   | Link Specs To A Card            | User saves specs by accepting suggestions, browsing, or typing |
| 3   | Run Silent Automation           | User runs selected specs in the background                     |
| 4   | Run Visible Browser Automation  | User starts a headed Chrome/Chromium runner                    |
| 5   | Metadata-Based Discovery        | Suggestions come from `automation-test` in `plan.yaml`         |

---

# Scenario 0: Existing Runtime Verification

## Goal

Keep current Smoke and Critical behavior unchanged.

## Expected Result

| Area      | Expected Behavior                                         |
|-----------|-----------------------------------------------------------|
| Buttons   | `Run smoke verify` and `Run critical verify` stay visible |
| Commands  | Smoke and Critical use existing runtime command selection |
| Metadata  | No selected specs are required                            |
| Artifacts | Runtime and verify logs remain available                  |

---

# Scenario 1: Configure Automation Repository

## Goal

Allow a workspace to define where automation tests live and how to run selected specs.

## Flow

```text
User opens workspace Integrations
  -> opens Runtime and Verification details
  -> selects Automation tests tab
  -> enables automation
  -> browses or enters automation repository path
  -> selects Cypress or Playwright
  -> saves default environment, command template, and artifact paths
```

## Expected Result

The workspace stores validated automation settings without changing runtime smoke or critical behavior.

---

# Scenario 2: Link Specs To A Card

## Goal

Make a card's automation coverage explicit.

## Flow

```text
User opens item Quality tab
  -> user reviews selected specs
  -> user accepts discovered specs, browses the registered automation repo, or enters a path
  -> frontend saves selected specs in item plan.yaml
```

## Expected Result

Selected specs survive reload and become the only specs used by `Run automation tests`.

---

# Scenario 3: Run Silent Automation

## Goal

Run selected Cypress or Playwright specs without showing the browser.

## Flow

```text
User selects Silent mode
  -> clicks Run automation tests
  -> runtime prepare/up/health runs
  -> automation command runs in automation repo
  -> artifacts and logs appear in Quality
  -> runtime teardown runs
```

## Expected Result

The job shows runtime setup steps, automation step, automation log, runtime setup log, and any collected test artifacts.

---

# Scenario 4: Run Visible Browser Automation

## Goal

Run selected specs in a visible browser for observation/debugging.

## Expected Result

| Runner     | Expected Command Behavior                 |
|------------|-------------------------------------------|
| Cypress    | Headed Chrome args are included           |
| Playwright | Headed Chromium project args are included |

The run button shows an in-button progress bar and `Starting browser...` while the browser is launching or the job is running.

---

# Scenario 5: Metadata-Based Discovery

## Goal

Avoid Markdown scanning for automation suggestions.

## Flow

```text
Quality panel requests verification tests
  -> backend reads current item and likely automation repo plan.yaml files
  -> backend extracts non-empty automation-test[].path entries
  -> frontend displays them as discovered specs
```

## Expected Result

Discovery is fast and only returns structured `automation-test` metadata.
