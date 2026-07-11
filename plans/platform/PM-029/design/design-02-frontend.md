# Frontend Design: PM-029

## Overview

The frontend extends workspace runtime settings and the item verification harness. Existing Smoke and Critical buttons stay in place. A new automation area lets users review discovered specs, persist selected specs, choose an environment, and run card-specific automation tests.

## Types And API

| Type                         | Purpose                                                                   |
|------------------------------|---------------------------------------------------------------------------|
| `WorkspaceAutomationConfig`  | Optional automation settings inside `WorkspaceRuntimeConfig`              |
| `VerificationRunMode`        | Distinguishes existing runtime runs from automation runs                  |
| `VerificationTestSelection`  | Selected specs and effective environment for one item                     |
| `DiscoveredVerificationSpec` | Candidate spec with path, source document, and confidence label           |
| `VerificationJob`            | Extended with mode, selected specs, automation repo, and rendered command |

The shared API client should add item verification-test endpoints and extend `createVerificationJob` input without breaking existing callers that only pass `profile`.

## Workspace Settings UI

| Control             | Behavior                                                                  |
|---------------------|---------------------------------------------------------------------------|
| Enable automation   | Shows or hides automation settings while preserving entered draft values  |
| Repository path     | Uses text input plus existing directory picker pattern                    |
| Runner selector     | Offers Cypress, Playwright, and Custom                                    |
| Default environment | Free text, defaulting to `local`                                          |
| Command template    | Editable command template with concise placeholder help outside the field |
| Artifact paths      | Reuses the existing repeatable artifact path input pattern                |

The settings UI lives under the current Runtime and verify integration card so users configure startup, health, runtime verify, and automation in one place.

## Item Verification Harness

| Area                    | Behavior                                                                |
|-------------------------|-------------------------------------------------------------------------|
| Runtime profile actions | Keep `Run smoke verify`, `Run critical verify`, and `Re-run latest`     |
| Automation status       | Shows whether automation is configured and whether specs are selected   |
| Discovered specs        | Shows suggestions from matching ticket docs and spec path references    |
| Selected specs          | Allows accepting suggestions, removing specs, and adding paths manually |
| Environment             | Defaults from workspace config and can be overridden for the card run   |
| Run automation tests    | Starts an automation verification job with selected specs               |

The existing polling, step rendering, artifact cards, preview, and open actions should be reused for automation jobs.

## States

| State                     | UI Behavior                                                 |
|---------------------------|-------------------------------------------------------------|
| Runtime not configured    | Existing runtime note remains                               |
| Automation not configured | Automation action disabled with setup note                  |
| Suggestions available     | Suggestions are shown separately from selected specs        |
| No specs selected         | Automation action disabled until at least one selected spec |
| Job running               | Existing verification busy and polling behavior applies     |
| Job failed                | Existing failure display shows failure type and failed step |

## Design Decisions

| Decision                                   | Rationale                                                              |
|--------------------------------------------|------------------------------------------------------------------------|
| Keep automation in the existing harness    | Users already look there for verification actions                      |
| Separate selected specs from suggestions   | Users need stable run intent instead of mutable discovery results      |
| Reuse existing artifact UI                 | Runtime and automation jobs should feel like one verification system   |
| Keep command details in workspace settings | Per-card UI should focus on specs and environment, not shell templates |
| Use existing dense settings styling        | This is an operational tool, not a marketing or onboarding surface     |
