import type { HarnessAgentRole, HarnessRoleModel, HarnessSettings as Settings, HarnessWorkspaceSettings } from '../../lib/types';

export function emptyWorkspaceSettings(workspaceId: string): HarnessWorkspaceSettings {
	return { workspaceId, verificationCommands: [], scope: [], models: {}, procedure: {} };
}

export function ownSettings(draft: Settings, workspaceId: string | undefined): HarnessWorkspaceSettings {
	return draft.workspaces.find((workspace) => workspace.workspaceId === workspaceId) ?? emptyWorkspaceSettings(workspaceId ?? '');
}

export function withOwnSettings(draft: Settings, own: HarnessWorkspaceSettings, patch: Partial<HarnessWorkspaceSettings>): HarnessWorkspaceSettings[] {
	const next = { ...own, ...patch };
	return draft.workspaces.some((workspace) => workspace.workspaceId === own.workspaceId)
		? draft.workspaces.map((workspace) => workspace.workspaceId === own.workspaceId ? next : workspace)
		: [...draft.workspaces, next];
}

export function effectiveRoleModel(draft: Settings, own: HarnessWorkspaceSettings, role: HarnessAgentRole): Required<HarnessRoleModel> {
	const fallback = draft.modelPolicy[role];
	const mine = own.models[role];
	return {
		engine: mine?.engine || fallback.engine,
		model: mine?.model || fallback.model,
		highDifficultyModel: mine ? mine.highDifficultyModel ?? '' : fallback.hardModel ?? '',
		maxTurns: mine?.maxTurns || fallback.maxTurns
	};
}

export function withRoleSkill(procedure: HarnessWorkspaceSettings['procedure'], role: HarnessAgentRole, patch: Partial<{ skill: string; track: string }>): HarnessWorkspaceSettings['procedure'] {
	const next = { ...(procedure[role] ?? { skill: '' }), ...patch };
	const result = { ...procedure };
	if (next.skill || next.track) result[role] = { skill: next.skill, ...(next.track ? { track: next.track } : {}) };
	else delete result[role];
	return result;
}
