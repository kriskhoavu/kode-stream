# Implementation Plan: PM-017 - Local-First Rollout And Distribution

## Overview

Deliver a production-ready local distribution model for Kode Stream across macOS, Windows, and Linux with phased channels, release automation, diagnostics-first onboarding, and pilot-driven hardening.

## Terminology Lock

Use consistently:

- `local-first CLI`
- `distribution channel`
- `release artifact`
- `doctor command`
- `compatibility matrix`
- `companion extension`

## Phases Summary

| Phase | Name                                 | Status |
|-------|--------------------------------------|--------|
| P0    | Scope Lock And Success Criteria      | Draft  |
| P1    | Release Foundation                   | Draft  |
| P2    | macOS Distribution                   | Draft  |
| P3    | Windows Distribution                 | Draft  |
| P4    | Linux Distribution                   | Draft  |
| P5    | Team Pilot                           | Draft  |
| P6    | General Availability                 | Draft  |
| X1    | Companion Extension Spike (Optional) | Draft  |

## Phase P0: Scope Lock And Success Criteria

**Deliverables:**

- [ ] Lock v1 command set: `init`, `plan create`, `plan publish`, `doctor`.
- [ ] Lock v1 Git providers: GitHub and Bitbucket.
- [ ] Lock platform matrix: macOS (arm64/amd64), Windows (x64), Linux (x64).
- [ ] Define measurable success criteria and publish in docs.

**Verification:** document review accepted by maintainers and reflected in `README.md` and PM-017 docs.

---

## Phase P1: Release Foundation

**Deliverables:**

- [ ] Produce reproducible builds for all target OS/arch combinations.
- [ ] Adopt SemVer release policy and changelog generation.
- [ ] Implement checksum generation (`SHA256SUMS`) for every release.
- [ ] Add signing/notarization workflow where available:
  - [ ] macOS signing/notarization path.
  - [ ] Windows code-signing path or explicit unsigned warnings in docs.
- [ ] Implement CI workflow to run tests, build artifacts, and publish GitHub Releases.

**Verification:** run release pipeline on a pre-release tag and validate artifacts, checksums, and notes.

---

## Phase P2: macOS Distribution

**Deliverables:**

- [ ] Create Homebrew tap repository (`<org>/homebrew-tap`).
- [ ] Add `kode-stream` formula referencing release artifact and sha256.
- [ ] Validate install/upgrade/uninstall on clean macOS Intel and Apple Silicon environments.
- [ ] Publish macOS install and quickstart documentation.

**Verification:**

- `brew tap <org>/homebrew-tap`
- `brew install kode-stream`
- `brew upgrade kode-stream`
- `brew uninstall kode-stream`

---

## Phase P3: Windows Distribution

**Deliverables:**

- [ ] Choose v1 Windows channel: winget (preferred) or signed installer.
- [ ] If winget, submit and merge manifest PR for release artifact.
- [ ] Validate install/upgrade/uninstall on a clean Windows VM.
- [ ] Publish Windows docs for Git Credential Manager and SSH checks.

**Verification:** clean-VM install matrix and `kode-stream doctor` pass on each method.

---

## Phase P4: Linux Distribution

**Deliverables:**

- [ ] Publish Linux tarball (`.tar.gz`) and checksum.
- [ ] Publish install script and manual install docs.
- [ ] Validate on Ubuntu LTS and one RHEL/Fedora-family distribution.
- [ ] Capture deferred decision for deb/rpm packaging.

**Verification:** checksum verification + install + `kode-stream doctor` + first publish flow.

---

## Phase P5: Team Pilot

**Deliverables:**

- [ ] Recruit 5-10 pilot users across macOS/Windows/Linux.
- [ ] Run pilot checklist end to end:
  - [ ] install succeeds.
  - [ ] `kode-stream doctor` passes.
  - [ ] create/update/publish docs/plans to GitHub and Bitbucket.
  - [ ] complete one real workflow from start to push.
- [ ] Collect friction points and prioritize top fixes before GA.

**Verification:** pilot report with success/failure rates and prioritized remediation backlog.

---

## Phase P6: General Availability

**Deliverables:**

- [ ] Release stable `v1.0.0`.
- [ ] Publish one-page onboarding with install, auth check, first publish, and troubleshooting.
- [ ] Publish compatibility matrix and rollback guidance.
- [ ] Define release cadence: patch weekly/biweekly, minor monthly.

**Verification:** GA checklist approved and published docs linked from repository root.

---

## Phase X1: Companion Extension Spike (Optional)

**Deliverables:**

- [ ] Build a one-week spike for Chrome companion extension.
- [ ] Confirm extension can trigger local CLI through secure localhost or native messaging.
- [ ] Validate extension adds workflow speed without lowering install success.
- [ ] Produce go/no-go recommendation for post-v1 roadmap.

**Verification:** documented spike demo and decision memo.

## Risk Controls

- [ ] Git auth issues: actionable `doctor` remediation per platform/provider.
- [ ] Path/permission variance: cross-platform integration tests.
- [ ] Broken release risk: checksum validation, canary release, and rollback docs.
- [ ] Trust risk: signed artifacts where possible and transparent release notes.

## Quality Gates Before GA

- [ ] Pilot install success rate greater than 95%.
- [ ] Core flow first-try success rate greater than 90% (`init -> plan publish -> push`).
- [ ] No open critical auth/data-loss bugs.
- [ ] Rollback path confirmed for all active channels.

## Immediate Preparation Checklist

- [ ] Define repository release structure (naming, changelog, checksums).
- [ ] Create Homebrew tap repository.
- [ ] Implement cross-platform CI build and release workflow.
- [ ] Implement `doctor` command and onboarding docs.
- [ ] Prepare pilot user list and feedback template.
