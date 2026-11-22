import { useEffect, useMemo, useState } from 'react';
import type { ClaudeTurn, HarnessAgentRun, HarnessStage, SafeSessionRecord } from '../../lib/types';
import type { HarnessClient } from './harnessClient';
import { stageKindLabels } from './vocabulary';
import type { HxStatus } from './vocabulary';

export const transcriptEvery = 5_000;

export type LiveStep = { id: string; title: string; agentRuns: HarnessAgentRun[] };

/**
 * The run's conversations grouped by the stage that ran them, in the order the
 * stages ran and, within a stage, the order the agents started. A stage of a
 * kind that ran more than once (a retry after a stop) is numbered, so "Plan 2"
 * is the second Plan stage. An agent whose stage is unknown goes last.
 */
export function liveSteps(stages: HarnessStage[], agentRuns: HarnessAgentRun[]): LiveStep[] {
	const byStart = (a: HarnessAgentRun, b: HarnessAgentRun) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? '');
	const live = agentRuns.filter((agentRun) => agentRun.state !== 'queued').sort(byStart);
	const ordered = [...stages].sort((a, b) => a.ordinal - b.ordinal || Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
	const seen: Partial<Record<HarnessStage['kind'], number>> = {};
	const steps: LiveStep[] = [];
	for (const stage of ordered) {
		const own = live.filter((agentRun) => agentRun.stageId === stage.id);
		if (own.length === 0) continue;
		const nth = (seen[stage.kind] ?? 0) + 1;
		seen[stage.kind] = nth;
		const repeated = ordered.filter((other) => other.kind === stage.kind).length > 1;
		steps.push({ id: stage.id, title: repeated ? `${stageKindLabels[stage.kind]} ${nth}` : stageKindLabels[stage.kind], agentRuns: own });
	}
	const placed = new Set(steps.flatMap((step) => step.agentRuns.map((agentRun) => agentRun.id)));
	const rest = live.filter((agentRun) => !placed.has(agentRun.id));
	if (rest.length > 0) steps.push({ id: 'other', title: 'Other', agentRuns: rest });
	return steps;
}

export function agentStatus(agentRun: HarnessAgentRun): HxStatus {
	switch (agentRun.state) {
		case 'running': return 'working';
		case 'succeeded': return 'done';
		case 'failed': return 'failed';
		case 'cancelled': return 'stopped';
		default: return 'not_started';
	}
}

/** The transcript of one agent run, read once when it is finished and every few seconds while it runs. */
export function useAgentTranscript(client: HarnessClient, agentRun?: HarnessAgentRun): { turns?: ClaudeTurn[]; error: string } {
	const [turns, setTurns] = useState<ClaudeTurn[] | undefined>();
	const [error, setError] = useState('');
	const id = agentRun?.id;
	const running = agentRun?.state === 'running';
	useEffect(() => {
		setTurns(undefined);
		setError('');
		if (!id) return;
		const controller = new AbortController();
		let timer: number | undefined;
		const read = () => {
			client.agentRunTranscript(id, 50, controller.signal)
				.then((result) => {
					if (controller.signal.aborted) return;
					setTurns(result.turns);
					if (running && document.visibilityState !== 'hidden') timer = window.setTimeout(read, transcriptEvery);
					else if (running) timer = window.setTimeout(read, transcriptEvery * 4);
				})
				.catch(() => { if (!controller.signal.aborted) setError('The conversation could not be read.'); });
		};
		read();
		return () => { controller.abort(); window.clearTimeout(timer); };
	}, [client, id, running]);
	return { turns, error };
}

export function useAgentSession(client: HarnessClient, agentRunId?: string): { record?: SafeSessionRecord } {
	const [record, setRecord] = useState<SafeSessionRecord | undefined>();
	useEffect(() => {
		setRecord(undefined);
		if (!agentRunId) return;
		const controller = new AbortController();
		client.agentRun(agentRunId, controller.signal).then((result) => { if (!controller.signal.aborted) setRecord(result.session); }).catch(() => undefined);
		return () => controller.abort();
	}, [agentRunId, client]);
	return useMemo(() => ({ record }), [record]);
}
