import { api } from '../../shared/api';
import { openEventSourceFeed } from './useHarnessEvents';
import type {
	HarnessAgentRun,
	HarnessApproval,
	HarnessCostSummary,
	HarnessEvent,
	HarnessRoleHealth,
	HarnessRoleId,
	HarnessRun,
	HarnessRunDetail,
	HarnessSettings,
	HarnessStatus,
	HarnessTask,
	SafeSessionRecord,
	SessionChanges
} from '../../lib/types';

export interface HarnessFeedHandlers {
	onOpen: () => void;
	onEvent: (event: HarnessEvent) => void;
	/** The server dropped this subscriber or the connection failed; refetch a snapshot. */
	onReset: () => void;
	onError: () => void;
}

/**
 * Everything the harness screens ask of the server. The real client wraps the
 * shared API; the dev fixture implements the same surface in memory, so every
 * view works in `npm run dev` without a backend.
 */
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
	agentRun(agentRunId: string, signal?: AbortSignal): Promise<{ agentRun: HarnessAgentRun; session?: SafeSessionRecord }>;
	approve(approval: HarnessApproval, idempotencyKey: string, reason?: string): Promise<HarnessApproval | undefined>;
	reject(approval: HarnessApproval, idempotencyKey: string, reason: string): Promise<HarnessApproval | undefined>;
	pin(task: HarnessTask, pinned: boolean): Promise<HarnessTask>;
	setPriority(task: HarnessTask, input: { priorityOverride?: number; before?: string }): Promise<HarnessTask>;
	/** Asks the harness to start a task now; the next free Planner takes it. */
	start(task: HarnessTask, idempotencyKey: string): Promise<HarnessTask>;
	pauseRole(role: HarnessRoleId): Promise<HarnessRoleHealth>;
	resumeRole(role: HarnessRoleId): Promise<HarnessRoleHealth>;
	settings(signal?: AbortSignal): Promise<HarnessSettings>;
	saveSettings(settings: HarnessSettings): Promise<HarnessSettings>;
	/** The workspaces a source can look in, with the Jira project key when the workspace has a Jira connection. */
	workspaces?(signal?: AbortSignal): Promise<HarnessWorkspaceOption[]>;
	/** Opens the live event feed after `afterSeq`; returns a function that closes it. */
	subscribe?(afterSeq: number, handlers: HarnessFeedHandlers): () => void;
	/** True for the in-memory fixture, so the page can say so. */
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
	agentRun: (agentRunId, signal) => api.harnessAgentRun(agentRunId, signal),
	approve: (approval, key, reason) => api.approveHarnessGate(approval, key, reason),
	reject: (approval, key, reason) => api.rejectHarnessGate(approval, key, reason),
	pin: (task, pinned) => api.pinHarnessTask(task, pinned),
	setPriority: (task, input) => api.setHarnessPriority(task, input),
	start: (task, idempotencyKey) => api.startHarnessTask(task, idempotencyKey),
	pauseRole: (role) => api.pauseHarnessRole(role),
	resumeRole: (role) => api.resumeHarnessRole(role),
	settings: (signal) => api.harnessSettings(signal),
	saveSettings: (settings) => api.saveHarnessSettings(settings),
	workspaces: async (signal) => (await api.workspaces(signal)).map((workspace) => ({ id: workspace.id, name: workspace.name, jiraProjectKey: workspace.jira?.projectKey || undefined })),
	subscribe: (afterSeq, handlers) => typeof EventSource === 'undefined'
		? (handlers.onError(), () => undefined)
		: openEventSourceFeed(api.harnessEventsURL(afterSeq || undefined), handlers)
};
