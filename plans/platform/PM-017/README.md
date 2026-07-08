# PM-017: Local-First Rollout And Distribution

PM-017 defines the v1 rollout plan to ship Kode Stream as a local-first CLI across macOS, Windows, and Linux. The plan keeps all repository work, credentials, and Git identity on user machines and adds release, distribution, and support workflows needed for broad adoption.

## Related Plans

| Item                          | Relationship                     | Key Context                                                                      |
|-------------------------------|----------------------------------|----------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Infrastructure baseline          | Existing CI/CD and release automation patterns can be reused and extended        |
| [PM-003](../PM-003/README.md) | Backend/API baseline             | Existing API/app boundaries guide where `doctor` checks should be implemented    |
| [PM-015](../PM-015/README.md) | Architecture and conventions     | Provides current implementation ownership and verification discipline             |
| [PM-016](../PM-016/README.md) | Local machine Git integration    | Reinforces local Git auth behavior and no centralized identity assumptions       |

## Scope

### Goal

Ship Kode Stream as a local tool on macOS/Windows/Linux with reproducible releases and clear onboarding so teammates can install quickly and publish plans/docs to GitHub and Bitbucket.

### Non-Goals

- No hosted API or centralized backend service.
- No server-side database or centralized filesystem.
- No server-managed Git credential or identity model.
- No extension-only product path in v1.

## Glossary

| Term                  | Meaning                                                                 |
|-----------------------|-------------------------------------------------------------------------|
| Local-First CLI       | Primary runtime is a user-installed local binary                        |
| Doctor Command        | Diagnostic command that validates local prerequisites and Git readiness  |
| Distribution Channel  | User install path such as Homebrew, winget, installer, or tarball       |
| Release Artifact      | Versioned binary bundle uploaded to GitHub Releases                     |
| Compatibility Matrix  | Documentation table of supported OS/arch/channel combinations           |
| Companion Extension   | Optional browser plugin that talks to local CLI but does not replace it |

## Data Flow

```text
Developer bumps version
  -> CI runs tests and cross-platform builds
  -> CI signs artifacts when available and generates SHA256SUMS
  -> CI publishes GitHub Release artifacts and changelog
  -> package channels (brew/winget/tarball docs) reference the new artifacts
  -> user installs locally and runs `kode-stream doctor`
  -> user runs core flow (init -> plan create -> plan publish -> git push)
```

## Design Decisions

| Decision                                                                    | Rationale                                                                 |
|-----------------------------------------------------------------------------|---------------------------------------------------------------------------|
| Keep CLI as the primary product surface in v1                               | Core value depends on local files and local Git credentials               |
| Publish by channel in phases (macOS first, then Windows, then Linux harden) | Reduces rollout risk and gives fast real-user feedback                    |
| Require `doctor` as a first-class command                                   | Auth and local environment issues are top rollout risk                    |
| Prefer GitHub + Bitbucket provider support in v1                            | Covers immediate team needs while constraining support scope              |
| Use SemVer and changelog automation                                          | Improves trust, rollback, and predictable upgrade expectations            |
| Treat browser extension as a deferred companion feature                      | Browser UX can be additive but cannot replace local CLI capabilities      |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Release And Distribution Design](design/design-01-release-and-distribution.md)
- [Companion Extension Feasibility](design/design-02-companion-extension-feasibility.md)
- [Doctor Command Specification](design/design-03-doctor-command.md)
- [Homebrew Tap And Formula Playbook](design/design-04-homebrew-tap-and-formula.md)
- [Implementation Plan](implementation-plan.md)
