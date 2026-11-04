import type { HarnessAgentRole, HarnessAgentRun, HarnessApproval, HarnessRun, HarnessRunDetail, HarnessStage, HarnessStageKind } from '../../lib/types';
import type { HxStatus, HxStep } from './vocabulary';
import { agentRunStatus, channelLabels, formatCost, formatMinutes, hxSteps, plural, stageStatus } from './vocabulary';

/**
 * Turns a run's stages, agent runs and approvals into the stepper people
 * read: groups of plain steps, each with one status. Pure, so the stepper,
 * the drawer's vertical stepper and the tests share one reading.
 */

export interface StepCard {
	id: string;
	step: HxStep;
	label: string;
	status: HxStatus;
	/** Short facts under the name: who, how long, how much. */
	meta: string[];
	/** When the step is running, the moment it started, for a live timer. */
	since?: string;
	current: boolean;
	agentRun?: HarnessAgentRun;
}

export interface StepGroup {
	id: string;
	label: string;
	cards: StepCard[];
}

export interface PipelineModel {
	groups: StepGroup[];
	current?: StepCard;
	/** Index of the group the flow connector leads into, if work is moving. */
	activeGroup: number;
}

function latest(agentRuns: HarnessAgentRun[], stageId: string | undefined, roles: HarnessAgentRole[]): HarnessAgentRun | undefined {
	return agentRuns
		.filter((agentRun) => agentRun.stageId === stageId && roles.includes(agentRun.role))
		.sort((a, b) => a.round - b.round || Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''))
		.at(-1);
}

function sum(agentRuns: HarnessAgentRun[], key: 'turns' | 'durationMs' | 'costUsd'): number {
	return agentRuns.reduce((total, agentRun) => total + agentRun[key], 0);
}

function fallback(stage: HarnessStage | undefined, producer: boolean): HxStatus {
	if (!stage) return 'not_started';
	if (stage.state === 'running') return producer ? 'working' : 'not_started';
	if (stage.state === 'awaiting_approval') return 'waiting';
	return stageStatus(stage.state);
}

function agentCard(id: string, step: HxStep, stage: HarnessStage | undefined, agentRuns: HarnessAgentRun[], roles: HarnessAgentRole[], producer: boolean, idleNote: string): StepCard {
	const mine = agentRuns.filter((agentRun) => agentRun.stageId === stage?.id && roles.includes(agentRun.role));
	const last = latest(agentRuns, stage?.id, roles);
	const status = last ? agentRunStatus(last.state, last.verdict) : fallback(stage, producer);
	const meta: string[] = [];
	if (last?.state === 'running') {
		meta.push(plural(last.turns, 'turn'), formatCost(last.costUsd));
	} else if (mine.length) {
		if (!producer && last?.verdict) meta.push(last.verdict === 'approve' ? 'approved' : `${last.blocking} must fix`);
		if (producer && mine.length > 1) meta.push(plural(new Set(mine.map((agentRun) => agentRun.round)).size, 'round'));
		meta.push(formatMinutes(sum(mine, 'durationMs')), formatCost(sum(mine, 'costUsd')));
	} else if (status === 'not_started') {
		meta.push(idleNote);
	}
	return { id, step, label: hxSteps[step].label, status, meta, since: last?.state === 'running' ? last.startedAt : undefined, current: false, agentRun: last };
}

function approvalCard(id: string, step: HxStep, stage: HarnessStage | undefined, approval: HarnessApproval | undefined, idleNote: string, now: number): StepCard {
	let status: HxStatus = stage ? stageStatus(stage.state) : 'not_started';
	if (stage?.state === 'awaiting_approval' || approval?.state === 'requested') status = 'waiting';
	if (approval?.state === 'approved') status = 'done';
	if (approval?.state === 'rejected') status = 'needs_fix';
	const meta: string[] = [];
	if (approval?.decidedBy && approval.state !== 'requested') {
		meta.push(`${approval.decidedBy.displayName} · ${approval.channel ? channelLabels[approval.channel] : 'here'}`);
		if (approval.decidedAt) meta.push(ago(approval.decidedAt, now));
	} else if (status === 'waiting') {
		meta.push('waiting on you');
	} else if (status === 'not_started') {
		meta.push(idleNote);
	}
	return { id, step, label: hxSteps[step].label, status, meta, since: status === 'waiting' ? stage?.startedAt : undefined, current: false };
}

function ago(at: string, now: number): string {
	const minutes = Math.max(0, Math.round((now - Date.parse(at)) / 60_000));
	return minutes < 1 ? 'just now' : minutes < 60 ? `${minutes} min ago` : `${Math.round(minutes / 60)} h ago`;
}

function roundLabel(name: string, stage: HarnessStage | undefined): string {
	return stage && stage.maxRounds > 1 && stage.state !== 'pending' ? `${name} · round ${stage.round} of ${stage.maxRounds}` : name;
}

