# PM-032: Cloud Mode With Local Agent Execution

PM-032 defines two runtime modes for Kode Stream: Local and Cloud. Local mode runs the full app on the user's machine.
Cloud mode runs the hosted web app and API on operator infrastructure, while repository files, Git credentials, terminal
commands, AI CLI sessions, and verification commands run through a local agent on the user's machine.

Cloud mode is a hosted UI, collaboration, and control-plane layer behind OAuth2Proxy. OAuth2Proxy is the public
authentication boundary and can redirect users to Keycloak. Kode Stream stays on a private VM/container port and trusts
the identity headers forwarded by OAuth2Proxy. Cloud never clones repositories onto the hosted VM and never executes
workspace commands on the hosted VM.

Cloud mode does not require a database service for the first release. The hosted app can use file-backed metadata under
`KODE_STREAM_DATA_DIR` for users, workspaces, agent state, audit logs, and published summaries. A database becomes useful
when Cloud needs multiple API replicas, stronger concurrent writes, reporting queries, or larger team scale.

## Related Plans

| Item                          | Relationship             | Key Context                                                                                |
|-------------------------------|--------------------------|--------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Local app baseline       | Established workspace registration, scanning, board, and item workspace behavior.          |
| [PM-017](../PM-017/README.md) | Distribution baseline    | Defines release channels, Homebrew priority, and no extension-only product path.           |
| [PM-029](../PM-029/README.md) | Runtime and verification | Added runtime commands, automation verification, artifacts, and bounded verification jobs. |
| [PM-031](../PM-031/README.md) | API transport baseline   | Confirms all `/api/*` routes are Gin-owned and ready for auth/capability middleware.       |

## Goal

Ship Cloud mode as a hosted control plane that connects authenticated users to their own local agent. The hosted app
provides login, team access, workspace metadata, board views, role-aware controls, connection status, and command
routing. The local agent provides repository access, Git operations, terminal, AI CLI, scanning, and verification from
the user's machine.

## Non-Goals

- No Cloud-hosted repository clone.
- No Cloud-hosted workspace terminal.
- No Cloud-hosted AI CLI or verification command execution.
- No browser extension or Chrome plugin.
- No silent local agent installation from the web app.
- No upload of SSH keys, Git tokens, credential helper output, or arbitrary local files to Cloud.

## Glossary

| Term                | Meaning                                                                                     | Code                            |
|---------------------|---------------------------------------------------------------------------------------------|---------------------------------|
| Runtime Mode        | Top-level server behavior selected at process startup.                                      | `local`, `cloud`                |
| Local Mode          | Trusted single-user app running on the user's machine.                                      | `KODE_STREAM_MODE=local`        |
| Cloud Mode          | Hosted app and API that authenticate users and route workspace operations to local agents.  | `KODE_STREAM_MODE=cloud`        |
| Cloud Agent         | User-machine process that connects outbound to Cloud and executes local workspace commands. | `kode-stream agent`             |
| Cloud Workspace     | Workspace registered in Cloud and backed by a user-machine repository through Cloud Agent.  | `WorkspaceLocation=cloud_agent` |
| Agent Link          | Authenticated outbound channel from Cloud Agent to Cloud.                                   | WebSocket control channel       |
| OAuth2Proxy         | Public auth gateway that redirects to Keycloak and forwards trusted identity headers.       | reverse proxy auth boundary     |
| Cloud User          | Authenticated identity forwarded from OAuth2Proxy.                                          | subject, email, profile headers |
| Role                | Authorization level used by Cloud mode.                                                     | `admin`, `editor`, `viewer`     |
| Capability          | Runtime feature flag that tells API and UI whether an action is available.                  | app state capability map        |
| AI Session Terminal | Interactive AI/terminal surface executed by the user's Cloud Agent.                         | terminal capability             |
| Deep Link           | Browser-opened URL that can wake an installed Cloud Agent.                                  | `kodestream://connect`          |

## Agent Connectivity

Cloud Agent establishes an outbound connection to the Cloud public URL. The user machine does not open an inbound port,
and Cloud does not connect directly to the user machine.

| Connectivity Option      | Role                                                           |
|--------------------------|----------------------------------------------------------------|
| Outbound HTTPS WebSocket | Default product path for Cloud Agent control and streams.      |
| Tailscale or VPN         | Optional operator network layer for private Cloud deployments. |
| Port forwarding          | Development-only troubleshooting tool, not a product path.     |

