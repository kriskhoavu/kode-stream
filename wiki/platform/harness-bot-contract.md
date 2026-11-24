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

[live] Routes under `/api/harness/bot` carry no Cloud session. Each request sends `KODE_STREAM_HARNESS_BOT_TOKEN` as a bearer token, compared in constant time. An unset token returns 503. A wrong token returns 401. Evidence: `internal/server/api/harness_bot_controller.go:19`.

## Decisions
<!-- chunkId: platform-harness-bot-contract-decisions -->
<!-- keywords: idempotency key, already decided, stale revision, webex channel -->

[live] `POST /runs/{id}/decision` and `POST /approvals/{id}/decide` take a decision, a reason, a request ID, an idempotency key and the person. Evidence: `internal/server/api/harness_bot_controller.go:81`.

- The same key returns the recorded approval.
- Another key on a decided approval returns 409 `already_decided`.
- A stale revision returns 409 `conflict`.
- A `question` gate is an agent's own stop. The webhook's `approval` carries its `questions` and the asking `role`. Approving with a reason sends the reason to the agent as the answer. Rejecting cancels the run.
- The decision is recorded with channel `webex` and actor `webex:<personId>`. The board shows that actor.

The bot's commands are `/harness status`, `/harness queue [n]`, `/harness run <id>`, `/harness approve <runId> [reason]` and `/harness reject <runId> <reason>`. Each decision needs a draft and a confirm card.

## Webhook
<!-- chunkId: platform-harness-bot-contract-webhook -->
<!-- keywords: outbox, at least once, hmac sha256, back-off, event id -->

[live] Approval requests, finished stages and blocked or finished runs go to an outbox in the same transaction as the event. The harness posts them to `KODE_STREAM_HARNESS_BOT_WEBHOOK_URL`. Evidence: `internal/harness/notify.go:16`.

[live] Each post carries `X-Harness-Event-Id`, `X-Harness-Attempt` and `X-Harness-Signature: sha256=<HMAC of the body>`. A failure retries after 5 seconds, doubling up to 10 minutes. After five attempts the harness records `notification.failed`. The bot deduplicates on the event ID. Evidence: `internal/harness/adapters/notifier/webhook/webhook.go:28`.

The harness these routes serve is [[platform-harness]].
