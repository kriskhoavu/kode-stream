---
slug: workspace-settings
title: Defaults And A Workspace's Own Settings
pageType: CONCEPT
topics: workspace, settings, harness, ai provider, settings file, export, import
summary: Every workspace starts from the app's defaults and may set its own provider, harness switches, tests, models, role skills and sources, which a team shares through .kode-stream/settings.yaml.
sourceRef: |
  PM-044 | Workspace settings
lastVerified: 2026-11-20
---

## Two Layers
<!-- chunkId: workspace-settings-two-layers -->
<!-- keywords: defaults, workspace override, settings for, picker -->

[live] Settings opens on the defaults every workspace starts from. A picker at its top, Settings for, switches to one workspace, whose page holds only what that workspace may set for itself. A blank value follows the defaults, and its field shows the default greyed or names it, as in `Default (on)`. Evidence: `web/src/pages/SettingsPage.tsx:54`.

[live] Kode Stream keeps both layers itself. The harness resolves a run's settings as the defaults with its workspace's choices laid over them: whether the harness runs there, whether a task starts on its own, whether a run waits at Review, and each role's engine, model, hard model and most turns. Evidence: `internal/harness/domain/settings.go:154`, `internal/harness/work.go:229`.

## What A Workspace Sets
<!-- chunkId: workspace-settings-what-a-workspace-sets -->
<!-- keywords: provider, test command, scope, models, role skills, sources, machine-wide -->

| Area | A workspace sets | Stays the defaults' alone |
|---|---|---|
| Local AI tools | the provider a new session starts with | executables, terminals and their arguments |
| Harness | run here, start on its own, Review before hand-off, stop and ask | role pauses, slots, intervals, retention |
| Models | each role's engine, model, hard model and most turns | tool permissions and allowed tools |
| Work | test command, paths the Builder may change, role skills, sources | — |

[live] A session started in a workspace, by the launch dialog or at a run's Review, starts with that workspace's provider, else the default. Evidence: `web/src/lib/types.ts:497`, `internal/ai/ai_settings_repository.go:51`.

[live] Role skills name the repository's own skill, and the track of it, that a role follows. They are a workspace setting; a repository's `.kode-stream/harness.yaml`, which held them before, is moved into its workspace's settings once, on start, and never read again. Evidence: `internal/harness/teamfile.go:109`, `internal/harness/build.go:242`.

## The Settings File
<!-- chunkId: workspace-settings-file -->
<!-- keywords: .kode-stream/settings.yaml, export, import, parts, team, commit -->

[live] A team shares a workspace's settings through `.kode-stream/settings.yaml` in its repository. Kode Stream never reads the file on its own. The workspace's Settings file panel exports chosen parts of its settings into the file, creating it, and imports chosen parts of the file over its settings, showing each part as the file holds it beside what the workspace has now. A part the import does not name stays as it was. Evidence: `web/src/features/settings/SettingsFilePanel.tsx:38`, `internal/server/api/settings_file_controller.go:81`, `internal/server/api/settings_file_controller.go:112`.

The parts are the AI provider, the harness switches, the test command, the paths the Builder may change, the models, the role skills and the sources. A personal value, such as a Jira search that names its owner, stays in Kode Stream unless someone exports it.

[live] The file may hold only what a commit may safely set: no tool permission, executable, terminal argument or credential. A key the format does not have, an unknown role or engine, a scope outside the repository, or a token where an environment setting's name belongs makes the whole file invalid; nothing is imported from it, and an export does not overwrite it. Evidence: `internal/workspace/settingsfile/settingsfile.go:127`, `internal/workspace/settingsfile/settingsfile.go:138`.

```yaml
version: 1
ai:
  provider: claude
harness:
  review: true
  tests:
    - go test ./...
  models:
    coder: { engine: claudecode, model: sonnet, hardModel: opus, maxTurns: 120 }
  procedure:
    planner: { skill: feature-planning, track: Plan }
  sources:
    - { kind: jira, enabled: true, query: "project = PM AND sprint in openSprints()" }
```

See [[platform-harness]] for what each harness setting does.
