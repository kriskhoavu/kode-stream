# Infrastructure Design: Cloud Mode With Local Agent Execution

## Overview

Cloud mode needs hosted app infrastructure with persistent metadata storage, OAuth2Proxy in front of the app, Keycloak
OIDC configuration at the proxy, health checks, and outbound agent channels. Workspace source trees and command
execution stay on user machines through Cloud Agent, a native local process backed by the `internal/agent` package.

## Cloud Runtime

| Concern      | Design                                                                                         |
|--------------|------------------------------------------------------------------------------------------------|
| Process      | Same Go binary serves embedded frontend and Gin API; the agent is a separate local subcommand. |
| Bind address | Cloud mode supports `KODE_STREAM_BIND_ADDR`, normally `0.0.0.0`.                               |
| App exposure | Kode Stream app port stays private to the VM/container network.                                |
| Public auth  | OAuth2Proxy is the internet-facing service and redirects users to Keycloak.                    |
| TLS          | Terminated before or at OAuth2Proxy, depending on operator topology.                           |
| Persistence  | Metadata and audit data are mounted volumes.                                                   |
| Identity     | Kode Stream trusts OAuth2Proxy identity headers in Cloud mode.                                 |
| Agent link   | Agents connect outbound over authenticated HTTPS WebSocket.                                    |
| Healthcheck  | Container and proxy check `GET /api/health`; proxy may skip auth for it.                       |
| Backups      | Operator backs up mounted metadata volumes.                                                    |
| Shell access | Web users never receive an interactive shell on the Cloud host.                                |

## Container Layout

| Layer      | Contents                                                 |
|------------|----------------------------------------------------------|
| Builder    | Node.js, npm install, frontend build, Go build.          |
| Runtime    | Minimal OS image and CA certificates.                    |
| User       | Non-root user owns metadata mount paths.                 |
| Entrypoint | Starts `kode-stream serve` with Cloud env configuration. |
| Volumes    | `/var/lib/kode-stream/data`.                             |

The image should not bake OAuth2Proxy or Keycloak secrets, Git credentials, repository contents, or user data.

## OAuth2Proxy And Keycloak Boundary

| Layer           | Exposure       | Responsibility                                                      |
|-----------------|----------------|---------------------------------------------------------------------|
| OAuth2Proxy     | Public HTTPS   | Login gate, Keycloak redirect, browser auth cookie, header forward. |
| Kode Stream app | Private port   | Reads trusted identity headers, enforces roles, stores metadata.    |
| Keycloak        | Public/private | Identity provider for OAuth2Proxy.                                  |

Kode Stream accepts proxy identity headers such as `X-Auth-Request-User`, `X-Auth-Request-Email`,
`X-Forwarded-User`, and `X-Forwarded-Email`. JWT or opaque-token introspection inside Kode Stream is optional for later
hardening and is not part of the PM-032 release requirement.

## Local Docker Auth Stack

The local Cloud auth stack copies the Helm deployment idea into Docker Compose:

| Service     | Local URL                           | Responsibility                                            |
|-------------|-------------------------------------|-----------------------------------------------------------|
| Keycloak    | `http://keycloak.localhost:8081`    | Imports the `kode-stream` realm and local users.          |
| OAuth2Proxy | `http://kode-stream.localhost:4318` | Browser entry point, Keycloak redirect, identity headers. |
| Kode Stream | private Docker network port `4317`  | Cloud API and embedded frontend, not directly published.  |

The local stack uses `docker/cloud/local-compose.yaml` and `docker/cloud/keycloak/kode-stream-realm.json`. It is a
developer smoke environment, not the production VM compose file.

## Cloud Storage

Cloud v1 requires a persistent metadata volume, not a managed database. Operators back up the metadata volume. A database
is an optional scaling change when the deployment needs multiple Cloud API replicas, richer reporting, or higher write
concurrency.

| Storage Area | Default Container Path      | Notes                                           |
|--------------|-----------------------------|-------------------------------------------------|
| Data root    | `/var/lib/kode-stream/data` | Users, workspace metadata, agent state, audit.  |
| Cache        | data-root scoped directory  | Optional summaries safe for offline UI display. |
| Temp         | runtime temp directory      | Should not hold durable credentials or repos.   |

Use explicit volume mounts in compose and VM docs.

## Cloud Agent Installation

| OS      | Support Level            | Install Path                                                     |
|---------|--------------------------|------------------------------------------------------------------|
| macOS   | Production first target  | Homebrew formula for `kode-stream-agent` or `kode-stream agent`. |
| Windows | Planned supported target | MSI or executable installer with startup task/service.           |
| Linux   | Planned supported target | `.deb`, `.rpm`, or tarball with systemd user service.            |

The Kode Stream binary hosts the `kode-stream agent` subcommand first. A separate binary remains optional only if
packaging or service-management constraints require it later.

## Cloud Agent Runtime

| Concern      | Design                                                                                                                            |
|--------------|-----------------------------------------------------------------------------------------------------------------------------------|
| Startup      | `kode-stream agent start --connect <deep-link-or-token> --cloud-url <url> --repo <path>` first, then optional background service. |
| Doctor       | `kode-stream agent doctor` checks Git, SSH agent, Cloud reachability, and deep-link registration.                                 |
| Deep link    | `kodestream://connect` wakes an installed agent.                                                                                  |
| Connection   | Agent opens outbound HTTPS WebSocket to Cloud with short-lived connect token.                                                     |
| Local access | Agent validates repo paths and reuses existing local services for Git, file, terminal, AI CLI, runtime, and verification work.    |
| Secrets      | Agent does not send Git credentials, SSH keys, or credential-helper output to Cloud.                                              |

## Agent Network Topology

