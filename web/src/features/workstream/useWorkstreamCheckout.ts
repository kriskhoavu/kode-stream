import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { ItemSummary, WorkspaceConfig, WorkstreamBranchLoadResult } from '../../lib/types';
import { useWorkspaceGuard } from './useWorkspaceGuard';

// A just-created item stays until a load reports it, so a lagging rescan cannot make it vanish.
export function useWorkstreamCheckout(workspace: WorkspaceConfig | undefined, refreshKey: unknown) {
	const [items, setItems] = useState<ItemSummary[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState('');
	const [selectedBranch, setSelectedBranch] = useState('');
	const [branchContext, setBranchContext] = useState<WorkstreamBranchLoadResult | null>(null);
	const pendingCreatedRef = useRef<ItemSummary | null>(null);
	const loadedWorkspaceRef = useRef<string | undefined>(undefined);
	const abortRef = useRef<AbortController | undefined>(undefined);
	const track = useWorkspaceGuard(workspace?.id);
	useEffect(() => () => abortRef.current?.abort(), [workspace?.id]);

	const withPendingCreated = (loaded: ItemSummary[]) => {
		const pending = pendingCreatedRef.current;
		if (!pending) return loaded;
		if (loaded.some((candidate) => candidate.id === pending.id)) {
			pendingCreatedRef.current = null;
			return loaded;
		}
		return [pending, ...loaded];
	};

	const begin = () => {
		abortRef.current?.abort();
		const controller = new AbortController();
		abortRef.current = controller;
		const active = track(workspace?.id);
		const finish = () => { if (abortRef.current === controller) abortRef.current = undefined; };
		return { controller, active, finish };
	};

	const apply = (result: WorkstreamBranchLoadResult) => {
		setBranchContext(result);
		setSelectedBranch(result.branch);
		setItems(withPendingCreated(result.items));
	};

	const loadCheckout = async (force = false, { silent = false }: { silent?: boolean } = {}) => {
		if (!workspace) return;
		const { controller, active, finish } = begin();
		if (!silent) setLoading(true);
		setError('');
		try {
			const result = await api.loadWorkstreamCheckout(workspace.id, { force }, controller.signal);
			if (controller.signal.aborted || !active()) return;
			apply(result);
		} catch (err) {
			if (controller.signal.aborted) return;
			try {
				const fallbackItems = await api.items(new URLSearchParams({ workspaceId: workspace.id }), controller.signal);
				if (controller.signal.aborted || !active()) return;
				setBranchContext(null);
				setSelectedBranch(workspace.baselineBranch);
				setItems(fallbackItems);
				setError('');
			} catch {
				if (controller.signal.aborted || !active()) return;
				setError(err instanceof Error ? err.message : 'Failed to load checkout');
				setItems([]);
			}
		} finally {
			if (!controller.signal.aborted && active()) setLoading(false);
			finish();
		}
	};

	const rememberCreated = (item: ItemSummary) => {
		pendingCreatedRef.current = item;
		setItems((current) => current.some((candidate) => candidate.id === item.id)
			? current.map((candidate) => (candidate.id === item.id ? { ...candidate, ...item } : candidate))
			: [item, ...current]);
	};

	useEffect(() => {
		if (!workspace) {
			setItems([]);
			setBranchContext(null);
			setSelectedBranch('');
			setLoading(false);
			return;
		}
		// A refreshKey bump reloads the same workspace silently so the cards stay.
		const sameWorkspace = loadedWorkspaceRef.current === workspace.id;
		loadedWorkspaceRef.current = workspace.id;
		void loadCheckout(false, { silent: sameWorkspace });
	}, [workspace?.id, refreshKey]);

	return { items, setItems, loading, error, setError, selectedBranch, branchContext, track, begin, apply, loadCheckout, rememberCreated };
}
