# PM-001: Plan Manager Read-Only MVP

## Overview

Plan Manager is a local Git-native web app for browsing planning documents.

The MVP lets a developer register local repositories, scan plan folders, view plans on a Kanban board, and open a plan workspace with a file tree, Markdown preview, raw Markdown view, metadata, and read-only Git diff.

The MVP is read-only for managed repositories. It does not edit plan files. It does not run Git write operations.

## Source Material

| Source                                       | Role                 | How It Guides This Plan                                                                                  |
|----------------------------------------------|----------------------|----------------------------------------------------------------------------------------------------------|
| [Requirement](../../../specs/requirement.md) | Product requirements | Defines repository management, plan discovery, Kanban, workspace, Git operations, and distribution goals |
| [Design Image](../../../specs/design.png)    | UI reference         | Defines the desktop shell, board layout, plan workspace, mobile board, and light/dark visual direction   |

## Glossary

| Term            | Meaning                                                   | Maps To (code)              |
|-----------------|-----------------------------------------------------------|-----------------------------|
| Repository      | A local Git repository registered in Plan Manager         | `RepositoryConfig`          |
| Plan Directory  | A configured folder that contains plan documents          | `planDirectories`           |
| Plan            | A ticket-level planning folder such as `plans/api/DI-170` | `PlanSummary`, `PlanDetail` |
| Plan Metadata   | Optional machine-readable metadata for a plan             | `plan.yaml`                 |
| Document        | A Markdown file that belongs to a plan                    | `PlanDocument`              |
| Scan            | Read-only indexing of configured plan directories         | `RepositoryScanner`         |
| Board Status    | The Kanban column for a plan                              | `PlanStatus`                |
| Workspace       | The details view for one plan                             | `PlanWorkspace`             |
| Visual Baseline | The required UI reference for v1                          | `specs/design.png`          |

## Components

| Layer    | Component           | Purpose                                                                         |
|----------|---------------------|---------------------------------------------------------------------------------|
| Backend  | Repository registry | Stores registered repositories in the user data directory                       |
| Backend  | Plan scanner        | Reads Git state, plan folders, `plan.yaml`, and Markdown files                  |
| Backend  | Plan index          | Caches searchable plan summaries and document metadata                          |
| Backend  | HTTP API            | Serves repository, plan, file, and diff data to the frontend                    |
| Frontend | App shell           | Matches the dark top bar, left nav, repository tabs, and search from the design |
| Frontend | Kanban board        | Shows plans by status with filters and compact cards                            |
| Frontend | Plan workspace      | Shows file tree, raw Markdown, preview, metadata, and read-only diff            |
| DevOps   | Build packaging     | Builds one local app binary with embedded frontend assets                       |
| DevOps   | AI verification     | Runs Playwright MCP checks during implementation                                |

## Data Flow

```text
Developer starts Plan Manager
  -> backend loads app config from user data directory
  -> frontend asks for repositories
  -> developer registers this repo and plan directories
  -> backend validates Git repo, branch, and folders
  -> developer triggers Scan
  -> scanner reads local branches and working tree
  -> scanner indexes plan.yaml first
  -> scanner falls back to folder and README parsing when plan.yaml is missing
  -> frontend renders board columns and cards
  -> developer opens a card
  -> frontend loads file tree, file content, metadata, and diff
```

## Design Decisions

| Decision                                    | Alternatives Considered                     | Rationale                                                                                          |
|---------------------------------------------|---------------------------------------------|----------------------------------------------------------------------------------------------------|
| Use Go plus React/Vite                      | Node-only, Rust plus React                  | Go gives a simple local binary and strong filesystem/Git access. React/Vite fits the proposed UI.  |
| Store app data outside managed repos        | Store config in each repo, config file only | The app should not dirty target repositories. A cache is needed for large plan sets.               |
| Make v1 read-only                           | Editable workspace, full Git manager        | Read-only browsing gives value first and avoids save, lock, credential, and branch mutation risks. |
| Use `plan.yaml` first                       | README-only parsing                         | Existing plans already use `plan.yaml`. It gives stable metadata and document order.               |
| Add fallback parsing                        | Require `plan.yaml`                         | Older plans and custom folders should still appear.                                                |
| Do not auto fetch in v1                     | Fetch every 15 seconds                      | Fetch changes `.git` refs and can trigger credentials. Manual scan is safer for v1.                |
| Treat `specs/design.png` as visual baseline | Treat image as inspiration only             | The UI must not drift away from the documented proposal.                                           |
| Use Playwright MCP as a phase gate          | Manual browser checks only                  | AI-agent-run browser checks make layout and workflow regressions visible during development.       |

## Implementation Clarifications

- PM-001 should support at least 100 repositories, 10,000 plans, and 100,000 files through cached plan summaries.
- Board and list views must read from cached metadata. They must not load every Markdown file on each render.
- File content should load only when the user opens a plan file.
- Backend code should keep clear boundaries between repository registry, Git access, scanning, indexing, and HTTP handlers.
- HTTP handlers must not read arbitrary filesystem paths directly. They must go through the plan index and file access layer.
- Manual Scan rebuilds derived metadata for one repository.
- A bad plan creates a scan warning. It must not fail the whole repository scan.
- The app must not write to registered repositories in PM-001.
- File reads must stay inside configured plan directories.
- The UI should match the layout, density, navigation, and mobile behavior of `specs/design.png`. It does not need pixel-perfect parity.

## Next Plan

After PM-001 is complete, create `PM-002: Plan Editing And Git Operations`.

PM-002 should turn the read-only workspace into a safe authoring workflow. It should cover Markdown editing, status moves, new plan creation, commit, pull, push, branch create, branch switch, dirty-state handling, and write-operation safeguards.

PM-002 should reuse the PM-001 terminology and APIs where possible. It should add write APIs only after the read-only scan, board, workspace, and Playwright MCP acceptance flow are stable.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Infrastructure Design](design/design-03-infrastructure.md)
- [Pipeline Design](design/design-04-pipeline.md)
- [Implementation Plan](implementation-plan.md)
