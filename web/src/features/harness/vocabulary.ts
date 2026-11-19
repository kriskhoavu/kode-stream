import { Check, CircleDashed, CircleDot, CircleX, Hand, LoaderCircle, OctagonX, SkipForward, UserCheck, Wrench } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type {
	HarnessAgentRole,
	HarnessAgentRunState,
	HarnessApprovalGate,
	HarnessChannel,
	HarnessEngineId,
	HarnessRoleId,
	HarnessSourceKind,
	HarnessStageKind,
	HarnessStageState,
	HarnessTask
} from '../../lib/types';

/**
 * The single source of every word the harness shows. The screens are read by
 * people who never see the backend, so no state name, engine id or model id
 * reaches them except through this module (raw ids go in tooltips only).
 */

export type HxStatus = 'not_started' | 'up_next' | 'starting' | 'working' | 'waiting' | 'done' | 'needs_fix' | 'failed' | 'stopped' | 'skipped' | 'manual';

/** Colour family of a status; each maps to one --hx-* token. */
export type HxTone = 'idle' | 'running' | 'waiting' | 'done' | 'stopped' | 'manual';

export const hxStatusOrder: readonly HxStatus[] = ['waiting', 'working', 'needs_fix', 'up_next', 'not_started', 'stopped', 'failed', 'done', 'manual', 'skipped'];

export const hxStatuses: Record<HxStatus, { label: string; tone: HxTone; icon: LucideIcon }> = {
	not_started: { label: 'Not started', tone: 'idle', icon: CircleDashed },
	up_next: { label: 'Up next', tone: 'done', icon: CircleDot },
	starting: { label: 'Starting', tone: 'running', icon: LoaderCircle },
	working: { label: 'Working', tone: 'running', icon: LoaderCircle },
	waiting: { label: 'Waiting on you', tone: 'waiting', icon: Hand },
	done: { label: 'Done', tone: 'done', icon: Check },
	needs_fix: { label: 'Needs a fix', tone: 'waiting', icon: Wrench },
	failed: { label: "Didn't pass", tone: 'stopped', icon: CircleX },
	stopped: { label: 'Stopped', tone: 'stopped', icon: OctagonX },
	skipped: { label: 'Skipped', tone: 'idle', icon: SkipForward },
	manual: { label: 'Done by hand', tone: 'manual', icon: UserCheck }
};

/** Stage words. Plan check, Code check and Fix are the reviewer and fixer halves of their stage. */
export type HxStep = 'start' | 'plan' | 'plan_check' | 'approval' | 'build' | 'code_check' | 'test' | 'fix' | 'review' | 'draft_pr';

export const hxSteps: Record<HxStep, { label: string; explain: string }> = {
	start: { label: 'Start', explain: 'A person said this task may begin. Nothing runs before that.' },
	plan: { label: 'Plan', explain: 'The Planner reads the task and writes a step-by-step plan. If the Checker finds blockers, it tries again, up to 3 rounds.' },
	plan_check: { label: 'Plan check', explain: 'A second agent reads the plan and lists anything that would block it.' },
	approval: { label: 'Your approval', explain: 'A person approves the plan before any code is written. From the dashboard or from Webex, the result shows here.' },
	build: { label: 'Build', explain: 'The Builder writes the code on its own branch, following the plan. It cannot push or merge.' },
	code_check: { label: 'Code check', explain: 'The Checker reads the change and lists must-fix items. The Builder fixes them, up to 3 rounds.' },
	test: { label: 'Test', explain: "The project's own tests run twice. Both runs must pass, so a flaky result is caught." },
	fix: { label: 'Fix', explain: 'If a test fails, the Builder gets the failure and fixes it, up to 2 rounds.' },
	review: { label: 'Your review', explain: 'You try the change in a session of your own, in the run\'s worktree. Then you hand it off, or send it back to Build or Plan with what to change.' },
	draft_pr: { label: 'Draft pull request', explain: 'The branch is pushed and a draft pull request opens for a person to review. Nothing merges on its own.' }
};

export const stageKindLabels: Record<HarnessStageKind, string> = {
	plan: 'Plan',
	plan_approval: 'Your approval',
	implement: 'Build',
	qa: 'Test',
	review: 'Your review',
	draft_pr: 'Draft pull request'
};

/** Where a run ended, after its status: "Stopped while planning". */
export const stageKindWhere: Record<HarnessStageKind, string> = {
	plan: 'while planning',
	plan_approval: 'at your plan approval',
	implement: 'while building',
	qa: 'while testing',
	review: 'at your review',
	draft_pr: 'while opening the draft pull request'
};

/** The verb shown on a working row: "Planning · round 1 of 3". */
export const stageKindDoing: Record<HarnessStageKind, string> = {
	plan: 'Planning',
	plan_approval: 'Waiting for your approval',
	implement: 'Building',
	qa: 'Testing',
	review: 'Waiting for your review',
	draft_pr: 'Opening the draft pull request'
};

export const roleLabels: Record<HarnessRoleId, string> = {
	scout: 'Finder',
	sorter: 'Sorter',
	planner: 'Planner',
	coder: 'Builder',
	reviewer: 'Checker',
	qa: 'Tester'
};

export const roleOrder: readonly HarnessRoleId[] = ['scout', 'sorter', 'planner', 'coder', 'reviewer', 'qa'];

export const agentRoleLabels: Record<HarnessAgentRole, string> = {
	planner: 'Planner',
	plan_reviewer: 'Checker',
	coder: 'Builder',
	code_reviewer: 'Checker',
	qa_fixer: 'Builder'
};

