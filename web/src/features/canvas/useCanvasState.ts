import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from '../../lib/api';
import type { CanvasNode, CanvasPlacementPatch, CanvasPosition, CanvasProjection, CanvasViewport } from '../../lib/types';

interface DirtyPlacement {
	patch: CanvasPlacementPatch;
	mutation: number;
	retries: number;
}

export function useCanvasState(workspaceId?: string) {
	const [projection, setProjectionState] = useState<CanvasProjection>();
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
	const [conflicts, setConflicts] = useState<string[]>([]);
	const [dirtyVersion, setDirtyVersion] = useState(0);
	const projectionRef = useRef<CanvasProjection | undefined>(undefined);
	const dirtyRef = useRef(new Map<string, DirtyPlacement>());
	const mutationRef = useRef(0);
	const viewportTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	const automaticPlacementRef = useRef<Promise<CanvasProjection> | undefined>(undefined);

	const setProjection = useCallback((next: CanvasProjection | undefined) => {
		if (next) {
			next = overlayDirty(next, dirtyRef.current);
		}
		projectionRef.current = next;
		setProjectionState(next);
	}, []);

	const placeNewNodes = useCallback(async (projection: CanvasProjection) => {
		if (projection.unplaced.length === 0) return projection;
		const capability = projection.nodes.find((node) => node.workspace)?.workspace?.actions['layout.move'];
		if (capability && capability.state !== 'available') return projection;
		if (automaticPlacementRef.current) {
			await automaticPlacementRef.current;
			return api.canvasLayout(projection.layout.id);
		}
		const operation = (async () => {
			let current = projection;
			while (current.unplaced.length > 0) {
				const refs = current.unplaced.slice(0, 50);
				const patches = refs.map((entityRef, index) => ({ nodeId: nodeID(entityRef), entityRef, position: deterministicPosition(current.nodes.length + index, entityRef.kind), collapsed: entityRef.kind === 'session', expectedRevision: 0 }));
				current = await api.patchCanvasPlacements(current.layout.id, patches);
				const remaining = new Set(current.unplaced.map(nodeID));
				if (refs.some((ref) => remaining.has(nodeID(ref)))) throw new Error('Canvas could not place newly discovered nodes.');
			}
			return current;
		})();
		automaticPlacementRef.current = operation;
		try {
			return await operation;
		} finally {
			if (automaticPlacementRef.current === operation) automaticPlacementRef.current = undefined;
		}
	}, []);

	const load = useCallback(async () => {
		if (!workspaceId) {
			setProjection(undefined);
			return;
		}
		setLoading(true);
		setError('');
		try {
			const next = await placeNewNodes(await api.resolveDefaultCanvas(workspaceId));
			dirtyRef.current.clear();
			setConflicts([]);
			setProjection(next);
		} catch (caught) {
			setError(messageFrom(caught));
		} finally {
			setLoading(false);
		}
	}, [placeNewNodes, setProjection, workspaceId]);

	const refresh = useCallback(async () => {
		const current = projectionRef.current;
		if (!current) return;
		try {
			setProjection(await placeNewNodes(await api.canvasLayout(current.layout.id)));
			setError('');
		} catch (caught) {
			setError(messageFrom(caught));
		}
	}, [placeNewNodes, setProjection]);

	useEffect(() => {
		void load();
		return () => {
			if (viewportTimer.current) clearTimeout(viewportTimer.current);
		};
	}, [load]);

	useEffect(() => {
		if (!projection?.layout.id) return;
		const onFocus = () => void refresh();
		const onVisibility = () => { if (document.visibilityState === 'visible') void refresh(); };
		window.addEventListener('focus', onFocus);
		document.addEventListener('visibilitychange', onVisibility);
		return () => { window.removeEventListener('focus', onFocus); document.removeEventListener('visibilitychange', onVisibility); };
	}, [projection?.layout.id, refresh]);

	const flushPlacements = useCallback(async () => {
		const current = projectionRef.current;
		if (!current || dirtyRef.current.size === 0) return;
		const batch = Array.from(dirtyRef.current.values()).slice(0, 50);
		try {
			const saved = await api.patchCanvasPlacements(current.layout.id, batch.map((entry) => entry.patch));
			for (const entry of batch) {
				if (dirtyRef.current.get(entry.patch.nodeId)?.mutation === entry.mutation) dirtyRef.current.delete(entry.patch.nodeId);
			}
			setConflicts((previous) => previous.filter((id) => dirtyRef.current.has(id)));
			setError('');
			setSaveStatus(dirtyRef.current.size > 0 ? 'saving' : 'saved');
			setProjection(saved);
			if (dirtyRef.current.size > 0) setDirtyVersion((version) => version + 1);
		} catch (caught) {
			setSaveStatus('saving');
			if (caught instanceof ApiError && caught.code === 'placement_conflict') {
				const affected = caught.nodeIds?.length ? caught.nodeIds : batch.map((entry) => entry.patch.nodeId);
				setConflicts((previous) => Array.from(new Set([...previous, ...affected])));
				setError('A node position changed elsewhere. Reload its position or reapply your move.');
				return;
			}
			let retry = false;
			for (const entry of batch) {
				const currentEntry = dirtyRef.current.get(entry.patch.nodeId);
				if (!currentEntry || currentEntry.mutation !== entry.mutation || currentEntry.retries >= 2) continue;
				currentEntry.retries += 1;
				retry = true;
			}
			setError(retry ? 'Could not save positions yet. Retrying…' : messageFrom(caught));
			if (retry) setTimeout(() => setDirtyVersion((version) => version + 1), 300 * (batch[0]?.retries + 1));
		}
	}, [setProjection]);

	useEffect(() => {
		if (!projection?.layout.id || dirtyRef.current.size === 0) return;
		const timer = setTimeout(() => void flushPlacements(), 350);
		return () => clearTimeout(timer);
	}, [dirtyVersion, flushPlacements, projection?.layout.id]);

	const dirtyCount = dirtyRef.current.size;
	useEffect(() => {
		if (dirtyCount === 0) return;
		const warn = (event: BeforeUnloadEvent) => event.preventDefault();
		window.addEventListener('beforeunload', warn);
		return () => window.removeEventListener('beforeunload', warn);
	}, [dirtyCount]);

	const moveNode = useCallback((nodeId: string, position: CanvasPosition) => {
		const current = projectionRef.current;
		const node = current?.nodes.find((candidate) => candidate.id === nodeId);
		if (!current || !node) return;
		const existing = dirtyRef.current.get(nodeId);
		const mutation = ++mutationRef.current;
		dirtyRef.current.set(nodeId, { patch: { nodeId, entityRef: node.entityRef, position, collapsed: node.collapsed, expectedRevision: existing?.patch.expectedRevision ?? node.revision }, mutation, retries: 0 });
		setSaveStatus('saving');
		setProjection({ ...current, nodes: current.nodes.map((candidate) => candidate.id === nodeId ? { ...candidate, position } : candidate) });
		setDirtyVersion((version) => version + 1);
	}, [setProjection]);

	const saveViewport = useCallback((viewport: CanvasViewport) => {
		const current = projectionRef.current;
		if (!current) return;
		setProjection({ ...current, layout: { ...current.layout, viewport } });
		if (viewportTimer.current) clearTimeout(viewportTimer.current);
		viewportTimer.current = setTimeout(async () => {
			const latest = projectionRef.current;
			if (!latest) return;
			try {
				setProjection(await api.patchCanvasViewport(latest.layout.id, latest.layout.version, viewport));
				setError('');
			} catch (caught) {
				setError(messageFrom(caught));
			}
		}, 500);
	}, [setProjection]);

	const reloadPosition = useCallback(async (nodeId: string) => {
		const current = projectionRef.current;
		if (!current) return;
		try {
			const server = await api.canvasLayout(current.layout.id);
			dirtyRef.current.delete(nodeId);
			setConflicts((previous) => previous.filter((id) => id !== nodeId));
			setProjection(server);
			setDirtyVersion((version) => version + 1);
		} catch (caught) {
			setError(messageFrom(caught));
		}
	}, [setProjection]);

	const reapplyPosition = useCallback(async (nodeId: string) => {
		const current = projectionRef.current;
		const local = current?.nodes.find((node) => node.id === nodeId);
		if (!current || !local) return;
		try {
			const server = await api.canvasLayout(current.layout.id);
			const latest = server.nodes.find((node) => node.id === nodeId);
			if (!latest) throw new Error('The node is no longer placed.');
			const mutation = ++mutationRef.current;
			dirtyRef.current.set(nodeId, { patch: { nodeId, entityRef: latest.entityRef, position: local.position, collapsed: local.collapsed, expectedRevision: latest.revision }, mutation, retries: 0 });
			setConflicts((previous) => previous.filter((id) => id !== nodeId));
			setProjection({ ...server, nodes: server.nodes.map((node) => node.id === nodeId ? { ...node, position: local.position } : node) });
			setDirtyVersion((version) => version + 1);
		} catch (caught) {
			setError(messageFrom(caught));
		}
	}, [setProjection]);

	const placeSession = useCallback(async (sessionId: string) => {
		const current = projectionRef.current;
		if (!current) return;
		const existing = current.nodes.find((node) => node.entityRef.sessionId === sessionId);
		if (existing && !existing.collapsed) return;
		const entityRef = current.unplaced.find((candidate) => candidate.kind === 'session' && candidate.sessionId === sessionId);
		if (!existing && !entityRef) return;
		setError('');
		try {
			setProjection(await api.patchCanvasPlacements(current.layout.id, [{
				nodeId: existing?.id ?? nodeID(entityRef!),
				entityRef: existing?.entityRef ?? entityRef!,
				position: existing?.position ?? deterministicPosition(current.nodes.length, 'session'),
				collapsed: false,
				expectedRevision: existing?.revision ?? 0
			}]));
		} catch (caught) {
			setError(messageFrom(caught));
		}
	}, [setProjection]);

	const removeNode = useCallback(async (nodeId: string) => {
		const current = projectionRef.current;
		const node = current?.nodes.find((candidate) => candidate.id === nodeId);
		if (!current || !node) return;
		try {
			dirtyRef.current.delete(nodeId);
			setProjection(await api.removeCanvasPlacement(current.layout.id, nodeId, node.revision));
			setConflicts((previous) => previous.filter((id) => id !== nodeId));
			setDirtyVersion((version) => version + 1);
		} catch (caught) {
			setError(messageFrom(caught));
		}
	}, [setProjection]);

	const setNodeCollapsed = useCallback((nodeId: string, collapsed: boolean) => {
		const current = projectionRef.current;
		const node = current?.nodes.find((candidate) => candidate.id === nodeId);
		if (!current || !node || node.collapsed === collapsed) return;
		const existing = dirtyRef.current.get(nodeId);
		const mutation = ++mutationRef.current;
		dirtyRef.current.set(nodeId, { patch: { nodeId, entityRef: node.entityRef, position: node.position, collapsed, expectedRevision: existing?.patch.expectedRevision ?? node.revision }, mutation, retries: 0 });
		setSaveStatus('saving');
		setProjection({ ...current, nodes: current.nodes.map((candidate) => candidate.id === nodeId ? { ...candidate, collapsed } : candidate) });
		setDirtyVersion((version) => version + 1);
	}, [setProjection]);

	const resetPositions = useCallback(() => {
		const current = projectionRef.current;
		if (!current) return;
		const ordered = [...current.nodes].sort((left, right) => left.kind.localeCompare(right.kind) || left.id.localeCompare(right.id));
		for (const [index, node] of ordered.entries()) moveNode(node.id, deterministicPosition(index, node.kind));
	}, [moveNode]);

	const arrangePlans = useCallback((grouping: 'status' | 'service' | 'service_status') => {
		const current = projectionRef.current;
		if (!current) return;
		const statusOrder = new Map(['unsorted', 'draft', 'in_progress', 'review', 'done'].map((status, index) => [status, index]));
		const plans = current.nodes.filter((node) => node.plan).sort((left, right) => {
			const leftPlan = left.plan!;
			const rightPlan = right.plan!;
			const byStatus = (statusOrder.get(leftPlan.status) ?? 99) - (statusOrder.get(rightPlan.status) ?? 99);
			const byService = (leftPlan.service ?? '').localeCompare(rightPlan.service ?? '');
			const byIdentifier = (leftPlan.identifier ?? left.id).localeCompare(rightPlan.identifier ?? right.id);
			if (grouping === 'status') return byStatus || byService || byIdentifier;
			if (grouping === 'service') return byService || byStatus || byIdentifier;
			return byService || byStatus || byIdentifier;
		});
		const groupKey = (node: CanvasNode) => {
			if (grouping === 'status') return node.plan!.status;
			if (grouping === 'service') return node.plan!.service || 'other';
			return `${node.plan!.service || 'other'}:${node.plan!.status}`;
		};
		const groups = Array.from(new Set(plans.map(groupKey)));
		const rowByGroup = new Map<string, number>();
		for (const node of plans) {
			const key = groupKey(node);
			const row = rowByGroup.get(key) ?? 0;
			moveNode(node.id, { x: 360 + groups.indexOf(key) * 320, y: row * 190 });
			rowByGroup.set(key, row + 1);
		}
	}, [moveNode]);

	return { projection, loading, error, conflicts, dirtyCount, saveStatus, hasUnsavedChanges: dirtyCount > 0, moveNode, setNodeCollapsed, arrangePlans, saveViewport, reloadPosition, reapplyPosition, placeSession, removeNode, resetPositions, reload: load, refresh };
}

function overlayDirty(projection: CanvasProjection, dirty: Map<string, DirtyPlacement>): CanvasProjection {
	if (dirty.size === 0) return projection;
	return { ...projection, nodes: projection.nodes.map((node) => {
		const entry = dirty.get(node.id);
		return entry ? { ...node, position: entry.patch.position, collapsed: entry.patch.collapsed } : node;
	}) };
}

function messageFrom(error: unknown): string {
	return error instanceof Error ? error.message : 'Canvas request failed.';
}

export function canvasNodeById(nodes: CanvasNode[], id: string) {
	return nodes.find((node) => node.id === id);
}

function nodeID(ref: CanvasProjection['unplaced'][number]) {
	if (ref.kind === 'workspace') return `workspace:${ref.workspaceId}`;
	if (ref.kind === 'plan') return `plan:${ref.itemId}`;
	return `session:${ref.sessionId}`;
}

function deterministicPosition(index: number, kind: CanvasProjection['unplaced'][number]['kind']): CanvasPosition {
	if (kind === 'workspace') return { x: 0, y: 0 };
	const offset = kind === 'session' ? 420 : 0;
	return { x: 360 + (index % 3) * 320, y: offset + Math.floor(index / 3) * 190 };
}
