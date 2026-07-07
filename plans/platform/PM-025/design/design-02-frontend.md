# Frontend Design: Jira-First Workspace

## Overview

Rename `KanbanPage` into the Workspace surface and make it the first screen for board views, item intake, Jira context, and AI launch. The Kanban-style board remains the primary view, but the route and navigation label become Workspace.

## Routes And Navigation

| Route        | Behavior                                                      |
|--------------|---------------------------------------------------------------|
| `/workspace` | Main Workspace surface with board view and new work item flow |
| `/`          | Redirects to `/workspace`                                     |
| `/kanban`    | Removed; no redirect or compatibility alias                   |
| `/items/:id` | Existing item detail route remains available after creation   |

## Data Model

| Type                   | Fields                                                     | Owner                |
|------------------------|------------------------------------------------------------|----------------------|
| `NewItemOrigin`        | `blank`, `jira`                                            | Intake modal         |
| `JiraIssueLookupState` | `state`, `issue`, `message`, `recoveryHint`, `refreshedAt` | API layer            |
| `NewItemDraft`         | source, identifier, title, status, owner, tags, Jira key   | Workspace page state |
| `AIPlanPreset`         | id, name, prompt, context mode, provider                   | AI launch UI         |

## State Management

| State                | Owner            | Behavior                                                         |
|----------------------|------------------|------------------------------------------------------------------|
| Workspace board data | Existing page    | Reuse current item loading, filtering, branches, and saved views |
| Intake draft         | New modal hook   | Preserves user edits while Jira lookup retries                   |
| Jira lookup          | New modal hook   | Cancels stale requests and blocks create until available         |
| AI preset selection  | AI launch dialog | Defaults to implementation plan preset after Jira-backed create  |
| Route naming         | App router       | Uses Workspace route names and removes Kanban references         |

## User Experience

- Rename the navigation item and page heading to Workspace.
- Keep the board controls dense and work-focused.
- Change the create button to `New Work Item`.
- Intake modal starts with Blank and From Jira choices.
- From Jira mode shows source, Jira key, fetch action, issue preview, and editable creation defaults.
- Create action is disabled while lookup is pending or failed.
- After creation, open the item detail and show an AI launch affordance with presets and free prompt.

## Components

| Component                 | Responsibility                                                   |
|---------------------------|------------------------------------------------------------------|
| `WorkspacePage`           | Renamed board surface and route owner                            |
| `NewWorkItemDialog`       | Origin choice, blank form, Jira form, preview, and create action |
| `JiraIssuePreview`        | Summary, status, assignee, labels, description, and attachments  |
| `AIPresetPicker`          | Preset list and free prompt entry for launch flows               |
| Existing board components | Columns, cards, filters, saved views, source settings            |

## Design Decisions

| Decision                         | Rationale                                                             |
|----------------------------------|-----------------------------------------------------------------------|
| Workspace is the page identity   | The surface now owns intake, planning, board views, Git, Jira, and AI |
| Board remains the default view   | Current users still need the Kanban/swimlane workflow                 |
| No `/kanban` compatibility route | The user explicitly does not require old links or saved route support |
| Intake stays modal first         | Creation is focused and does not require a new page for v1            |
| AI launch happens after creation | Existing AI controls expect an indexed item path                      |
