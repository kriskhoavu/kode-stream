# Canvas Verification Freshness

Canvas reports command outcome and repository freshness as separate facts. A historical passing command is not shown as
current after the repository inputs change.

## Fingerprint

Each verification run captures a deterministic fingerprint at start and completion. It covers branch, HEAD, staged
content, relevant tracked and untracked worktree content, and the selected verification configuration. The resolver also
computes the current fingerprint whenever Canvas state is refreshed.

## Freshness States

| State          | Meaning                                                                                    |
|----------------|--------------------------------------------------------------------------------------------|
| `fresh`        | Start and completion matched, and the current fingerprint still matches the completed run. |
| `stale`        | The repository or verification configuration changed after the completed result.           |
| `inconclusive` | State changed during the run or a complete fingerprint could not be produced.              |

Branch changes, commits, staged changes, unstaged changes, relevant untracked files, and verification-configuration
changes can make a result stale. Canvas refreshes on explicit Git/Canvas refresh, after verification polling, when the
window regains focus, and when the document becomes visible. Position refresh never moves a saved node.

Verification jobs remain in memory for PM-037. Application restart does not restore verification history; durable
verification lineage is future work. The Workbench shows abbreviated verified and current revisions and labels a stale
pass as **Passed (historical)**.
