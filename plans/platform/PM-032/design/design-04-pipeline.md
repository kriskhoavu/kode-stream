# Pipeline Design: Cloud Mode With Local Agent Execution

## Overview

The pipeline verifies Local and Cloud runtime behavior, hosted app deployment, frontend capability gating, Cloud Agent
packaging, and the security boundary that keeps repository execution on user machines.

## Build Outputs

| Output                | Trigger       | Purpose                                                    |
|-----------------------|---------------|------------------------------------------------------------|
| Local binary          | release       | Local app and optional agent subcommand.                   |
| Cloud container image | Cloud release | Hosted UI, API, trusted-proxy auth, metadata, and routing. |
| macOS agent artifact  | Cloud release | Homebrew-installed Cloud Agent.                            |
| Checksums             | release       | Artifact verification.                                     |
| Deployment docs       | Cloud release | Operator and user setup instructions.                      |

## Verification Stages

| Stage              | Command Or Check                 | Gate                                                     |
|--------------------|----------------------------------|----------------------------------------------------------|
| Go tests           | `rtk go test ./...`              | Runtime, auth, storage, agent, and API tests pass.       |
| Frontend typecheck | `rtk npm run typecheck`          | Runtime types and capability UI compile.                 |
| Frontend tests     | `rtk npm test -- --run`          | UI role, mode, and agent-state tests pass.               |
| Production build   | `rtk npm run build` and Go build | Embedded frontend and binary build.                      |
| Container build    | Docker build                     | Image builds without secrets or repository contents.     |
| Container smoke    | Run image with test env          | `/api/health` and Cloud startup validation pass.         |
| Agent package      | Homebrew formula/test            | macOS install, deep link, and `agent doctor` smoke pass. |

## Cloud Release Gates

| Gate                | Requirement                                                                                    |
|---------------------|------------------------------------------------------------------------------------------------|
| Config validation   | Cloud mode fails fast when required public URL, cookie secret, and admin settings are missing. |
| Healthcheck         | Health endpoint remains unauthenticated and stable.                                            |
| Auth smoke          | OAuth2Proxy forwards identity headers and Kode Stream reports Cloud mode.                      |
| Role policy         | Representative viewer/editor/admin route tests pass.                                           |
| Agent connection    | Agent establishes authenticated WebSocket to Cloud.                                            |
| Proxy WebSocket     | Reverse proxy supports `/api/agents/channel` upgrade and long sessions.                        |
| Local repo scan     | Agent registers and scans a local Git repo.                                                    |
| Command routing     | File, Git, terminal, AI, runtime, and verification route through agent.                        |
| Credential boundary | No SSH keys or Git credential output is serialized to Cloud.                                   |
| Hosted boundary     | Cloud image and tests contain no repository storage or command execution path.                 |
| Metadata volume     | Cloud smoke persists user, agent, workspace, and audit metadata without a DB.                  |
| Secret hygiene      | Build and logs do not include configured credentials.                                          |

## Documentation Gates

| Document            | Required Content                                                                       |
|---------------------|----------------------------------------------------------------------------------------|
| README              | Local and Cloud mode summary with Cloud Agent execution.                               |
| Cloud deploy guide  | Container/VM setup, env vars, OAuth2Proxy, Keycloak, metadata volume, backup, upgrade. |
| Agent install guide | macOS Homebrew first, Windows/Linux planned, reconnect flow.                           |
| Security notes      | Credential handling, role policy, agent routing, command boundaries.                   |
| Troubleshooting     | OAuth2Proxy/Keycloak failures, agent offline, deep-link, WebSocket proxy, VPN issues.  |

## CI And Local Commands

| Scope    | Command                                                                    |
|----------|----------------------------------------------------------------------------|
| Backend  | `rtk go test ./...`                                                        |
| Frontend | `rtk npm run typecheck && rtk npm test -- --run`                           |
| Build    | `rtk npm run build && rtk go build -o ./bin/kode-stream ./cmd/kode-stream` |
| Docker   | Build image, run with Cloud env, check `/api/health`.                      |
| Homebrew | Formula install/test for Cloud Agent packaging.                            |

## Design Decisions

| Decision                           | Rationale                                                       |
|------------------------------------|-----------------------------------------------------------------|
| Verify Cloud and agent together    | Cloud workspaces require a connected user-machine agent.        |
| Keep full Go tests as backend gate | Mode, auth, routing, and agent changes cross route families.    |
| Verify frontend capability states  | UI must not expose command controls when the agent is offline.  |
| Gate macOS agent packaging         | Cloud mode depends on a reliable user-machine execution bridge. |
| Document VM operations             | Operators need backup, proxy, and OIDC setup guidance.          |