The default endpoint is `wss://<cloud-public-url>/api/agents/channel`. The agent authenticates with a short-lived
connect token for first pairing, then uses a rotated agent credential bound to the Cloud user and device.

## Component Flow

[Mermaid component diagram](design/cloud-agent-component.mmd)

Browser Cloud UI -> OAuth2Proxy -> Keycloak login when needed -> Kode Stream Cloud API on a private port -> Agent
channel manager -> outbound WebSocket -> Cloud Agent on user machine -> local repository, Git credentials, terminal, AI
CLI, and verification tools.

| Component             | Runs On      | Responsibility                                                                   |
|-----------------------|--------------|----------------------------------------------------------------------------------|
| Browser Cloud UI      | User browser | Shows hosted app, starts agent pairing, renders streamed output.                 |
| OAuth2Proxy           | Cloud/VPS    | Public endpoint, TLS/auth gateway, Keycloak redirect, identity headers.          |
| Kode Stream Cloud API | Cloud/VPS    | Trusts proxy identity headers, enforces roles, stores metadata, routes commands. |
| Agent channel manager | Cloud/VPS    | Owns agent sessions, heartbeats, command envelopes, stream routing.              |
| Cloud Agent           | User machine | Executes workspace file, Git, terminal, AI CLI, and verification actions.        |
| Local repository      | User machine | Provides source files and workspace root.                                        |
| Local Git credentials | User machine | Stay inside SSH agent, Git credential helper, or local config.                   |
| Terminal and AI tools | User machine | Run command-capable sessions for the owner workspace.                            |

Command path: Browser Cloud UI -> Kode Stream Cloud API -> Agent channel manager -> Cloud Agent -> local
repository/tooling -> Cloud Agent -> Agent channel manager -> Browser Cloud UI.

## Mode Summary

| Mode  | Runs Where                  | Workspace Source                        | Git Credentials        | Terminal And AI CLI              |
|-------|-----------------------------|-----------------------------------------|------------------------|----------------------------------|
| Local | User machine                | User-selected local path or local clone | User machine Git setup | User machine                     |
| Cloud | Hosted app plus Cloud Agent | User-selected local path through agent  | User machine Git setup | User machine through Cloud Agent |

## Data Flow

Cloud workspace registration: authenticated user -> Cloud UI -> connect Cloud Agent -> agent native folder picker ->
user selects local repository -> agent validates Git root and source paths -> agent scans locally -> Cloud stores
workspace metadata, status, and optional summaries -> UI routes file, Git, terminal, AI, runtime, and verification
actions to the live Cloud Agent.

## Workspace Execution Rule

Workspace command execution always follows the workspace owner agent in Cloud mode.

| Action                        | Cloud Host | Cloud Agent |
|-------------------------------|------------|-------------|
| Render web UI and API         | yes        | no          |
| Authenticate user session     | yes        | no          |
| Store workspace metadata      | yes        | no          |
| Read repository files         | no         | yes         |
| Run Git commands              | no         | yes         |
| Run terminal or AI CLI        | no         | yes         |
| Run verification commands     | no         | yes         |
| Access SSH agent or Git token | no         | yes         |

Cloud never creates a server-side checkout for a workspace. A workspace is usable for command-capable features only when
its owner Cloud Agent is connected.

## Design Decisions

| Decision                                      | Rationale                                                                      |
|-----------------------------------------------|--------------------------------------------------------------------------------|
| Keep only Local and Cloud runtime modes       | Agent is an execution component of Cloud, not a separate product mode.         |
| Put OAuth2Proxy in front of Cloud             | Login, Keycloak redirect, and browser auth cookies stay at the proxy boundary. |
| Make Cloud a hosted control plane             | Hosted infrastructure should manage UI, status, metadata, policy, and routing. |
| Use file-backed metadata for Cloud v1         | Avoids running a database before Cloud needs scale or query complexity.        |
| Execute workspace operations on user machines | Repository files, Git credentials, local tools, and terminals stay local.      |
| Use outbound WebSocket for agent connection   | Works behind NAT/firewalls and avoids exposing the user machine.               |
| Require explicit Cloud Agent connection       | Cloud cannot silently access user workspaces or host credentials.              |
| Avoid browser extension infrastructure        | Native file, Git, terminal, and AI CLI access require a native process.        |
| Store metadata, not repository contents       | Cloud should not own workspace source trees or command execution state.        |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Infrastructure Design](design/design-03-infrastructure.md)
- [Pipeline Design](design/design-04-pipeline.md)
- [Implementation Plan](implementation-plan.md)
