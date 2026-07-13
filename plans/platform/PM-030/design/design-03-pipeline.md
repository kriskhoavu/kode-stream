# Pipeline Design: PM-030

## Overview

PM-030 does not need a new deployment pipeline. Pipeline work means adding CI-safe checks to the existing local verification flow and documenting when those checks become required.

## Pipeline Stages

| Stage                 | Command                                                      | Required By                      |
|-----------------------|--------------------------------------------------------------|----------------------------------|
| Dependency check      | `rtk go mod tidy` and clean diff review                      | Gin dependency phase             |
| Backend tests         | `rtk go test ./...`                                          | Every phase                      |
| Transport focus tests | `rtk go test ./internal/server/... ./internal/common/...`    | Error and route migration phases |
| Boundary check        | Go test or script that rejects Gin imports outside transport | First Gin route phase            |
| Frontend typecheck    | `rtk npm run typecheck`                                      | Before route removal             |
| Benchmark comparison  | `rtk go test -bench . ./internal/server/...`                 | Baseline and hardening phases    |

## Required Gates

| Gate             | Condition                                                     | Blocks                           |
|------------------|---------------------------------------------------------------|----------------------------------|
| Baseline gate    | Route inventory and benchmark baseline are recorded.          | First route migration            |
| Error gate       | Domain error mapper supports current envelope and key codes.  | Complex route migration          |
| Parity gate      | Migrated route group has contract or parity tests.            | Old route removal                |
| Boundary gate    | Gin import check passes.                                      | Merge of Gin-backed route groups |
| Cache gate       | Cache policy and invalidation tests exist.                    | Cache decorator rollout          |
| Concurrency gate | Deadline, cancellation, queue full, and shutdown tests exist. | Worker pool rollout              |

## Options Considered

| Option                       | Description                                                | Decision                                                                                        |
|------------------------------|------------------------------------------------------------|-------------------------------------------------------------------------------------------------|
| Big-bang Gin switch          | Replace all `ServeMux` routes with Gin in one PR.          | Rejected because route count and side-effect risk are high.                                     |
| Dual-stack with parity tests | Keep old and new handlers temporarily for selected groups. | Selected because it supports route-by-route rollback.                                           |
| Transport-only migration     | Add Gin without error, cache, or concurrency work.         | Rejected as incomplete because current error mapping and workload behavior remain inconsistent. |
| Architecture rewrite first   | Move all code to new package layout before Gin.            | Rejected because package churn would obscure behavior changes.                                  |

## Design Decisions

| Decision                                          | Alternatives Considered         | Rationale                                                                    |
|---------------------------------------------------|---------------------------------|------------------------------------------------------------------------------|
| Make boundary checks required after Gin lands     | Code review only                | Automated guardrails prevent framework leakage.                              |
| Run frontend typecheck before removing old routes | Backend-only verification       | Client contracts are TypeScript-consumed even when no frontend code changes. |
| Keep benchmark as informational until hardening   | Hard fail on initial p95 target | Early route migration should optimize correctness before tuning.             |
