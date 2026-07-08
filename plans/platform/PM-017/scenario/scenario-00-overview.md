# Scenario 0: Cross-Platform Install To First Publish

## Goal

Enable a new teammate to install Kode Stream in under 10 minutes, pass diagnostics, and publish a real plan/doc update to GitHub or Bitbucket from their local machine.

## Starting State

| # | Title                    | Summary                                                              |
|---|--------------------------|----------------------------------------------------------------------|
| 1 | User has a dev machine   | macOS, Windows, or Linux with shell access                           |
| 2 | User has repository auth | SSH keys or HTTPS credentials are configured for GitHub/Bitbucket    |
| 3 | No local install yet     | `kode-stream` is not installed on PATH                              |

## Visual State (Before)

```text
Terminal:
  kode-stream: command not found

Repository host access:
  unknown
```

## Execution Flows

### Flow 0.1: macOS Homebrew Install

```text
User runs brew tap <org>/homebrew-tap
    -> User runs brew install kode-stream
    -> User runs kode-stream doctor
    -> Doctor validates git, auth, repo access, and local prerequisites
    -> User runs init/create/publish flow
    -> User pushes to remote repository successfully
```

### Flow 0.2: Windows Install (winget or installer)

```text
User installs kode-stream via v1 Windows channel
    -> User runs kode-stream doctor
    -> Doctor verifies Git Credential Manager or SSH setup
    -> User runs first publish flow
    -> User pushes to GitHub or Bitbucket successfully
```

### Flow 0.3: Linux Tarball Install

```text
User downloads Linux tarball + SHA256SUMS
    -> User verifies checksum
    -> User installs binary into local PATH
    -> User runs kode-stream doctor and publish flow
    -> User pushes successfully
```

## Visual State (After)

```text
Terminal:
  kode-stream doctor
  PASS: git is installed
  PASS: git auth works
  PASS: repository access confirmed

Release confidence:
  install success >95% in pilot
  core flow success >90% first try
```

## Edge Cases

- User has Git installed but no credential helper or SSH key configured.
- Corporate policy blocks unsigned binaries or extension/native host registration.
- Local port is already occupied if user enables optional local service mode.
- Homebrew/winget metadata points to stale checksums after re-release.
- Windows path and permission differences break first-run config creation.
