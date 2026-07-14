# Pipeline Design: Cloud And Agent Modes

## Overview

The pipeline must verify mode-aware backend behavior, frontend capability gating, container build health, and macOS agent packaging. Cloud mode can ship first once image build, server smoke, and auth/config validation are reliable. Agent mode packaging can start with macOS Homebrew and later add Windows/Linux channels.

## Build Outputs

| Output                | Trigger             | Purpose                                              |
|-----------------------|---------------------|------------------------------------------------------|
| Local binary          | existing release    | Current local app distribution.                      |
| Cloud container image | Cloud phase release | VM/container deployment artifact.                    |
| macOS agent artifact  | Agent phase release | Homebrew-installed local agent or binary subcommand. |
| Checksums             | every release       | Artifact verification.                               |
| Deployment docs       | Cloud release       | Operator setup and upgrade instructions.             |

## Verification Stages

| Stage              | Command Or Check                 | Gate                                                         |
|--------------------|----------------------------------|--------------------------------------------------------------|
| Go tests           | `rtk go test ./...`              | Backend mode, auth, storage, Git, agent, and API tests pass. |
| Frontend typecheck | `rtk npm run typecheck`          | Runtime types and capability UI compile.                     |
| Frontend tests     | `rtk npm test -- --run`          | UI role/mode tests pass.                                     |
| Production build   | `rtk npm run build` and Go build | Embedded frontend and binary build.                          |
| Container build    | Docker build                     | Image builds without secrets.                                |
| Container smoke    | Run image with test env          | `/api/health` and Cloud startup validation pass.             |
| Agent package      | Homebrew formula/test            | macOS install and `agent doctor` smoke pass.                 |

## Cloud Release Gates

| Gate              | Requirement                                                              |
|-------------------|--------------------------------------------------------------------------|
| Config validation | Cloud mode fails fast when required OIDC/session settings are missing.   |
| Healthcheck       | Health endpoint remains unauthenticated and stable.                      |
| Auth smoke        | Test or documented local OIDC mock flow verifies login/session behavior. |
| Role policy       | Representative viewer/editor/admin route tests pass.                     |
| Clone smoke       | Git URL registration works against a test repository.                    |
| Secret hygiene    | Build and logs do not include configured credentials.                    |
| Rollback          | Previous image can reuse the same data volume layout.                    |

## Agent Release Gates

| Gate                | Requirement                                                                  |
|---------------------|------------------------------------------------------------------------------|
| macOS install       | Homebrew install or service setup succeeds on supported macOS target.        |
| Doctor              | `kode-stream agent doctor` checks Git, SSH agent visibility, and Cloud URL.  |
| Deep link           | `kodestream://connect` launches or focuses installed agent on macOS.         |
| Outbound connection | Agent establishes authenticated WebSocket to Cloud.                          |
| Local repo scan     | Agent registers and scans an existing local Git repo.                        |
| Credential boundary | Tests and review confirm no SSH keys or Git credential output is serialized. |

## Documentation Gates

| Document            | Required Content                                                    |
|---------------------|---------------------------------------------------------------------|
| README              | Local, Cloud, and Agent mode summary.                               |
| Cloud deploy guide  | Container/VM setup, env vars, OIDC, reverse proxy, backup, upgrade. |
| Agent install guide | macOS Homebrew first, Windows/Linux planned, reconnect flow.        |
| Security notes      | Credential handling, role policy, command execution risks.          |
| Troubleshooting     | OIDC failures, clone failures, agent offline, reverse proxy issues. |

## CI And Local Commands

| Scope    | Command                                                                    |
|----------|----------------------------------------------------------------------------|
| Backend  | `rtk go test ./...`                                                        |
| Frontend | `rtk npm run typecheck && rtk npm test -- --run`                           |
| Build    | `rtk npm run build && rtk go build -o ./bin/kode-stream ./cmd/kode-stream` |
| Docker   | Build image, run with Cloud env, check `/api/health`.                      |
| Homebrew | Formula install/test once agent packaging phase begins.                    |

## Design Decisions

| Decision                                 | Rationale                                                              |
|------------------------------------------|------------------------------------------------------------------------|
| Add Cloud container smoke before release | Runtime config and bind behavior are deployment-sensitive.             |
| Keep full Go tests as backend gate       | Mode and auth changes cross many route families.                       |
| Verify frontend capability states        | UI must not expose broken or dangerous controls in Cloud mode.         |
| Gate macOS agent separately              | Agent mode is designed in PM-032 but ships after Cloud mode.           |
| Document VM operations                   | Cloud users need backup, proxy, and OIDC setup, not only code changes. |
