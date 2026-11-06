import type { HarnessController } from '../harness/useHarness';
import { agentRoleToRole } from '../harness/vocabulary';
import type { CardWork } from './harnessJoin';

/**
 * Hands a card from the harness to a person: the role working on it pauses,
 * so it finishes the round it is on and starts nothing more, and the caller
 * opens the Sessions mode for the person to continue from. Nothing waiting on
 * an approval is decided.
 */
export async function takeOverByHand(work: CardWork, harness: HarnessController): Promise<void> {
	const run = work.run;
	const working = run ? (harness.state.details[run.id]?.agentRuns.find((agentRun) => agentRun.state === 'running') ?? run.activeAgentRun) : undefined;
	if (!working) return;
	const role = agentRoleToRole[working.role];
	if (harness.state.status?.roles.find((candidate) => candidate.role === role)?.paused) return;
	await harness.pauseRole(role);
}
