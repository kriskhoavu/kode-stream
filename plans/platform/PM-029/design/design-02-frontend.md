# Frontend Design: PM-029

## Overview

The frontend exposes automation verification through workspace settings and the item Quality panel. Runtime verification remains visible and separate from automation. Users can configure the automation repo, browse specs from the registered repo, choose silent or visible browser mode, and run selected specs from the current item.

## Workspace Settings

Workspace details use top-level sections for Overview, Health, and Integrations. Integration details use a back button and focused detail views so each integration does not show unrelated settings.

Runtime settings contain sibling tabs:

| Tab                  | Purpose                                                           |
|----------------------|-------------------------------------------------------------------|
| Runtime verification | App startup, teardown, health, runtime verify commands, artifacts |
| Automation tests     | Automation repo, runner, environment, command template, artifacts |

The automation repository field supports both text entry and a folder browser. The Browse action selects a directory path; the repo should also be registered as a workspace before card-level spec browsing is available.

## Item Quality Panel

The item right panel has `Info`, `Jira`, and `Quality` tabs. The main item view has `Plan`, `Explorer`, and `Git` tabs.

Quality contains:

| Area                   | Behavior                                                         |
|------------------------|------------------------------------------------------------------|
| Runtime actions        | `Run smoke verify`, `Run critical verify`, `Re-run latest`       |
| Automation environment | Free text environment saved with item selection                  |
| Run mode               | Segmented toggle for `Silent` and `Visible browser`              |
| Selected specs         | Chips for saved specs, remove action, manual add path input      |
| Browse                 | Modal browser rooted at registered automation workspace          |
| Run automation tests   | Starts automation job and shows in-button progress while running |
| Artifacts              | Friendly cards for automation log, runtime setup log, reports    |

Manual add and Browse are attached to the spec path input. Run automation tests is a full-width automation action. The manual spec input uses normal panel colors in light and dark themes.

## Spec Browser

The spec browser uses the registered automation workspace tree API. It allows directory navigation and multi-select of files matching Cypress or Playwright spec naming patterns:

- `*.cy.ts`, `*.cy.js`, `*.cy.tsx`, `*.cy.jsx`
- `*.spec.ts`, `*.test.ts`, and JS/TSX/JSX variants

Selected paths are saved as item `verificationTests.selectedSpecs`.

## Run Progress

`Run automation tests` shows a compact animated progress bar inside the button while the automation launch request is in flight or the automation job is queued/running. In visible browser mode, the label reads `Starting browser...` to cover the Chrome/Chromium startup wait.

## Knowledge Flag Review

No Knowledge page UI change is required for moving `wiki_enriched` to `plan.yaml`. The frontend Knowledge page reads Knowledge API responses built from docs wiki pages, not feature-plan enrichment flags.
