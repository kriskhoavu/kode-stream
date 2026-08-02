# Terminal Canvas Sessions

Terminal Canvas presents durable session history without making terminal processes durable. A plan launch creates a
safe session record and, when startup succeeds, associates it with the existing in-memory embedded-terminal binding.
Canvas reuses that binding and terminal presentation; selecting another node does not launch or cancel a process.

## Branch-Safe Launch

The launch request carries the workspace, plan, expected branch, observed commit, and one idempotency key for the pending
submission. Immediately before process start, the server resolves the plan again, reads the current checkout, and
revalidates the `terminal.launch` capability. A mismatch returns `terminal_branch_mismatch` with safe expected/current
branch context. Canvas never switches branches automatically, and repeated requests with one key start at most one
process.

## Durable And Live State

The durable record may contain workspace and plan references, provider, bounded intent, requested branch, observed
commit, lifecycle timestamps, state, and exit code. It never contains prompts, executable arguments, environment
variables, grants, input, output, credentials, file content, or terminal buffers.

The process manager remains the authority for the PTY, process, grants, subscribers, reconnect lease, buffer, and
cancellation. A page reload may reattach while that binding still exists. After an application restart, a record left
starting or running without a binding becomes interrupted and does not offer false reconnection.

Removing a session from Canvas deletes only its placement. **Cancel process** is a separate confirmed operation and
leaves the durable record and placement available.

## Capability Boundary

PM-037 delivers Local checkout plus Local process execution. Cloud Agent execution is a future execution-provider
adapter. Agentless Remote Snapshot support is a separate content-provider capability and is not a restricted Canvas
mode or a delivered terminal surface.
