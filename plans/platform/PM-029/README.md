# PM-029: External Automation Verification Runner

PM-029 makes card-level automation testing a first-class Quality workflow. A workspace can register an external Cypress or Playwright repository, link specs to an item, run those specs after the app runtime is healthy, and inspect runtime plus automation logs from the same verification job.

The implementation keeps workspace Smoke and Critical verification separate from feature automation coverage. Smoke and Critical remain runtime health checks. Automation tests are explicit card-linked checks.

## Related Plans

| Item                          | Relationship             | Key Context                                                                            |
|-------------------------------|--------------------------|----------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Verification baseline    | Established local verification and browser acceptance as part of the platform workflow |
| [PM-015](../PM-015/README.md) | Architecture conventions | Calls for clearer backend ownership and frontend feature-controller boundaries         |
| [PM-028](../PM-028/README.md) | Current item surface     | Keeps item detail and embedded Explorer as the main place for item-specific actions    |

## Glossary

| Term                         | Meaning                                                                         | Code / Surface                         |
|------------------------------|---------------------------------------------------------------------------------|----------------------------------------|
| Runtime Verification Profile | Existing workspace-level `smoke`, `critical`, or `full` command                 | `VerifyProfile`                        |
| Automation Repository        | Separate registered workspace that owns Cypress or Playwright tests             | Workspace automation settings          |
| Selected Spec                | Spec path explicitly saved for one item                                         | `verificationTests.selectedSpecs`      |
| Planned Automation Path      | Spec path authored by feature or test planning skills before it is selected     | `automation-test-paths[].path`         |
| Discovered Spec              | Suggested spec read from automation repo plan metadata                          | Item verification-test API             |
| Run Mode                     | Silent/headless or visible headed browser execution                             | `displayMode`                          |
| Runtime Setup Log            | `prepare`, `up`, `health`, and `down` output for the app runtime around the job | `runtime.log` / Runtime setup log card |
| Automation Log               | Cypress or Playwright command output                                            | `automation.log` / Automation log card |

## Current Behavior

| Action               | Scope        | Runs                                                               |
|----------------------|--------------|--------------------------------------------------------------------|
| Run smoke verify     | Workspace    | Existing `runtime.commands.verify.smoke` command                   |
| Run critical verify  | Workspace    | Existing `runtime.commands.verify.critical`, falling back to smoke |
| Run automation tests | Current item | Saved Cypress or Playwright specs from the automation repository   |

`Run automation tests` executes after runtime `prepare`, `up`, and `health`. For Docker Compose runtimes, `runtime.log` contains the configured compose setup and teardown command output for that automation job.

## Metadata Model

| Location            | Field                     | Purpose                                                                 |
|---------------------|---------------------------|-------------------------------------------------------------------------|
| Item `plan.yaml`    | `verificationTests`       | Runtime state: selected specs, environment, display mode, updated time  |
| Feature `plan.yaml` | `automation-test-paths`   | Planning metadata: known automation spec paths authored by skills/users |
| Feature `plan.yaml` | `plan.wiki_enriched`      | Wiki enrichment state for plan ingestion                                |
| Feature `README.md` | None for machine metadata | Human-readable plan only                                                |

`verificationTests` and `automation-test-paths` are intentionally separate. `verificationTests` is what the card will run. `automation-test-paths` is a source for suggestions and can contain empty placeholders until a tester provides real paths.

## Automation Spec Discovery

Discovery uses structured metadata only:

1. Read likely `plan.yaml` files in the automation repository:
   - `plans/{ticket}/plan.yaml`
   - `plans/{scope}/{ticket}/plan.yaml`
   - `plans/{itemID}/plan.yaml`
2. Extract non-empty `automation-test-paths[].path` entries.

This keeps `/api/items/{id}/verification-tests` responsive and avoids scanning test-plan Markdown files whenever the Quality panel opens.

## Quality UI

The item side panel has `Info`, `Jira`, and `Quality` tabs. Quality contains:

- Runtime actions: `Run smoke verify`, `Run critical verify`, and `Re-run latest`.
- Automation controls: environment, run mode, selected specs, manual add, repo-backed browse, and `Run automation tests`.
- Artifact cards with friendly labels such as `Automation log`, `Runtime setup log`, and `Test report`.

Automation run mode supports:

| Mode            | Behavior                                                                    |
|-----------------|-----------------------------------------------------------------------------|
| Silent          | Current headless/background command behavior                                |
| Visible browser | Cypress appends headed Chrome args; Playwright appends headed Chromium args |

## Knowledge Flag Review

Moving `wiki_enriched` from README frontmatter to `plan.yaml` should not break the Knowledge page UI. The Knowledge page indexes docs pages from `docs/**` using wiki page frontmatter such as `slug`, `title`, `domain`, and relationships. It does not read feature-plan `wiki_enriched`.

The required change is in the wiki enrichment workflow: the skill must update `plan.wiki_enriched` in `plan.yaml` instead of adding or editing README frontmatter.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
