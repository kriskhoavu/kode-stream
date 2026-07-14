# Infrastructure Design: Cloud And Agent Modes

## Overview

Cloud mode needs a deployable VM/container shape with persistent storage, reverse-proxy TLS, OIDC configuration, and health checks. Agent mode needs installable native binaries, starting with macOS Homebrew, that can run as a background service and open outbound connections to Cloud.

## Cloud Runtime

| Concern      | Design                                                           |
|--------------|------------------------------------------------------------------|
| Process      | Same Go binary serves embedded frontend and Gin API.             |
| Bind address | Cloud mode supports `KODE_STREAM_BIND_ADDR`, normally `0.0.0.0`. |
| TLS          | Terminated by reverse proxy such as Nginx, Caddy, or Traefik.    |
| Persistence  | Data and clone roots are mounted volumes.                        |
| Identity     | OIDC secrets are environment or secret-manager inputs.           |
| Healthcheck  | Container and proxy check `GET /api/health`.                     |
| Backups      | Operator backs up mounted data and clone volumes.                |

## Container Layout

| Layer      | Contents                                                           |
|------------|--------------------------------------------------------------------|
| Builder    | Node.js, npm install, frontend build, Go build.                    |
| Runtime    | Minimal OS image, `git`, CA certificates, optional SSH client.     |
| User       | Non-root user owns data and clone mount paths.                     |
| Entrypoint | Starts `kode-stream serve` with Cloud env configuration.           |
| Volumes    | `/var/lib/kode-stream/data` and `/var/lib/kode-stream/clone-root`. |

The image should not bake OIDC secrets, Git credentials, or user data.

## Cloud Storage

| Storage Area | Default Container Path            | Notes                                                      |
|--------------|-----------------------------------|------------------------------------------------------------|
| Data root    | `/var/lib/kode-stream/data`       | User registries, indexes, audit, settings, agent metadata. |
| Clone root   | `/var/lib/kode-stream/clone-root` | Cloud-managed clones, scoped by derived user ID.           |
| Temp         | runtime temp directory            | Should not hold durable credentials or clone data.         |

Use explicit volume mounts in compose and VM docs.

## Git Credential Options

| Option                    | Priority | Notes                                                                    |
|---------------------------|----------|--------------------------------------------------------------------------|
| Git provider OAuth/App    | first    | Best long-term UX and rotation model for GitHub/GitLab/Bitbucket.        |
| Encrypted PAT/token       | second   | Broad compatibility but requires secure server-side secret storage.      |
| Encrypted SSH private key | later    | High-risk fallback only; requires passphrase, rotation, and delete flow. |

Cloud mode must never ask for local filesystem paths or local SSH agent sockets from a browser user.

## Agent Installation

| OS      | First Support Level     | Install Path                                                     |
|---------|-------------------------|------------------------------------------------------------------|
| macOS   | Production first target | Homebrew formula for `kode-stream-agent` or `kode-stream agent`. |
| Windows | Planned follow-up       | MSI or executable installer with startup task/service.           |
| Linux   | Planned follow-up       | `.deb`, `.rpm`, or tarball with systemd user service.            |

The existing Kode Stream binary may host the agent subcommand, or the release can publish a separate agent binary. The plan should choose the lower-maintenance packaging option during implementation, with macOS Homebrew as the first delivery gate.

## Agent Runtime

| Concern      | Design                                                                                            |
|--------------|---------------------------------------------------------------------------------------------------|
| Startup      | `kode-stream agent start` and optional background service.                                        |
| Doctor       | `kode-stream agent doctor` checks Git, SSH agent, Cloud reachability, and deep-link registration. |
| Deep link    | `kodestream://connect` wakes an installed agent.                                                  |
| Connection   | Agent opens outbound WebSocket to Cloud with short-lived connect token.                           |
| Local access | Agent validates repo paths and executes Git/files/terminal locally.                               |
| Secrets      | Agent does not send Git credentials, SSH keys, or credential-helper output to Cloud.              |

## VM Deployment

| Step | Operator Action                                               |
|------|---------------------------------------------------------------|
| 1    | Provision VM and DNS.                                         |
| 2    | Install Docker or run native binary as a service.             |
| 3    | Configure reverse proxy TLS and public URL.                   |
| 4    | Configure OIDC client and environment secrets.                |
| 5    | Mount data and clone volumes.                                 |
| 6    | Start Cloud mode and verify health.                           |
| 7    | Register admin users and test Git URL workspace registration. |

## Security Controls

| Control                         | Purpose                                                      |
|---------------------------------|--------------------------------------------------------------|
| Non-root container user         | Limits blast radius of process compromise.                   |
| Per-user storage directories    | Prevents accidental cross-user workspace mixing.             |
| Session cookie signing          | Protects Cloud auth sessions.                                |
| CSRF on mutating routes         | Protects browser-authenticated Cloud writes.                 |
| Capability-gated command routes | Prevents non-admin arbitrary command execution in Cloud.     |
| Log redaction                   | Prevents Git credentials from clone progress and error logs. |
| Backup guidance                 | Makes Cloud deployment recoverable.                          |

## Design Decisions

| Decision                               | Rationale                                                             |
|----------------------------------------|-----------------------------------------------------------------------|
| Containerize Cloud mode                | VM deployments become reproducible and easier to upgrade.             |
| Keep TLS outside the Go server         | Reverse proxies are better suited for certificates and routing.       |
| Prioritize macOS agent install         | Matches existing Homebrew distribution direction and immediate users. |
| Defer Windows/Linux polish             | Keeps initial scope focused while preserving portable CLI design.     |
| Avoid browser extension infrastructure | Native agent is required, and extension install does not solve it.    |
