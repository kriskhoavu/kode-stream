import { useState } from 'react';
import { api } from '../../shared/api';
import type { JiraConnection, KnowledgeSettings, SourceSurface, WorkspaceConfig, WorkspaceRuntimeConfig } from '../../lib/types';
import { sourcePaths } from '../../lib/sources';
import { parseSources } from './sourceSettings';
import { cloneRuntimeConfig, errorMessage, normalizeKnowledgeSettings, shownInByPath, sourcesFromDraft } from './workspaceDraft';
import type { WorkspaceEditSection } from './workspaceDetailTabs';
import type { WorkspaceNotice } from './workspaceNotice';

export interface WorkspaceEditDraft {
	name: string;
	path: string;
	baselineBranch: string;
	sources: string;
	shownIn: Record<string, SourceSurface[]>;
	jira: JiraConnection | null;
	knowledge: KnowledgeSettings;
	runtime: WorkspaceRuntimeConfig | null;
}

const emptyDraft: WorkspaceEditDraft = { name: '', path: '', baselineBranch: '', sources: '', shownIn: {}, jira: null, knowledge: {}, runtime: null };

function draftFromWorkspace(repo: WorkspaceConfig): WorkspaceEditDraft {
	return {
		name: repo.name,
		path: repo.path,
		baselineBranch: repo.baselineBranch,
		sources: sourcePaths(repo).join(', '),
		shownIn: shownInByPath(repo),
		jira: repo.jira ? { ...repo.jira } : null,
		knowledge: { enabled: repo.knowledge?.enabled ?? true, enrichExecutable: repo.knowledge?.enrichExecutable ?? '', enrichArgs: [...(repo.knowledge?.enrichArgs ?? [])] },
		runtime: cloneRuntimeConfig(repo.runtime)
	};
}

/**
 * Editing one section of one workspace: which workspace and section are open, the draft, and saving
 * it. The runtime section saves through its own endpoint; every other section updates the workspace.
 */
export function useWorkspaceEdit({ setBusy, onNotice, onChanged }: {
	setBusy: (pending: boolean) => void;
	onNotice: (notice: WorkspaceNotice | null) => void;
	onChanged: () => void | Promise<void>;
}) {
	const [editingId, setEditingId] = useState('');
	const [editingSection, setEditingSection] = useState<WorkspaceEditSection>('');
	const [editDraft, setEditDraft] = useState<WorkspaceEditDraft>(emptyDraft);

	const startEdit = (repo: WorkspaceConfig, section: Exclude<WorkspaceEditSection, ''>) => {
		setEditingId(repo.id);
		setEditingSection(section);
		setEditDraft(draftFromWorkspace(repo));
		onNotice(null);
	};

	const discardEdit = () => {
		setEditingId('');
		setEditingSection('');
	};

	const confirmDiscardEdit = () => !editingId || window.confirm('Discard unsaved workspace changes?');

	const saveEdit = async (repo: WorkspaceConfig) => {
		setBusy(true);
		onNotice(null);
		try {
			if (editingSection === 'runtime') {
				await api.saveWorkspaceRuntime(repo.id, editDraft.runtime);
			} else {
				await api.updateWorkspace(repo.id, {
					name: editDraft.name,
					path: editDraft.path,
					baselineBranch: editDraft.baselineBranch,
					sources: sourcesFromDraft(parseSources(editDraft.sources), editDraft.shownIn),
					registrationMode: repo.registrationMode,
					remoteUrl: repo.remoteUrl,
					jira: editDraft.jira ?? undefined,
					knowledge: normalizeKnowledgeSettings(editDraft.knowledge)
				});
			}
			discardEdit();
			onNotice({ tone: 'success', title: 'Workspace updated', details: [editDraft.name || repo.name] });
			void onChanged();
		} catch (err) {
			onNotice({ tone: 'error', title: `Update failed for ${repo.name}`, details: [errorMessage(err)] });
		} finally {
			setBusy(false);
		}
	};

	return { editingId, editingSection, editDraft, setEditDraft, startEdit, discardEdit, confirmDiscardEdit, saveEdit };
}
