# UI Automation: PM-044

## Applicability

Browser validation is required: the Inbox, the Pipeline stepper, the Task Queue, the run drawer and the
settings are user-visible workflows. The backend phases are covered by Go tests; this page covers the journeys.

## Runtime Inputs

| Input               | Source                                                                      |
|---------------------|-----------------------------------------------------------------------------|
| Base URL            | Supplied when executing                                                     |
| Storage             | SQLite (the harness does not run on `datadir`)                              |
| One local workspace | Isolated writable Git workspace with a Jira connection or GitHub repository |
| Claude Code         | Installed and signed in on the executing machine                            |
| Bot stub            | A local HTTP receiver for the webhook, with the shared secret               |

## Playbooks

| Journey                                          | Covers           | Status |
|--------------------------------------------------|------------------|--------|
| Fill the queue, pin, dismiss, start              | Scenarios 1–3    | Draft  |
| Plan to approval, approve, build, test, draft PR | Scenarios 4, 6–8 | Draft  |
| Cancel and restart mid-run                       | Scenarios 9–10   | Draft  |
| Approve from the bot surface, check "via Webex"  | Scenario 11      | Draft  |
| Cost by role equals the ledger total             | Scenario 12      | Draft  |

## Latest Result

No journey has been executed.
