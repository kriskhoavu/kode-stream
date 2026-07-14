# PM-032: Cloud And Agent Modes

PM-032 expands Kode Stream from a local-only app into two remote-ready operating models. Cloud mode runs Kode Stream on a VM or container and starts first. Agent mode keeps user-owned repositories, Git credentials, terminal tools, and AI CLIs on the user machine while the hosted web app acts as the UI and control plane.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                  |
|-------------------------------|--------------------------|----------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Local app baseline       | Established local workspace registration, scanning, board, and item workspace behavior.      |
| [PM-017](../PM-017/README.md) | Distribution baseline    | Defines local-first release channels, Homebrew priority, and no extension-only product path. |
| [PM-029](../PM-029/README.md) | Runtime and verification | Added runtime commands, automation verification, artifacts, and bounded verification jobs.   |
| [PM-031](../PM-031/README.md) | API transport baseline   | Confirms all `/api/*` routes are Gin-owned and ready for auth/capability middleware.         |

## Goal

Ship Cloud mode first with safe team access, Git URL workspace registration, server-managed clone storage, role-aware UI, and VM/container deployment. Design Agent mode in the same feature so Cloud mode does not block the later credential-safe path for user-owned local repositories.

## Non-Goals

- No public SaaS or multi-tenant billing model.
- No browser extension or Chrome plugin.
- No silent local agent installation from the web app.
- No raw local filesystem paths in Cloud mode.
- No requirement to implement Agent mode before Cloud mode is usable.

## Glossary

| Term            | Meaning                                                                                        | Code                            |
|-----------------|------------------------------------------------------------------------------------------------|---------------------------------|
| Runtime Mode    | Top-level operating mode selected at process startup.                                          | `local`, `cloud`, `agent`       |
| Cloud Mode      | Hosted Kode Stream server that authenticates users and clones Git URL workspaces on the VM.    | `KODE_STREAM_MODE=cloud`        |
| Agent Mode      | Local user process that connects outbound to Cloud and operates user-machine repositories.     | `kode-stream agent`             |
| Cloud Workspace | Workspace cloned and operated inside server-managed cloud storage.                             | `WorkspaceLocation=cloud_clone` |
| Agent Workspace | Workspace whose files and Git operations stay on the user's machine.                           | `WorkspaceLocation=agent_local` |
| Capability      | Runtime feature flag that tells API and UI whether an action is available for mode and role.   | app state capability map        |
| Cloud User      | Authenticated user from the configured identity provider.                                      | OIDC subject and profile        |
| Role            | Authorization level used by Cloud mode.                                                        | `admin`, `editor`, `viewer`     |
| Cloud Data Root | Persistent root for user state, indexes, audit logs, and server-managed clones.                | `KODE_STREAM_DATA_DIR`          |
| Agent Link      | Authenticated outbound connection from local agent to Cloud for commands and streamed results. | WebSocket control channel       |
| Deep Link       | Browser-opened URL that can wake an already installed agent.                                   | `kodestream://connect`          |

## Mode Summary

| Mode  | Runs Where                 | Workspace Source                    | Git Credentials                                 | Terminal And AI CLI                            |
|-------|----------------------------|-------------------------------------|-------------------------------------------------|------------------------------------------------|
| Local | User machine               | Existing local path or remote clone | Existing local Git setup                        | Existing local behavior                        |
| Cloud | VM or container            | Git URL clone only                  | Cloud credential provider or secret             | Admin-only or unavailable by capability        |
| Agent | User machine plus Cloud UI | Existing local path on user machine | Existing local Git setup, never stored by Cloud | Runs locally through agent after user connects |

## Data Flow

Cloud workspace registration: authenticated user -> Cloud UI -> `/api/workspaces` with Git URL -> Cloud auth and capability checks -> per-user path resolver -> remote clone -> scanner -> per-user registry and indexes -> board and item workspace.

Agent workspace registration: authenticated user -> Cloud UI -> connect local agent -> agent native folder picker -> agent validates local Git repo -> agent scans and indexes locally -> Cloud stores connection metadata and optional published summaries -> UI routes future workspace actions to the live agent.

## Design Decisions

| Decision                                               | Alternatives Considered                   | Rationale                                                                                 |
|--------------------------------------------------------|-------------------------------------------|-------------------------------------------------------------------------------------------|
| Implement Cloud mode first                             | Implement agent first                     | Gives deployable VM/container value sooner while designing the agent boundary correctly.  |
| Allow only Git URLs for Cloud workspace registration   | Allow server paths or imported workspaces | Cloud users should not register arbitrary VM filesystem paths.                            |
| Use per-user Cloud storage by default                  | One shared registry and clone root        | Reduces accidental cross-user visibility and keeps ownership clear.                       |
| Keep Agent mode credential-local                       | Upload SSH keys or tokens to Cloud        | User SSH keys, credential helpers, and local Git identity should remain on the user host. |
| Use guided native agent install, not browser extension | Chrome extension or terminal-only install | Extensions cannot replace the native agent, and terminal-only setup is poor for users.    |
| Gate risky Cloud commands by role and capability       | Expose all local features remotely        | Terminal, AI CLI, runtime, and verification can execute arbitrary commands.               |
| Keep Local mode trusted and compatible                 | Force auth into every mode                | Existing local-first users should not lose current workflows.                             |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Infrastructure Design](design/design-03-infrastructure.md)
- [Pipeline Design](design/design-04-pipeline.md)
- [Implementation Plan](implementation-plan.md)
