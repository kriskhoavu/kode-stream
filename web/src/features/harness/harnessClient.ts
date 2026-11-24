import { api } from '../../shared/api';
import { openEventSourceFeed } from './useHarnessEvents';
import type {
	HarnessCreateTaskInput,
	HarnessRunFileDiff, HarnessRunPlan,
	HarnessAgentRun,
	HarnessApproval,
	HarnessCostSummary,
	HarnessEvent,
	HarnessRoleHealth,
	HarnessRoleId,
	HarnessRun,
	HarnessRunDetail,
	HarnessSettings,
	HarnessSourcePoll,
	HarnessStatus,
	HarnessTask, HarnessTranscript,
	SafeSessionRecord,
	SessionChanges
} from '../../lib/types';

export interface HarnessFeedHandlers {
	onOpen: () => void;
	onEvent: (event: HarnessEvent) => void;
	onReset: () => void;
	onError: () => void;
}

export interface HarnessWorkspaceOption {
	id: string;
	name: string;
	jiraProjectKey?: string;
}

export interface HarnessClient {
	status(signal?: AbortSignal): Promise<HarnessStatus>;
	tasks(signal?: AbortSignal): Promise<HarnessTask[]>;
	runs(signal?: AbortSignal): Promise<HarnessRun[]>;
	approvals(signal?: AbortSignal): Promise<HarnessApproval[]>;
	cost(groupBy: HarnessCostSummary['groupBy'], range?: { from?: string; to?: string }, signal?: AbortSignal): Promise<HarnessCostSummary>;
	run(runId: string, signal?: AbortSignal): Promise<HarnessRunDetail>;
	runChanges(runId: string, signal?: AbortSignal): Promise<SessionChanges>;
	runPlan?(runId: string, signal?: AbortSignal): Promise<HarnessRunPlan>;
	runFileDiff?(runId: string, path: string, signal?: AbortSignal): Promise<HarnessRunFileDiff>;
	agentRun(agentRunId: string, signal?: AbortSignal): Promise<{ agentRun: HarnessAgentRun; session?: SafeSessionRecord }>;
	agentRunTranscript(agentRunId: string, limit?: number, signal?: AbortSignal): Promise<HarnessTranscript>;
	approve(approval: HarnessApproval, idempotencyKey: string, reason?: string): Promise<HarnessApproval | undefined>;
	reject(approval: HarnessApproval, idempotencyKey: string, reason: string, decision?: 'reject' | 'replan'): Promise<HarnessApproval | undefined>;
	pin(task: HarnessTask, pinned: boolean): Promise<HarnessTask>;
	setPriority(task: HarnessTask, input: { priorityOverride?: number; before?: string }): Promise<HarnessTask>;
	setQuestions(task: HarnessTask, askQuestions: boolean | null): Promise<HarnessTask>;
	start(task: HarnessTask, idempotencyKey: string): Promise<HarnessTask>;
	createTask?(input: HarnessCreateTaskInput): Promise<HarnessTask>;
	removeTask?(task: HarnessTask): Promise<HarnessTask>;
	resetTask?(task: HarnessTask): Promise<HarnessTask>;
	pauseRole(role: HarnessRoleId): Promise<HarnessRoleHealth>;
	resumeRole(role: HarnessRoleId): Promise<HarnessRoleHealth>;
	settings(signal?: AbortSignal): Promise<HarnessSettings>;
	saveSettings(settings: HarnessSettings): Promise<HarnessSettings>;
	clearQueue(workspaceId?: string): Promise<{ removed: number }>;
	cancelRun(run: HarnessRun, reason?: string): Promise<HarnessRun>;
	workspaces?(signal?: AbortSignal): Promise<HarnessWorkspaceOption[]>;
	pollScouts?(workspaceId?: string): Promise<{ started: boolean; sources?: HarnessSourcePoll[] }>;
	subscribe?(afterSeq: number, handlers: HarnessFeedHandlers): () => void;
	readonly fixture?: boolean;
}

export const apiHarnessClient: HarnessClient = {
	status: (signal) => api.harnessStatus(signal),
	tasks: (signal) => api.harnessTasks(signal),
	runs: (signal) => api.harnessRuns(signal),
	approvals: (signal) => api.harnessApprovals(signal),
	cost: (groupBy, range, signal) => api.harnessCostSummary(groupBy, range, signal),
	run: (runId, signal) => api.harnessRun(runId, signal),
	runChanges: (runId, signal) => api.harnessRunChanges(runId, signal),
	runPlan: (runId, signal) => api.harnessRunPlan(runId, signal),
	runFileDiff: (runId, path, signal) => api.harnessRunFileDiff(runId, path, signal),
	agentRun: (agentRunId, signal) => api.harnessAgentRun(agentRunId, signal),
	agentRunTranscript: (agentRunId, limit, signal) => api.harnessAgentRunTranscript(agentRunId, limit, signal),
	approve: (approval, key, reason) => api.approveHarnessGate(approval, key, reason),
	reject: (approval, key, reason, decision) => api.rejectHarnessGate(approval, key, reason, decision),
	pin: (task, pinned) => api.pinHarnessTask(task, pinned),
	setPriority: (task, input) => api.setHarnessPriority(task, input),
	start: (task, idempotencyKey) => api.startHarnessTask(task, idempotencyKey),
	createTask: (input) => api.createHarnessTask(input),
	removeTask: (task) => api.removeHarnessTask(task),
	resetTask: (task) => api.resetHarnessTask(task),
	setQuestions: (task, askQuestions) => api.setHarnessTaskQuestions(task, askQuestions),
	pauseRole: (role) => api.pauseHarnessRole(role),
	resumeRole: (role) => api.resumeHarnessRole(role),
	settings: (signal) => api.harnessSettings(signal),
	saveSettings: (settings) => api.saveHarnessSettings(settings),
	clearQueue: (workspaceId) => api.clearHarnessQueue(workspaceId),
	cancelRun: (run, reason) => api.cancelHarnessRun(run.id, run.revision, reason),
	workspaces: async (signal) => (await api.workspaces(signal)).map((workspace) => ({ id: workspace.id, name: workspace.name, jiraProjectKey: workspace.jira?.projectKey || undefined })),
	pollScouts: (workspaceId) => api.pollHarnessScouts(workspaceId),
	subscribe: (afterSeq, handlers) => typeof EventSource === 'undefined'
		? (handlers.onError(), () => undefined)
		: openEventSourceFeed(api.harnessEventsURL(afterSeq || undefined), handlers)
};