export const agentRoleToRole: Record<HarnessAgentRole, HarnessRoleId> = {
	planner: 'planner',
	plan_reviewer: 'reviewer',
	coder: 'coder',
	code_reviewer: 'reviewer',
	qa_fixer: 'coder'
};

export const gateTitles: Record<HarnessApprovalGate, string> = {
	start: 'start work on',
	plan: 'approve the plan for',
	resume: 'decide how to continue',
	review: 'review',
	question: 'answer a question on'
};

export const sourceLabels: Record<HarnessSourceKind, string> = { jira: 'Jira', github: 'GitHub', bitbucket: 'Bitbucket' };

export const channelLabels: Record<HarnessChannel, string> = { dashboard: 'here', webex: 'via Webex', api: 'via the API' };

export const findingLabels = { blocking: 'Must fix', advisory: 'Suggestion' } as const;

const engineLabels: Record<HarnessEngineId, string> = { claudecode: 'Claude', codex: 'Codex', opencode: 'OpenCode' };

export function engineLabel(engine?: HarnessEngineId): string {
	return engine ? engineLabels[engine] ?? engine : 'No model';
}

/** "opus" → "Opus"; "claude-opus-5-5" → "Opus 5.5"; anything else is shown as given. */
export function modelLabel(model?: string): string {
	if (!model) return '';
	const concrete = /^claude-([a-z]+)-(\d+)(?:-(\d+))?/.exec(model);
	if (concrete) return `${capitalize(concrete[1])} ${concrete[2]}${concrete[3] ? `.${concrete[3]}` : ''}`;
	if (/^[a-z]+$/.test(model)) return capitalize(model);
	return model;
}

/** The chip text for who is working: "Claude · Opus 5.5". */
export function workerLabel(engine?: HarnessEngineId, model?: string): string {
	if (!engine) return 'No model';
	const name = modelLabel(model);
	return name ? `${engineLabel(engine)} · ${name}` : engineLabel(engine);
}

export function stageStatus(state: HarnessStageState): HxStatus {
	switch (state) {
		case 'running': return 'working';
		case 'awaiting_approval': return 'waiting';
		case 'passed': return 'done';
		case 'failed': return 'failed';
		case 'retry_wait': return 'needs_fix';
		case 'cancelled': return 'stopped';
		case 'skipped': return 'skipped';
		default: return 'not_started';
	}
}

export function agentRunStatus(state: HarnessAgentRunState, verdict?: 'approve' | 'changes_requested'): HxStatus {
	switch (state) {
		case 'running': return 'working';
		case 'queued': return 'up_next';
		case 'succeeded': return verdict === 'changes_requested' ? 'needs_fix' : 'done';
		case 'failed': return 'failed';
		case 'cancelled': return 'stopped';
		case 'interrupted': return 'needs_fix';
		default: return 'not_started';
	}
}

/** A task's plain status. `waitsFor` lists dependencies not yet done; a ready task with none is "Up next". */
export function taskStatus(task: HarnessTask, waitsFor: readonly string[] = []): HxStatus {
	if (task.finishedOutside) return 'manual';
	if (task.needsYou) return 'waiting';
	switch (task.state) {
		case 'running': return 'working';
		case 'blocked': return 'stopped';
		case 'cancelled': return 'stopped';
		case 'done': return 'done';
		case 'dismissed':
		case 'stale': return 'skipped';
		// Started by a person, not yet taken by a Planner.
		case 'ranked': return task.startRequested ? 'starting' : waitsFor.length ? 'not_started' : 'up_next';
		default: return 'not_started';
	}
}

export function formatCost(usd: number | undefined): string {
	if (usd === undefined || !Number.isFinite(usd)) return '—';
	return `$${usd.toFixed(2)}`;
}

export function formatMinutes(ms: number | undefined): string {
	if (!ms || ms < 0) return '0 min';
	const minutes = Math.round(ms / 60000);
	if (minutes < 1) return 'under 1 min';
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	const rest = minutes % 60;
	return rest ? `${hours} h ${rest} min` : `${hours} h`;
}

/** A running clock: "7:12", or "1:02:03" past an hour. */
export function formatClock(ms: number): string {
	const total = Math.max(0, Math.floor(ms / 1000));
	const hours = Math.floor(total / 3600);
	const minutes = Math.floor((total % 3600) / 60);
	const seconds = total % 60;
	const pad = (value: number) => String(value).padStart(2, '0');
	return hours ? `${hours}:${pad(minutes)}:${pad(seconds)}` : `${minutes}:${pad(seconds)}`;
}

export function formatAgo(at: string | undefined, now: number): string {
	if (!at) return 'never';
	const minutes = Math.round((now - Date.parse(at)) / 60000);
	if (!Number.isFinite(minutes)) return 'never';
	if (minutes < 1) return 'just now';
	if (minutes < 60) return `${minutes} min ago`;
	const hours = Math.round(minutes / 60);
	if (hours < 24) return `${hours} h ago`;
	return `${Math.round(hours / 24)} d ago`;
}

export function formatTime(at: string | undefined): string {
	if (!at) return '';
	const date = new Date(at);
	return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

export function plural(count: number, one: string, many = `${one}s`): string {
	return `${count} ${count === 1 ? one : many}`;
}

function capitalize(value: string): string {
	return value.charAt(0).toUpperCase() + value.slice(1);
}
