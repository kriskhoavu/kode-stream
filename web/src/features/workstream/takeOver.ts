import type { HarnessController } from '../harness/useHarness';
import { agentRoleToRole } from '../harness/vocabulary';
import type { CardWork } from './harnessJoin';

export async function takeOverByHand(work: CardWork, harness: HarnessController): Promise<{ agentRunId?: string }> {
	const run = work.run;
	const working = run ? (harness.state.details[run.id]?.agentRuns.find((agentRun) => agentRun.state === 'running') ?? run.activeAgentRun) : undefined;
	if (!working) return {};
	const role = agentRoleToRole[working.role];
	if (!harness.state.status?.roles.find((candidate) => candidate.role === role)?.paused) await harness.pauseRole(role);
	return { agentRunId: working.id };
}
