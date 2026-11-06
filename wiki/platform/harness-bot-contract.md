---
slug: platform-harness-bot-contract
title: Harness Bot Contract
pageType: REFERENCE
topics: harness, chat-bot, webex, webhook, hmac, idempotency
summary: The API chat-bot calls to read and decide harness work, and the signed webhook the harness sends it.
sourceRef: |
  PM-044 | Solution 03 bot contract
lastVerified: 2026-11-04
---

## Authentication
<!-- chunkId: platform-harness-bot-contract-authentication -->
<!-- keywords: bearer token, constant time, 503, 401 -->

[live] The bot routes under `/api/harness/bot` carry no Cloud session; each request presents `KODE_STREAM_HARNESS_BOT_TOKEN` as a bearer token, compared in constant time. An unset token answers 503 and a wrong one 401. Evidence: `internal/server/api/harness_bot_controller.go:21`.

## Decisions
<!-- chunkId: platform-harness-bot-contract-decisions -->
<!-- keywords: idempotency key, already decided, stale revision, channel webex -->

[live] `POST /runs/{id}/decision` and `POST /approvals/{id}/decide` take a decision, a reason, a request id, an idempotency key and the person. The same key returns the recorded approval; another key on a decided approval answers 409 `already_decided`; a stale revision answers 409 `conflict`. Evidence: `internal/harness/approvals.go:42`.

A `question` gate is an agent's own stop: the webhook's `approval` carries its `questions` and the asking `role`, approving with a reason sends that reason back to the agent as the answer, and rejecting cancels the run. The decision is recorded with channel `webex` and the actor `webex:<personId>`, which is how the board shows who approved from Webex. The bot's own commands are `/harness status`, `/harness queue [n]`, `/harness run <id>`, `/harness approve <runId> [reason]` and `/harness reject <runId> <reason>`, each decision behind its draft and confirm card.

## Webhook
<!-- chunkId: platform-harness-bot-contract-webhook -->
<!-- keywords: outbox, at least once, hmac sha256, back-off, event id -->

[live] Approval requests, finished stages and blocked or finished runs are written to an outbox in the same transaction as the event and posted to `KODE_STREAM_HARNESS_BOT_WEBHOOK_URL`. Evidence: `internal/harness/notify.go:18`.

[live] Each post carries `X-Harness-Event-Id`, `X-Harness-Attempt` and `X-Harness-Signature: sha256=<HMAC of the body>`. A failure retries after 5 seconds doubling to 10 minutes, and after five attempts the harness records `notification.failed`. The bot dedupes on the event id. Evidence: `internal/harness/adapters/notifier/webhook/webhook.go:31`, `internal/harness/notify.go:32`.

The harness these routes serve is [[platform-harness]].
