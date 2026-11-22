import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { ItemSummary, WorkspaceConfig, WorkstreamBranchLoadResult } from '../../lib/types';
import { useWorkspaceGuard } from './useWorkspaceGuard';

/**
 * The board's checkout: the branch it shows and that branch's items, loaded for the workspace and
 * reloaded silently on each refreshKey bump. One load runs at a time; a newer one aborts it, and a
 * workspace change discards its answer. A just-created item stays on the board until a load reports
 * it, so a rescan that has not caught up cannot make the new card flash and vanish.
 */
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

	/** Starts a load for the current workspace, aborting the one in flight. */
	const begin = () => {
		abortRef.current?.abort();
		const controller = new AbortController();
		abortRef.current = controller;
		const active = track(workspace?.id);
		const finish = () => { if (abortRef.current === controller) abortRef.current = undefined; };
		return { controller, active, finish };
	};

	/** Shows a loaded checkout. */
	const apply = (result: WorkstreamBranchLoadResult) => {
		setBranchContext(result);
		setSelectedBranch(result.branch);
		setItems(withPendingCreated(result.items));
	};

	// silent keeps the board on screen while it refreshes, so a background reload
	// never blanks the columns the user is already looking at.
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

	/** Puts a just-created item on the board at once and keeps it there until a load reports it. */
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
		// A refreshKey bump is the app refreshing (on window focus, among others),
		// not a new board: reload the same workspace silently so the cards stay.
		const sameWorkspace = loadedWorkspaceRef.current === workspace.id;
		loadedWorkspaceRef.current = workspace.id;
		void loadCheckout(false, { silent: sameWorkspace });
	}, [workspace?.id, refreshKey]);

	return { items, setItems, loading, error, setError, selectedBranch, branchContext, track, begin, apply, loadCheckout, rememberCreated };
}
