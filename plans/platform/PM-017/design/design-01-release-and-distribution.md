# Design: Release And Distribution Architecture

## Overview

PM-017 introduces a release architecture that produces reproducible binaries, channel-ready metadata, and diagnostics-first onboarding. The model keeps all runtime behavior local and does not introduce any hosted service dependency.

## Platform Matrix (v1)

| OS      | Architecture             | Channel Priority                |
|---------|--------------------------|---------------------------------|
| macOS   | arm64, amd64             | Homebrew tap                    |
| Windows | x64                      | winget (preferred) or installer |
| Linux   | x64                      | tarball + checksum              |

## Command Scope (v1)

| Command         | Purpose                                             |
|-----------------|-----------------------------------------------------|
| `init`          | Initialize Kode Stream config in local repository  |
| `plan create`   | Create a plan artifact locally                      |
| `plan publish`  | Publish docs/plans through local Git workflow       |
| `doctor`        | Validate local runtime, auth, and repo access       |

## Release Artifacts

| Artifact                | Description                                              |
|-------------------------|----------------------------------------------------------|
| OS/arch binary archives | Versioned bundles for each supported target              |
| `SHA256SUMS`            | Checksum manifest for all artifacts                      |
| Changelog entry         | Release notes from conventional commit metadata          |
| Signature metadata      | Signing/notarization output when available               |

## CI/CD Contract

| Stage         | Inputs                    | Outputs                                      |
|---------------|---------------------------|----------------------------------------------|
| Verify        | source + lockfiles        | test and lint/typecheck pass/fail            |
| Build         | tagged version            | reproducible binaries per platform            |
| Package       | binaries                  | archives, checksums, optional signatures      |
| Publish       | package outputs           | GitHub Release assets + release notes         |
| Channel sync  | release metadata          | Homebrew formula updates and winget manifest  |

## Doctor Command Checks

| Check Area          | Example Validation                                 | Failure Guidance                                      |
|---------------------|----------------------------------------------------|-------------------------------------------------------|
| Runtime             | binary executable and writable config locations    | fix PATH/permissions                                  |
| Git                 | `git --version` and basic command availability     | install Git and restart shell                         |
| Auth                | provider-specific read check over HTTPS/SSH        | setup SSH key or credential manager                   |
| Repo access         | can fetch/list refs for configured remote          | verify remote URL and user permissions                |
| Optional local port | if service mode enabled, local listener is reachable | choose free port or stop conflicting process          |

## Security And Trust Model

| Area                | Decision                                              |
|---------------------|-------------------------------------------------------|
| Credentials         | Stay in user-managed local Git auth stores            |
| Filesystem          | Operate only on user-selected local repositories      |
| Network             | Only remote Git provider operations required by user  |
| Integrity           | Publish checksums and signed artifacts where possible |
| Rollback            | Preserve prior versions in release channels           |

## Quality Gates Before GA

- Install success rate greater than 95% during pilot.
- Core flow success rate greater than 90% on first try.
- No unresolved critical auth or data-loss issues.
- Documented rollback per channel (brew pin, prior binaries, versioned installers).

## Design Decisions

| Decision                                           | Rationale                                                           |
|----------------------------------------------------|---------------------------------------------------------------------|
| Use phased channels over all-at-once release       | Limits blast radius and isolates platform failures                  |
| Bake diagnostics into v1 instead of support docs only | Faster self-service remediation and lower onboarding support load |
| Keep Linux tarball first                            | Fastest path to coverage while packaging strategy matures           |
| Keep localhost service optional                     | Supports future browser companion without changing CLI-first model  |
