import { useEffect, useState } from 'react';
import { api } from '../../shared/api';
import type { GitStatus } from '../../lib/types';

/** A workspace's Git status and its local branches, reloaded when the workspace changes. */
export function useWorkspaceGitInfo(workspaceId?: string) {
	const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
	const [currentBranch, setCurrentBranch] = useState('');
	const [branches, setBranches] = useState<string[]>([]);
	useEffect(() => {
		const clearBranches = () => {
			setCurrentBranch('');
			setBranches([]);
		};
		if (!workspaceId) {
			setGitStatus(null);
			clearBranches();
			return;
		}
		const controller = new AbortController();
		api.gitStatus(workspaceId, controller.signal)
			.then((status) => { if (!controller.signal.aborted) setGitStatus(status); })
			.catch(() => { if (!controller.signal.aborted) setGitStatus(null); });
		api.workspaceBranches(workspaceId, controller.signal)
			.then((response) => {
				if (controller.signal.aborted) return;
				setCurrentBranch(response.current);
				setBranches(response.branches);
			})
			.catch(() => { if (!controller.signal.aborted) clearBranches(); });
		return () => controller.abort();
	}, [workspaceId]);
	return { gitStatus, currentBranch, branches };
}
