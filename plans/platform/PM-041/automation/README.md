# UI Automation: PM-041

## Applicability

Browser validation is required because this plan includes a frontend track, and because its central question —
whether the page title, search field and filter chips stay legible over each backdrop — cannot be answered by a
unit test.

## Runtime Inputs

| Input          | Source                                       |
|----------------|----------------------------------------------|
| Base URL       | Supplied when executing                      |
| Authentication | Local mode; no credentials                   |
| Test data      | Any workspace with at least one indexed plan |

## Playbooks

| Playbook            | Covers                                                                 | Status |
|---------------------|------------------------------------------------------------------------|--------|
| Variant switching   | Settings control writes the preference; each variant paints its band   | Passed |
| Backdrop off        | None drops the band and restores the topbar fill, blur and border      | Passed |
| Theme matrix        | Both variants in light and dark on Workstream, Workbench and Knowledge | Passed |
| Reduced motion      | Both variants freeze, sweep pinned at its start position               | Passed |
| Off-route unchanged | Settings and other non-band routes look identical under every variant  | Passed |
| Topbar popups       | Workspace and profile menus hit-test on top over a band                | Passed |

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Not requested. The feature changes a decorative layer and a local preference, and creates no new user journey.
Existing coverage of the Workstream, Workbench and Knowledge routes is sufficient.
