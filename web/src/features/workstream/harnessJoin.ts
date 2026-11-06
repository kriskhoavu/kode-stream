import type { HarnessApproval, HarnessRun, HarnessStageKind, HarnessTask, ItemSummary } from '../../lib/types';
import type { HarnessState } from '../harness/harnessState';
import { queueTasks, waitsFor } from '../harness/harnessState';
import type { HxStatus } from '../harness/vocabulary';
import { stageKindLabels, taskStatus } from '../harness/vocabulary';

/** Who works on a card: the harness, or a person by hand. */
export type WorkedBy = 'harness' | 'hand';

export const workedByLabels: Record<WorkedBy, string> = { harness: 'Harness', hand: 'By hand' };

/** What the harness holds for one card. */
export interface CardWork {
	task: HarnessTask;
	run?: HarnessRun;
	/** Approvals still waiting on a person, newest first. */
	waiting: HarnessApproval[];
	status: HxStatus;
	waitsFor: string[];
	/** 0 to 1 through the run's stages; absent before a run starts. */
	progress?: number;
	/** The stage chip's words: "Build · round 2 of 3", or the status word. */
	stageLabel: string;
	/** True for a card made from a task no item holds yet. */
	synthetic: boolean;
}

export interface HarnessJoin {
	/** Work by item id, including the synthetic cards. */
	byItemId: Map<string, CardWork>;
	/** Cards for tasks no item holds, in the Unsorted column, marked "no plan yet". */
	synthetic: ItemSummary[];
}

export const syntheticItemPrefix = 'harness:';
export const syntheticMetadataSource = 'harness';

const stageOrder: readonly HarnessStageKind[] = ['plan', 'plan_approval', 'implement', 'qa', 'pr_approval', 'draft_pr'];

/** The key the Jira key joins on. An item's identifier is its ticket: the Jira panel reads the issue by it. */
export function itemJoinKey(item: Pick<ItemSummary, 'identifier'>): string {
	return item.identifier.trim().toUpperCase();
}

export function taskJoinKey(task: Pick<HarnessTask, 'source'>): string {
	return task.source.key.trim().toUpperCase();
}

export function isSyntheticItem(item: Pick<ItemSummary, 'id'>): boolean {
	return item.id.startsWith(syntheticItemPrefix);
}

/** Joins the harness's tasks to the board's items by Jira key; a task without an item becomes a card of its own. */
export function joinHarness(items: ItemSummary[], state: HarnessState, workspaceId?: string): HarnessJoin {
	const byItemId = new Map<string, CardWork>();
	const synthetic: ItemSummary[] = [];
	if (!state.loaded) return { byItemId, synthetic };
	const itemsByKey = new Map<string, ItemSummary>();
	for (const item of items) {
		const key = itemJoinKey(item);
		if (key && !itemsByKey.has(key)) itemsByKey.set(key, item);
	}
	for (const task of queueTasks(state.tasks)) {
		if (workspaceId && task.workspaceId !== workspaceId) continue;
		const item = itemsByKey.get(taskJoinKey(task));
		if (item) {
			if (!byItemId.has(item.id)) byItemId.set(item.id, cardWork(task, state, false));
			continue;
		}
		const card = syntheticItem(task);
		synthetic.push(card);
		byItemId.set(card.id, cardWork(task, state, true));
	}
	return { byItemId, synthetic };
}

export function workedBy(work: CardWork | undefined): WorkedBy {
	return work ? 'harness' : 'hand';
}

export function cardWork(task: HarnessTask, state: HarnessState, synthetic: boolean): CardWork {
	const run = task.runId ? state.runs[task.runId] : undefined;
	const waiting = Object.values(state.approvals)
		.filter((approval) => approval.taskId === task.id && approval.state === 'requested')
		.sort((a, b) => Date.parse(b.requestedAt) - Date.parse(a.requestedAt));
	const waits = waitsFor(task, state.tasks);
	const status = taskStatus(task, waits);
	return { task, run, waiting, status, waitsFor: waits, progress: runProgress(run), stageLabel: stageLabel(status, run), synthetic };
}

/** How far a run is through its six stages: a stage under way counts half. */
export function runProgress(run?: HarnessRun): number | undefined {
	if (!run) return undefined;
	if (run.state === 'passed') return 1;
	const stage = run.currentStage;
	if (!stage) return 0;
	const index = stageOrder.indexOf(stage.kind);
	if (index < 0) return 0;
	const done = stage.state === 'passed' || stage.state === 'skipped' ? 1 : 0.5;
	return Math.min(1, (index + done) / stageOrder.length);
}

function stageLabel(status: HxStatus, run?: HarnessRun): string {
	const stage = run?.currentStage;
	if (!stage || status === 'done' || status === 'manual' || status === 'skipped') return '';
	const roundWord = stage.kind === 'qa' ? 'run' : 'round';
	const round = stage.maxRounds > 1 && (status === 'working' || status === 'needs_fix') ? ` · ${roundWord} ${stage.round} of ${stage.maxRounds}` : '';
	return `${stageKindLabels[stage.kind]}${round}`;
}

export function syntheticItem(task: HarnessTask): ItemSummary {
	return {
		id: `${syntheticItemPrefix}${task.id}`,
		workspaceId: task.workspaceId,
		workspaceName: '',
		branch: '',
		editable: false,
		scope: 'Unsorted',
		identifier: task.source.key,
		title: task.title,
		status: 'unsorted',
		owner: task.source.assignee,
		tags: [],
		updatedAt: task.updatedAt ?? task.source.updatedAt,
		metadataSource: syntheticMetadataSource,
		itemPath: ''
	};
}

/** The harness's work on one item, by its Jira key; nothing while the harness is off or holds no task for it. */
export function workForItem(item: Pick<ItemSummary, 'identifier' | 'workspaceId'> | null | undefined, state: HarnessState): CardWork | undefined {
	if (!item || !state.loaded) return undefined;
	const key = itemJoinKey(item);
	if (!key) return undefined;
	const task = queueTasks(state.tasks).find((candidate) => candidate.workspaceId === item.workspaceId && taskJoinKey(candidate) === key);
	return task ? cardWork(task, state, false) : undefined;
}
