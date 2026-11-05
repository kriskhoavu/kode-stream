# Solution 03: Bot Contract

chat-bot is the chat surface. kode-stream ships the bot-facing API, the command contract and the outbound
webhook. The bot change itself is a separate plan in the chat-bot checkout.

## Authentication

Every bot route takes `Authorization: Bearer <KODE_STREAM_HARNESS_BOT_TOKEN>`, compared in constant time.

| Condition                          | Answer                         |
|------------------------------------|--------------------------------|
| Token not configured on the server | 503 `{ code: "unavailable" }`  |
| Header missing or token wrong      | 401 `{ code: "unauthorized" }` |
| Harness storage unsupported        | 503 `{ code: "unavailable" }`  |

The routes are public in the router (no Cloud session), like `/agents/channel`, and authenticate themselves.

## Routes (base `/api/harness/bot`)

| Method | Path                    | Answer                                                    |
|--------|-------------------------|-----------------------------------------------------------|
| GET    | `/status`               | `HarnessStatus`                                           |
| GET    | `/queue?limit=n`        | `{ tasks: HarnessTask[] }`, at most 50                    |
| GET    | `/approvals`            | `{ approvals: HarnessApproval[] }` waiting                |
| GET    | `/runs/:id`             | `HarnessRun` with stages                                  |
| POST   | `/runs/:id/decision`    | `HarnessApproval`: decides the run's one waiting approval |
| POST   | `/approvals/:id/decide` | `HarnessApproval`                                         |

Decision body:

```json
{
  "decision": "approve",
  "reason": "Looks right",
  "requestId": "webex-message-id",
  "idempotencyKey": "bot-7f3c...",
  "expectedRevision": 1,
  "actor": { "personId": "Y2lzY29...", "email": "anna@example.com", "displayName": "Anna" }
}
```

`expectedRevision` is optional for the bot: when absent the current revision is used. `actor.personId` is
required. The decision is recorded with `channel: "webex"` and `decidedBy: { actorId: "webex:<personId>",
displayName, email }`, which is how the dashboard shows "Approved via Webex by Anna". Replaying the same `idempotencyKey` answers 200 with the recorded approval. A decision on an
approval already decided under another key answers 409 `already_decided`. A stale `expectedRevision` answers 409
`conflict`. `/runs/:id/decision` answers 404 when the run has no waiting approval.

## Commands

| Command                             | Calls                                            |
|-------------------------------------|--------------------------------------------------|
| `/harness status`                   | `GET /status`                                    |
| `/harness queue [n]`                | `GET /queue?limit=n`                             |
| `/harness run <id>`                 | `GET /runs/:id`                                  |
| `/harness approve <runId> [reason]` | draft → confirm card → `POST /runs/:id/decision` |
| `/harness reject <runId> <reason>`  | draft → confirm card → `POST /runs/:id/decision` |

The bot's two-step draft → confirm card carries `requestId` and `idempotencyKey`, as its deploy-tag card does.

## Outbound Webhook

`POST $KODE_STREAM_HARNESS_BOT_WEBHOOK_URL` for the event types `approval.requested`, `stage.finished`,
`run.blocked` and `run.finished`.

| Header                | Value                                                      |
|-----------------------|------------------------------------------------------------|
| `X-Harness-Event-Id`  | `evt-<seq>`                                                |
| `X-Harness-Signature` | `sha256=<hex HMAC-SHA256 of the raw body with the secret>` |
| `X-Harness-Attempt`   | `1` … `5`                                                  |
| `Content-Type`        | `application/json`                                         |

Body:

```json
{
  "eventId": "evt-42",
  "seq": 42,
  "kind": "approval.requested",
  "at": "2026-11-04T09:12:00Z",
  "task": { "id": "tsk-…", "sourceKey": "PM-02", "title": "…", "url": "https://…" },
  "run": { "id": "run-…", "stage": "plan_approval", "round": 2, "costUsd": 1.42, "turns": 37 },
  "approval": { "id": "apr-…", "gate": "plan", "revision": 1, "expiresAt": "…", "questions": ["…"], "role": "planner" },
  "summary": "≤600 characters",
  "links": { "dashboard": "http://127.0.0.1:4317/harness/runs/run-…" }
}
```

The bot answers `202 { "received": true }` and dedupes on `eventId`. Delivery is at least once: the outbox retries
every 5 seconds with back-off from 5 seconds to 10 minutes, at most 5 attempts, then appends
`notification.failed`.
