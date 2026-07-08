# Frontend Design: Jira-First Workstream

## Overview

The Workstream surface is the first screen for board views, item intake, Jira context, and AI launch. The board remains the primary view, while Workspace stays reserved for the sidebar section and registered workspace list.

## Routes And Navigation

| Route         | Behavior                                                       |
|---------------|----------------------------------------------------------------|
| `/workstream` | Main Workstream surface with board view and new work item flow |
| `/`           | Opens the Workstream route state                               |
| `/items/:id`  | Existing item detail route remains available after creation    |

## Data Model

| Type                   | Fields                                                     | Owner                 |
|------------------------|------------------------------------------------------------|-----------------------|
| `NewItemOrigin`        | `blank`, `jira`                                            | Intake modal          |
| `JiraIssueLookupState` | `state`, `issue`, `message`, `recoveryHint`, `refreshedAt` | API layer             |
| `NewItemDraft`         | source, identifier, title, status, owner, tags, Jira key   | Workstream page state |
| `AIPlanPreset`         | id, name, prompt, context mode, provider                   | AI launch UI          |

## State Management

| State                 | Owner            | Behavior                                                          |
|-----------------------|------------------|-------------------------------------------------------------------|
| Workstream board data | Workstream page  | Loads branch context, items, filtering, branches, and saved views |
| Intake draft          | New modal hook   | Preserves user edits while Jira lookup retries                    |
| Jira lookup           | New modal hook   | Cancels stale requests and blocks create until available          |
| AI preset selection   | AI launch dialog | Defaults to implementation plan preset after Jira-backed create   |
| Route naming          | App router       | Uses Workstream route names and a single canonical surface route  |

## User Experience

- Keep the board controls dense and work-focused.
- Change the create button to `New Work Item`.
- Intake modal starts with Blank and From Jira choices.
- From Jira mode shows source, Jira key, fetch action, issue preview, and editable creation defaults.
- Create action is disabled while lookup is pending or failed.
- After creation, open the item detail and show an AI launch affordance with presets and free prompt.

## Components

| Component                 | Responsibility                                                   |
|---------------------------|------------------------------------------------------------------|
| `WorkstreamPage`          | Surface and route owner                                          |
| `NewWorkItemDialog`       | Origin choice, blank form, Jira form, preview, and create action |
| `JiraIssuePreview`        | Summary, status, assignee, labels, description, and attachments  |
| `AIPresetPicker`          | Preset list and free prompt entry for launch flows               |
| Existing board components | Columns, cards, filters, saved views, source settings            |

## Design Decisions

| Decision                         | Rationale                                                         |
|----------------------------------|-------------------------------------------------------------------|
| Workstream is the page identity  | The surface owns intake, planning, board views, Git, Jira, and AI |
| Board remains the default view   | Users need fast status scanning and drag-friendly planning        |
| Single canonical route           | Navigation, saved state, and deep links stay predictable          |
| Intake stays modal first         | Creation is focused and does not require a new page for v1        |
| AI launch happens after creation | Existing AI controls expect an indexed item path                  |
