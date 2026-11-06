# Solution 04: One board

The Harness page goes away. A ticket is one card on the Workstream board whether a person or the harness works on it,
the card's Agents tab holds both a harness run and a hand-driven session, and the harness's settings live under
Settings. The mockup this page describes is the eight-screen canvas "Workstream and Harness, one board".

## Rules

1. **One card per ticket.** The Jira key joins a harness task (`HarnessTask.source.key`) to a work item
   (`WorkItem.jiraKey`). A card's column follows the item's status; a harness run moves the item
   (Draft → In Progress → Review) the way a person would. A task the Finder found that no plan holds yet is a card in
   the Unsorted column, marked "no plan yet".
2. **Who, not where.** "By hand" or "Harness" is a chip on the card and a "Worked by" facet in the filter bar, never
   a page. Switching who works on a card is one action: Run with the harness, Take over by hand.
3. **No Harness page.** What waits on you is the amber ring on the card and the strip above the board. What is
   working and what is done are columns. The queue is a view of the board. Settings hold the rest.
4. **One Agents tab, two modes.** "Harness run" shows the compact stepper, the working agent, the Checker's findings,
   the ledger and the steering actions. "Sessions" is the hand-driven session that exists today. Both list their
   conversations the same way and open the same transcript view.
5. **A source says where it shows.** Each workspace source carries `shownIn: ['workstream' | 'knowledge']`. Only
   sources shown in Workstream feed the board; Knowledge reads every source shown there. Defaults keep every page as
   it is: a source named `plans` is Workstream only, one named `wiki` is Knowledge only, anything else is both.

## Screens

| Screen               | Where                                    | Holds                                                                                                                                                    |
|----------------------|------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| Board                | `/` Status view                          | Harness-aware cards (worker chip, stage chip, progress bar, amber ring), the "Needs you" strip, the Worked by facet                                      |
| Waves, Queue         | `/` view switcher Status · Waves · Queue | Waves as today's wave board over the same cards; Queue as ranked rows with "why here", score, Start, Retry, pin, drag                                    |
| Card drawer          | Workstream drawer, Agents tab            | Harness run mode: chip stepper, working agent, findings, ledger, Pause after this round, Take over by hand, Add a note; Sessions mode: today's AgentsTab |
| Card full width      | `/items/:id?view=run`                    | Today's run page whole: wide stepper, plan preview, Steps · Findings · Changes · Conversations, Stop in the top bar                                      |
| Run with the harness | card menu and drawer header              | Dialog: branch, model per role, approvals wanted, start now or queue                                                                                     |
| Sources              | Workspaces page, each source row         | "Shown in" Workstream and Knowledge ticks                                                                                                                |
| Settings             | `/settings`, Harness section             | Who does what, where work comes from, how it runs, pause a role, spend note                                                                              |

## What leaves

- The Harness navigation button, `/harness/*` routes, `HarnessPage`, `InboxView`, `ApprovalsView` and the
  harness count pill. `/harness/runs/:id` redirects to the item that holds the run; the other paths redirect to `/`.
- Budgets stay out: no spend meter, no limit. Cost shows per run in the ledger only.

## Data the merge needs

- `GET /api/harness/queue` and `GET /api/harness/runs` already carry `sourceKey`; the board joins them to items on
  the client by `jiraKey`. A task with no item of its key renders as a synthetic card from the task alone.
- `POST /api/harness/tasks` takes an item's Jira key, title and body for "Run with the harness" on a card the Finder
  did not create, and returns the task; the dispatcher then starts it as a `start` gate decided at once.
- A workspace source becomes `{ path, shownIn }`; a bare string still parses as the default above.
