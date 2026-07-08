# Design: Doctor Command Specification

## Overview

`kode-stream doctor` is a diagnostics-first command for local setup validation. It should give fast pass/fail feedback, machine-readable output, and actionable remediation steps for each failed check.

## CLI Contract (v1)

| Command | Purpose |
|---------|---------|
| `kode-stream doctor` | Run full local prerequisites and auth checks |
| `kode-stream doctor --provider github` | Run provider-targeted auth/repo checks |
| `kode-stream doctor --provider bitbucket` | Run provider-targeted auth/repo checks |
| `kode-stream doctor --format json` | Emit structured check results |
| `kode-stream doctor --repo <url-or-path>` | Validate repository access against explicit target |
| `kode-stream doctor --strict` | Return non-zero for warnings as well as failures |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0`  | All required checks passed |
| `1`  | At least one required check failed |
| `2`  | Invalid command usage or unsupported flags |
| `3`  | Environment warning only (used when `--strict` is not set) |

## Check Pipeline

Checks run in deterministic order and stop only on unrecoverable prerequisites.

| Order | Check ID | Required | Description |
|-------|----------|----------|-------------|
| 1 | `runtime.binary` | yes | Verify current executable is runnable |
| 2 | `runtime.config-path` | yes | Verify config/data directory is resolvable and writable |
| 3 | `git.installed` | yes | Verify `git` exists on PATH and can run |
| 4 | `git.version` | no | Warn when Git is older than supported baseline |
| 5 | `repo.context` | yes | Resolve current or explicit repository context |
| 6 | `git.remote-config` | yes | Validate remote URL format and provider match |
| 7 | `auth.provider` | yes | Validate provider auth via non-destructive remote check |
| 8 | `repo.read-access` | yes | Verify `ls-remote`/fetch metadata access |
| 9 | `local.port` | no | If service mode requested, validate localhost bind reachability |

## OS-Specific Checks

| OS | Additional Check | Validation |
|----|------------------|------------|
| macOS | notarization status (best effort) | Verify binary metadata when available |
| Windows | credential helper readiness | Verify Git Credential Manager or SSH key path |
| Linux | key tooling availability | Verify shell tools used in install docs are present |

## Provider Validation Strategy

| Provider | Auth Probe | Notes |
|----------|------------|-------|
| GitHub | `git ls-remote <remote>` | Prefer HTTPS/SSH that user already configured |
| Bitbucket | `git ls-remote <remote>` | Same mechanism, provider-specific remediation text |

Doctor must never mutate repository state. No commit, push, branch switch, or file changes are allowed.

## Output Model

### Human Output

```text
kode-stream doctor

PASS runtime.binary        kode-stream executable found
PASS git.installed         git version 2.45.1
PASS repo.context          /Users/me/work/acme-repo
FAIL auth.provider         cannot read remote refs from git@github.com:acme/repo.git
  fix: add SSH key to GitHub or switch remote to HTTPS and configure credential manager

Result: 3 passed, 1 failed, 0 warnings
```

### JSON Output (`--format json`)

```json
{
  "ok": false,
  "summary": {
    "passed": 3,
    "failed": 1,
    "warnings": 0
  },
  "checks": [
    {
      "id": "git.installed",
      "status": "pass",
      "required": true,
      "message": "git version 2.45.1"
    },
    {
      "id": "auth.provider",
      "status": "fail",
      "required": true,
      "message": "cannot read remote refs",
      "remediation": [
        "Add SSH key to provider account",
        "Or switch remote to HTTPS and configure credential manager"
      ]
    }
  ]
}
```

## Error Message Rules

- Messages must identify the exact failed check and target (remote URL, path, or command).
- Remediation must be platform-specific when needed.
- Remediation text should be imperative and testable.
- Sensitive values (tokens, full secrets, auth headers) must never be printed.

## Integration Points

| Layer | Responsibility |
|-------|----------------|
| `cmd/kode-stream` | Parse flags and print output format |
| `internal/application/health` | Reuse health check patterns and compose doctor checks |
| `internal/gitadapter` | Execute non-destructive Git probes |
| `internal/config` | Resolve and validate app data/config directories |

## Test Plan

| Test Type | Coverage |
|-----------|----------|
| Unit | each check success/failure path and message contract |
| Integration | real Git repo with valid and invalid remote/auth states |
| Cross-platform | path/permission and credential helper differences |
| Snapshot | human output and JSON schema stability |

## Rollout

1. Implement minimum required checks (`runtime`, `git`, `repo`, `auth`).
2. Add `--format json` for CI/pilot telemetry use.
3. Expand with optional checks (port and OS-specific tooling) after pilot feedback.
