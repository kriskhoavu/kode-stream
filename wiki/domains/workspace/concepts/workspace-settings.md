---
slug: workspace-settings
title: Defaults And A Workspace's Own Settings
pageType: CONCEPT
topics: workspace, settings, harness, ai provider, settings file
summary: Settings holds the machine-wide defaults, and each workspace's page holds its own provider, harness switches, tests, models, role skills and sources, which a team shares through .kode-stream/settings.yaml.
sourceRef: |
  PM-044 | Workspace settings
lastVerified: 2026-11-20
---

## Two Layers
<!-- chunkId: workspace-settings-two-layers -->
<!-- keywords: defaults, workspace override, workspaces page, own models -->

[live] **Settings** holds what is the same for every workspace on this machine: appearance, storage, the AI tools' executables, terminals and default provider, the default harness switches, and the role pauses. A workspace's page under **Workspaces** holds its own provider, harness switches, models and role skills (AI and harness tab), and its sources beside its Jira connection (Integrations tab). Evidence: `web/src/pages/SettingsPage.tsx:24`, `web/src/features/workspaces/workspaceDetailTabs.ts:6`.

- A workspace's switches follow the defaults until it sets them. Each field names the default, as in `Default (on)`.
- A workspace with no provider starts with Claude while Claude is enabled, else with the default provider.
- Models belong to each workspace outright. On first start each workspace copies every role's engine, model, hard model and most turns from the defaults. An empty hard model keeps hard work on the normal model. Evidence: `internal/harness/domain/settings.go:181`, `internal/harness/teamfile.go:147`.
- The harness resolves a run's settings as the defaults with the workspace's choices on top. Evidence: `internal/harness/domain/settings.go:141`.

## What A Workspace Sets
<!-- chunkId: workspace-settings-what-a-workspace-sets -->
<!-- keywords: provider, test command, scope, role skills -->

| Area           | The workspace's page                                                   | Settings, for every workspace                                    |
|----------------|------------------------------------------------------------------------|------------------------------------------------------------------|
| Local AI tools | The provider a new session starts with                                 | Executables, terminals and their arguments, the default provider |
| Harness        | Run here, start on its own, Review before hand-off, stop and ask       | The same switches as defaults, role pauses, slots, intervals, retention |
| Models         | Each role's engine, model, hard model and most turns                   | Tool permissions and allowed tools                               |
| Work           | Test command, paths the Builder may change, role skills, sources       | —                                                                |

[live] A session started in a workspace, from the launch dialog or a run's Review, uses that workspace's provider, else Claude, else the default. A session outside any workspace uses the default. Evidence: `internal/ai/ai_settings_repository.go:50`.

[live] Role skills name the repository's own skill, and its track, that a role follows. If a repository still holds `.kode-stream/harness.yaml`, Kode Stream moves its role skills into the workspace's settings once, on start, and never reads the file again. Evidence: `internal/harness/teamfile.go:101`.

## The Settings File
<!-- chunkId: workspace-settings-file -->
<!-- keywords: export, import, team sharing, settings yaml -->

[live] A team shares settings through `.kode-stream/settings.yaml`. Kode Stream reads it only on **Import settings** and writes it only on **Export settings**, both in the workspace header. Export writes the chosen parts. Import shows each chosen part from the file beside the current value and lays it over the settings. Unnamed parts stay. Evidence: `web/src/features/settings/SettingsFileDialog.tsx:35`, `internal/server/api/settings_file_controller.go:77`.

The parts are the AI provider, harness switches, test command, Builder paths, models, role skills and sources. A personal value, such as a Jira search that names its owner, stays local unless someone exports it.

[live] The file holds only what a commit may safely set: no tool permission, executable, terminal argument or credential. An unknown key, role or engine, a scope outside the repository, or a token where an environment variable name belongs makes the whole file invalid. Nothing is imported from it, and export does not overwrite it. Evidence: `internal/workspace/settingsfile/settingsfile.go:112`.

## Not supported
<!-- chunkId: workspace-settings-not-supported -->
<!-- keywords: automatic import, credentials, executables -->

Reading the file automatically, sharing tool permissions or executables through it, and sharing one workspace's models with another.

## Known conflicts
<!-- chunkId: workspace-settings-known-conflicts -->
<!-- keywords: harness yaml, automatic sync, shared models -->

- "Kode Stream keeps `.kode-stream/settings.yaml` in sync." It does not. Only Export writes it and only Import reads it.
- "Changing one workspace's models changes the defaults." It does not. Each workspace owns its models.

## Related pages

See [[platform-harness]], [[workspace-ai-launch-reference]] and [[workspace-jira-workstream]].