Cloud Agent uses an outbound WebSocket connection to the Cloud public URL. This keeps the user machine behind
NAT/firewalls and avoids inbound firewall rules, public user-machine ports, and SSH-style access from Cloud to the user
host.

| Topology              | Use                                             |
|-----------------------|-------------------------------------------------|
| Public HTTPS endpoint | Default VPS and public Cloud deployment path.   |
| Tailscale or VPN      | Optional private deployment path for operators. |
| Port forwarding       | Local development and debugging only.           |

Reverse proxies must support WebSocket upgrade on `/api/agents/channel`, preserve authentication headers, and use idle
timeouts suitable for long-running terminal and AI CLI streams. The current foreground agent proves connect, heartbeat,
and metadata publish; durable credentials, reconnect backoff, and background service packaging are follow-on agent
phases.

## Component Diagram

[Mermaid component diagram](cloud-agent-component.mmd)

Browser Cloud UI -> Cloud reverse proxy -> Kode Stream Cloud API -> Agent channel manager -> outbound WebSocket -> Cloud
Agent -> local repository and local tools.

| Component             | Boundary     | Inputs                                           | Outputs                                        |
|-----------------------|--------------|--------------------------------------------------|------------------------------------------------|
| Browser Cloud UI      | User browser | OIDC session, user actions, rendered streams     | Connect requests, workspace commands           |
| OAuth2Proxy           | Cloud/VPS    | HTTPS, Keycloak login, WebSocket traffic         | Authenticated Cloud API and agent traffic      |
| Kode Stream Cloud API | Cloud/VPS    | Trusted proxy identity headers                   | Metadata responses, command envelopes          |
| Agent channel manager | Cloud/VPS    | Agent WebSocket, command envelopes, heartbeats   | Stream frames, command status, stale state     |
| Cloud Agent           | User machine | Command envelopes, local policy, local workspace | File/Git/terminal/AI results, scan summaries   |
| Local repository      | User machine | Agent file and Git operations                    | Source content, Git status, scan data          |
| Local tools           | User machine | Agent-launched commands                          | Terminal, AI CLI, runtime, verification output |

Trust boundary: OAuth2Proxy owns browser authentication and forwards identity to Kode Stream over a private network.
Cloud/VPS stores identity-derived metadata, audit, and routing state. User machine stores repository contents, Git
credentials, local tools, and command execution.

## Hosted Execution Boundary

Cloud infrastructure hosts UI, API, identity, metadata, audit, and routing. It does not host workspace source trees and
does not run workspace commands.

| Boundary      | Requirement                                                                    |
|---------------|--------------------------------------------------------------------------------|
| Repository    | No Cloud-hosted checkout or source storage.                                    |
| Terminal      | No terminal session attaches to the Cloud VM or container.                     |
| Git           | Git commands run through the user-machine agent.                               |
| AI CLI        | AI CLI commands run through the user-machine agent.                            |
| Verification  | Verification commands run through the user-machine agent.                      |
| Secrets       | OIDC/session secrets stay in Cloud; Git and local tool credentials stay local. |
| Offline state | Cloud shows cached metadata and disables command-capable controls.             |
| Audit         | Cloud logs actor, workspace, agent, command type, and result status.           |

## VM Deployment

| Step | Operator Action                                                            |
|------|----------------------------------------------------------------------------|
| 1    | Provision VM and DNS.                                                      |
| 2    | Install Docker or run native binary as a service.                          |
| 3    | Configure OAuth2Proxy as the public endpoint and keep Kode Stream private. |
| 4    | Configure Keycloak OIDC client and OAuth2Proxy secrets.                    |
| 5    | Configure Kode Stream Cloud env and admin bootstrap users.                 |
| 6    | Mount metadata data volume.                                                |
| 7    | Start Cloud mode and verify health through OAuth2Proxy.                    |
| 8    | Register admin users and test Cloud Agent connection.                      |

## Security Controls

| Control                         | Purpose                                                                |
|---------------------------------|------------------------------------------------------------------------|
| Non-root container user         | Limits blast radius of process compromise.                             |
| Metadata-only Cloud storage     | Prevents hosted source tree and hosted credential exposure.            |
| File-backed metadata volume     | Keeps Cloud deployable without a database service.                     |
| OAuth2Proxy auth boundary       | Keeps login, Keycloak redirect, and browser auth cookies at the proxy. |
| Session cookie signing          | Protects fallback app-owned Cloud sessions.                            |
| Proxy CSRF/session controls     | Protect browser-authenticated Cloud writes in OAuth2Proxy mode.        |
| Capability-gated command routes | Prevents unauthorized command routing to agents.                       |
| Agent-scoped command envelopes  | Keeps workspace commands bound to owner, workspace, and agent.         |
| Outbound-only agent connection  | Avoids exposing inbound ports on user machines.                        |
| Log redaction                   | Prevents credentials from streamed logs and agent errors.              |
| Backup guidance                 | Makes Cloud deployment recoverable.                                    |

## Design Decisions

| Decision                                 | Rationale                                                                     |
|------------------------------------------|-------------------------------------------------------------------------------|
| Containerize Cloud mode                  | VM deployments become reproducible and easier to upgrade.                     |
| Keep TLS and login outside the Go server | OAuth2Proxy and fronting proxies are better suited for auth and certificates. |
| Prioritize macOS agent install           | Matches Homebrew distribution direction and immediate users.                  |
| Keep Cloud storage metadata-only         | Source trees, Git credentials, and command execution stay local.              |
| Avoid database service in Cloud v1       | Metadata volume is enough for a single hosted control-plane instance.         |
| Use outbound HTTPS WebSocket             | Works on ordinary home and corporate networks without port forwarding.        |
| Avoid browser extension infrastructure   | Native agent is required for Git, files, terminal, and AI CLI access.         |
