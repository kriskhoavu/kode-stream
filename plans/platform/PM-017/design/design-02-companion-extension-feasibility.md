# Design: Companion Chrome Extension Feasibility

## Overview

This document evaluates a Chrome extension as a companion surface for Kode Stream. It is explicitly not a replacement for the local CLI runtime.

## Constraints

| Constraint                     | Impact                                                         |
|--------------------------------|----------------------------------------------------------------|
| Browser sandbox                | Extension cannot directly run local Git commands               |
| Filesystem restrictions        | Limited local file access without explicit user interactions   |
| MV3 background lifecycle       | Long-running local orchestration is unreliable in extension    |
| Enterprise policy constraints  | Native host and permission policy can block installation       |

## Viable Architecture

| Component                 | Responsibility                                                    |
|--------------------------|-------------------------------------------------------------------|
| Chrome extension UI      | Capture browser context and trigger user workflows                |
| Local Kode Stream CLI   | Own all file operations, plan generation, and Git operations      |
| Local bridge             | Native messaging host or localhost API bound to `127.0.0.1`       |

## Scope Recommendation

| Stage          | Capability                                              |
|----------------|----------------------------------------------------------|
| Spike          | Create plan from current page and send to local CLI      |
| Pilot add-on   | Show local command/job status in extension panel         |
| Future         | Rich browser capture helpers and workflow shortcuts      |

## Security Model

| Area          | Recommendation                                               |
|---------------|--------------------------------------------------------------|
| Local bridge  | Bind to localhost only and require explicit user consent     |
| Permissions   | Minimize extension permissions in manifest                   |
| Auth          | Reuse local Git credentials; no credential forwarding        |
| Logging       | Redact repository secrets and tokens in extension messages   |

## Go/No-Go Criteria

- Installation complexity does not materially reduce pilot success rate.
- Core flow remains CLI-compatible when extension is absent.
- Security review passes for local bridge and extension permissions.
- Support cost stays acceptable for target team size.
