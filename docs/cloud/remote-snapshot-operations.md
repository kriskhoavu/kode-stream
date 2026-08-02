# Remote Snapshot Operations

## Purpose

Remote Snapshot workspaces provide read-only access to a provider repository at an immutable commit. They do not use a
Cloud Agent, local path, checkout, Git command, file mutation, terminal, AI, runtime, or verification process.

## Implementation Status

The current backend resolves a Remote Snapshot ref to a commit SHA and exposes read-only metadata, tree, and file
endpoints. Workspace details show the repository, ref, commit, and local-terminal guidance. Provider-instance setup,
durable encrypted user connections, the registration UI, snapshot-backed board/search, and the operational smoke flow
are planned follow-up work; do not treat the procedures below as an available self-service workflow.

## Planned Provider Setup

1. Create a named GitHub or Bitbucket Server/Data Center provider instance with a read-only repository credential.
2. Store the credential only in the Cloud deployment secret store; never commit it or return it from an API response.
3. Connect the user identity to that instance and select an authorized repository and branch, tag, or commit.
4. Confirm that Kode Stream resolves the selection to a commit SHA before displaying files.

## Planned Recovery

| Condition                     | User action                                  | Operator action                                                   |
|-------------------------------|----------------------------------------------|-------------------------------------------------------------------|
| Authorization revoked         | Reconnect the provider account.              | Confirm the provider grant still has read-only repository scope.  |
| Repository forbidden          | Select an authorized repository.             | Verify repository membership; do not broaden workspace ownership. |
| Ref missing                   | Select an existing branch, tag, or commit.   | Check retention and renamed branches.                             |
| Provider outage or rate limit | Retry later; no local fallback is attempted. | Review provider health and rate-limit policy.                     |
| Credential rotation           | Reconnect after rotation.                    | Rotate the deployment secret and revoke the old credential.       |

## Planned Smoke Checklist

- Register an operator-owned test repository as a Remote Snapshot workspace.
- Select a branch and confirm the returned workspace includes its resolved commit SHA.
- Browse a tree and file; both responses must report that same SHA.
- Confirm file, Git, terminal, AI, runtime, and verification commands return an unsupported read-only response.
- Register an Agent-Backed workspace and run the PM-032 smoke unchanged.

## Security Boundary

Remote Snapshot responses must not include a local path, dirty state, agent ID, command envelope, or credential material.
Provider writes are outside this release scope.