export function pipelineModel(detail: HarnessRunDetail, now = Date.now()): PipelineModel {
	const stageOf = (kind: HarnessStageKind) => detail.stages.find((stage) => stage.kind === kind);
	const approvalOf = (gate: HarnessApproval['gate']) => detail.approvals
		.filter((approval) => approval.gate === gate)
		.sort((a, b) => Date.parse(a.requestedAt) - Date.parse(b.requestedAt))
		.at(-1);
	const plan = stageOf('plan');
	const implement = stageOf('implement');
	const qa = stageOf('qa');
	const start = approvalOf('start');

	const startCard: StepCard = start
		? approvalCard('start', 'start', undefined, start, 'you start it', now)
		: { id: 'start', step: 'start', label: hxSteps.start.label, status: 'done', meta: ['started on its own'], current: false };

	const testStatus: HxStatus = qa ? (qa.state === 'awaiting_approval' ? 'waiting' : stageStatus(qa.state)) : 'not_started';
	const testCard: StepCard = {
		id: 'test', step: 'test', label: hxSteps.test.label, status: testStatus,
		meta: testStatus === 'not_started' ? ['runs twice'] : qa?.state === 'running' ? [`run ${qa.round} of ${qa.maxRounds}`] : qa ? [formatMinutes(qa.durationMs)] : [],
		since: qa?.state === 'running' ? qa.startedAt : undefined, current: false
	};
	const fixCard = agentCard('fix', 'fix', qa, detail.agentRuns, ['qa_fixer'], true, 'only if a test fails');
	if (qa && qa.state === 'passed' && !fixCard.agentRun) {
		fixCard.status = 'skipped';
		fixCard.meta = ['not needed'];
	} else if (!fixCard.agentRun && fixCard.status === 'working') {
		fixCard.status = 'not_started';
		fixCard.meta = ['only if a test fails'];
	}

	const draft = stageOf('draft_pr');
	const prGate = stageOf('pr_approval');
	const handOff: StepCard[] = [];
	if (prGate && prGate.state !== 'skipped') handOff.push(approvalCard('pr_approval', 'pr_approval', prGate, approvalOf('draft_pr'), 'after the tests', now));
	const draftStatus: HxStatus = draft ? stageStatus(draft.state) : 'not_started';
	handOff.push({
		id: 'draft_pr', step: 'draft_pr', label: hxSteps.draft_pr.label, status: draftStatus,
		meta: detail.run.prUrl ? [prLabel(detail.run.prUrl)] : draftStatus === 'not_started' ? [prGate ? 'you approve first' : 'after the tests'] : [],
		current: false
	});

	const groups: StepGroup[] = [
		{ id: 'start', label: 'Start', cards: [startCard] },
		{ id: 'plan', label: roundLabel('Plan', plan), cards: [
			agentCard('plan', 'plan', plan, detail.agentRuns, ['planner'], true, 'first step'),
			agentCard('plan_check', 'plan_check', plan, detail.agentRuns, ['plan_reviewer'], false, 'after Plan')
		] },
		{ id: 'approval', label: 'You', cards: [approvalCard('approval', 'approval', stageOf('plan_approval'), approvalOf('plan'), 'after Plan check', now)] },
		{ id: 'build', label: roundLabel('Build', implement), cards: [
			agentCard('build', 'build', implement, detail.agentRuns, ['coder'], true, 'after your approval'),
			agentCard('code_check', 'code_check', implement, detail.agentRuns, ['code_reviewer'], false, 'after Build')
		] },
		{ id: 'test', label: 'Test', cards: [testCard, fixCard] },
		{ id: 'handoff', label: 'Hand-off', cards: handOff }
	];

	// The current step: the first waiting or working one, else the first not done.
	const all = groups.flatMap((group) => group.cards);
	const current = all.find((card) => card.status === 'waiting') ?? all.find((card) => card.status === 'working')
		?? (detail.run.state === 'running' ? all.find((card) => card.status === 'not_started' || card.status === 'up_next') : undefined);
	if (current) current.current = true;
	const activeGroup = current && (current.status === 'working') ? groups.findIndex((group) => group.cards.includes(current)) : -1;
	return { groups, current, activeGroup };
}

export function groupStatus(group: StepGroup): 'done' | 'active' | 'idle' {
	if (group.cards.every((card) => card.status === 'done' || card.status === 'skipped' || card.status === 'manual')) return 'done';
	if (group.cards.some((card) => card.status !== 'not_started')) return 'active';
	return 'idle';
}

/** The run's overall status, for the picker and the header badge. */
export function runStatus(run: HarnessRun): HxStatus {
	if (run.state === 'running') return run.currentStage?.state === 'awaiting_approval' ? 'waiting' : 'working';
	if (run.state === 'blocked') return 'stopped';
	if (run.state === 'passed') return 'done';
	if (run.state === 'failed') return 'failed';
	return 'stopped';
}

export function prLabel(url: string): string {
	const match = /\/([^/]+)\/pull(?:s|-requests)?\/(\d+)/.exec(url);
	return match ? `${match[1]}#${match[2]}` : url;
}
