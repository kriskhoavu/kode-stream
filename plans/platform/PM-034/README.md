# PM-034: Chrome Extension And Agentless Cloud Showcase

PM-034 packages Kode Stream as a manually loaded Chrome extension showcase for Local mode and adds an agentless Cloud
workspace path for remote Git snapshots. The extension calls the local Kode Stream API on `127.0.0.1:4317`; the Cloud
path reads an approved Git-provider repository at a selected immutable commit. Local files, dirty state, Git commands,
terminal sessions, AI CLI, verification, and guarded writes remain an opt-in Cloud Agent responsibility.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                    |
|-------------------------------|--------------------------|------------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Local app baseline       | Established local Git workspace registration, scanning, board, and item workspace behavior.    |
| [PM-002](../PM-002/README.md) | File and Git baseline    | Added safe Markdown editing and guarded Git operations through the local backend boundary.     |
| [PM-033](../PM-033/README.md) | Local storage baseline   | Confirms app-owned state stays outside repository content and local mode defaults to data-dir. |
| [PM-032](../PM-032/README.md) | Cloud execution baseline | Preserves the existing Cloud Agent path for local machine execution behind a shared interface. |

## Goal

Provide a proof that Kode Stream can be experienced like a local Chrome application without moving privileged work into
Chrome:

- Build an unpacked Chrome MV3 extension artifact from the existing React app.
- Let extension-hosted UI call the local Kode Stream API through an API origin adapter.
- Showcase workspace browsing, Markdown read/edit/save, Git status, branch listing, and branch switching.
- Show a clear unavailable state when the local server is not running.
- Keep the normal localhost web build and embedded Go binary build unchanged.
- Let Cloud users register an approved provider repository without a Cloud Agent and browse a selected remote snapshot.
- Split Cloud workspace behavior by access adapter so agent-backed commands and agentless provider reads remain isolated.

## Non-Goals

- No Chrome Web Store release.
- No direct Chrome File System Access implementation.
- No direct Git execution inside the extension.
- No native messaging host.
- No embedded terminal or AI session WebSocket support in v1.
- No backend API response shape changes.
- No agentless local working-tree, dirty-state, file-write, Git mutation, or process-execution support.
- No provider-side Git writes in the first agentless Cloud release.

## Glossary

| Term                     | Meaning                                                                  | Code                              |
|--------------------------|--------------------------------------------------------------------------|-----------------------------------|
| Extension Surface        | React UI running from a Chrome extension page.                           | `chrome_extension` build surface  |
| Local API Origin         | Loopback Kode Stream server used by the extension.                       | `http://127.0.0.1:4317`           |
| API Origin Adapter       | Frontend helper that resolves relative API paths per surface.            | API URL resolver                  |
| Unpacked Extension       | Developer-loaded Chrome extension directory.                             | `dist/chrome-extension`           |
| Privileged Operation     | Filesystem, Git, terminal, AI CLI, or verification action.               | Go local server route             |
| Showcase Scope           | Files and Git behavior proven by manual extension testing.               | Workspaces, files, Git branches   |
| Workspace Access Mode    | How Cloud obtains workspace data and determines supported actions.       | `agent_backed`, `remote_snapshot` |
| Remote Snapshot          | Provider branch, tag, or commit resolved to an immutable commit SHA.     | provider integration              |
| Workspace Access Adapter | Interface implementation for one Cloud workspace access mode.            | agent or provider adapter         |
| Terminal Handoff         | User-controlled local terminal instruction; not Cloud command execution. | Cloud action guidance             |

## Components

| Layer         | Component                 | Purpose                                                                                |
|---------------|---------------------------|----------------------------------------------------------------------------------------|
| Frontend      | API origin adapter        | Converts `/api/*` calls to the local API origin when running as an extension.          |
| Frontend      | Extension mode guard      | Detects the extension surface and hides unsupported terminal/AI streaming UI.          |
| Build         | Extension Vite build      | Emits extension-safe assets with relative paths and a generated manifest.              |
| Pipeline      | Showcase verification     | Runs type/test checks and documents manual unpacked-extension acceptance steps.        |
| Cloud backend | Workspace access resolver | Selects the agent-backed or remote-snapshot adapter without route-level mode branches. |
| Cloud backend | Remote snapshot adapter   | Reads provider refs, trees, files, and commit metadata using read-only authorization.  |

## Data Flow

Browser extension page -> React app -> API origin adapter -> Kode Stream local API -> registered workspace -> files and
Git history.

Cloud remote workspace -> workspace access resolver -> remote snapshot adapter -> Git provider API -> commit-pinned
tree, file, board, search, and capability response.

## Design Decisions

| Decision                                        | Alternatives Considered                | Rationale                                                                    |
|-------------------------------------------------|----------------------------------------|------------------------------------------------------------------------------|
| Keep the Go local server as the trusted backend | Direct extension filesystem and JS Git | Preserves existing guards, Git behavior, local storage, and command model.   |
| Bundle the SPA instead of only launching a tab  | Companion wrapper that opens localhost | Proves the Chrome application experience while still using local APIs.       |
| Limit v1 to Files + Git                         | Full terminal/AI streaming             | WebSocket origin handling needs a separate security design.                  |
| Support unpacked local import first             | Chrome Web Store packaging             | Faster showcase validation without store review or signing constraints.      |
| Configure local API origin through localStorage | Hard-code one port only                | Allows testing non-default ports while keeping v1 simple.                    |
| Use workspace access adapters in Cloud          | Route-level `if agent` branches        | Preserves PM-032 behavior while adding a separately testable agentless path. |
| Start agentless Cloud as read-only snapshots    | Provider-side mutations                | Remote provider writes would not update the user's local checkout.           |
| Use terminal handoff for agentless workspaces   | Cloud-triggered local terminal         | Browsers cannot safely execute or observe arbitrary local processes.         |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Frontend Design](design/design-01-frontend.md)
- [Infrastructure Design](design/design-02-infrastructure.md)
- [Pipeline Design](design/design-03-pipeline.md)
- [Agentless Cloud Design](design/design-04-agentless-cloud.md)
- [Implementation Plan](implementation-plan.md)
- [Showcase Runbook](../../../docs/release/chrome-extension-showcase.md)
