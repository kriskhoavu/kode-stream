import { useCallback, useEffect, useRef, useState } from 'react';
import type { GitActivityEntry, GitStatus } from '../../lib/types';
import { api } from '../../shared/api';
import { notifyReliabilityChanged } from '../reliability/hooks';

export type GitOperation = 'fetch' | 'pull' | 'push';
export type GitBusy = GitOperation | 'commit' | 'branch' | '';
export type GitProblem = { caught: unknown; fallback: string } | { result: { message?: string; recoveryHint?: string } };

export function useItemGit({ workspaceId, activityPath, onChanged, onProblem }: {
	workspaceId?: string;
	activityPath: string;
	onChanged: (operation: 'pull' | 'commit' | 'branch') => Promise<void> | void;
	onProblem: (problem: GitProblem) => void;
}) {
	const [status, setStatus] = useState<GitStatus | null>(null);
	const [statusLoading, setStatusLoading] = useState(false);
	const [activity, setActivity] = useState<GitActivityEntry[]>([]);
	const [activityLoading, setActivityLoading] = useState(false);
	const [message, setMessage] = useState('');
	const [selectedPaths, setSelectedPaths] = useState<string[]>([]);
	const [branchName, setBranchName] = useState('');
	const [busy, setBusy] = useState<GitBusy>('');
	const generation = useRef(0);
	const statusAbort = useRef<AbortController | undefined>(undefined);
	const activityAbort = useRef<AbortController | undefined>(undefined);
	const latest = useRef({ onChanged, onProblem });
	latest.current = { onChanged, onProblem };

	useEffect(() => {
		generation.current += 1;
		return () => {
			generation.current += 1;
			statusAbort.current?.abort();
			activityAbort.current?.abort();
		};
	}, [workspaceId]);

	const loadStatus = useCallback(async () => {
		statusAbort.current?.abort();
		if (!workspaceId) { setStatus(null); return; }
		const controller = new AbortController();
		statusAbort.current = controller;
		setStatusLoading(true);
		try {
			const next = await api.gitStatus(workspaceId, controller.signal);
			if (!controller.signal.aborted) setStatus(next);
		} catch {
			if (!controller.signal.aborted) setStatus(null);
		} finally {
			if (!controller.signal.aborted) setStatusLoading(false);
		}
	}, [workspaceId]);

	const loadActivity = useCallback(async () => {
		activityAbort.current?.abort();
		if (!workspaceId) { setActivity([]); setActivityLoading(false); return; }
		const controller = new AbortController();
		activityAbort.current = controller;
		setActivityLoading(true);
		try {
			const entries = await api.gitActivity(workspaceId, { path: activityPath || undefined, limit: 8, signal: controller.signal });
			if (!controller.signal.aborted) setActivity(entries);
		} catch {
			if (!controller.signal.aborted) setActivity([]);
		} finally {
			if (!controller.signal.aborted) setActivityLoading(false);
		}
	}, [workspaceId, activityPath]);

	useEffect(() => { void loadStatus(); }, [loadStatus]);
	useEffect(() => { void loadActivity(); }, [loadActivity]);

	const act = async (kind: Exclude<GitBusy, ''>, fallback: string, call: (id: string) => Promise<{ ok: boolean; status: GitStatus; message?: string; recoveryHint?: string }>, after?: () => void) => {
		if (!workspaceId) return;
		const started = generation.current;
		const current = () => generation.current === started;
		setBusy(kind);
		try {
			const result = await call(workspaceId);
			notifyReliabilityChanged();
			if (!current()) return;
			setStatus(result.status);
			after?.();
			void loadActivity();
			if (kind === 'pull' || kind === 'commit' || kind === 'branch') await latest.current.onChanged(kind);
			if (current() && !result.ok && result.message) latest.current.onProblem({ result });
		} catch (caught) {
			if (current()) latest.current.onProblem({ caught, fallback });
		} finally {
			if (current()) setBusy('');
		}
	};

	const run = (operation: GitOperation) => act(operation, `${operation} failed`, (id) => (
		operation === 'fetch' ? api.gitFetch(id) : operation === 'pull' ? api.gitPull(id, {}) : api.gitPush(id)
	));
	const commit = () => act('commit', 'Commit failed', (id) => api.gitCommit(id, { message, paths: selectedPaths }), () => { setMessage(''); setSelectedPaths([]); });
	const createBranch = () => branchName.trim()
		? act('branch', 'Branch operation failed', (id) => api.createBranch(id, { name: branchName.trim(), checkout: true }), () => setBranchName(''))
		: Promise.resolve();
	const togglePath = (path: string) => setSelectedPaths((current) => current.includes(path) ? current.filter((item) => item !== path) : [...current, path]);

	return { status, setStatus, statusLoading, loadStatus, activity, activityLoading, loadActivity, activityPath, message, setMessage, selectedPaths, togglePath, branchName, setBranchName, busy, run, commit, createBranch };
}

export type ItemGit = ReturnType<typeof useItemGit>;
