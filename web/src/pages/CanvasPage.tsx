import { useEffect, useMemo, useState } from 'react';
import type { CanvasLocation } from '../app/router';
import { useCanvasState } from '../features/canvas/useCanvasState';
import { CanvasBoard } from '../features/canvas/CanvasBoard';
import { CanvasWorkbench } from '../features/canvas/CanvasWorkbench';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { api } from '../lib/api';
import type { WorkspaceConfig } from '../lib/types';
import '../features/canvas/canvas.css';

export function CanvasPage({ workspace, location, onLocationChange, onOpenItem, onOpenWorkspaces }: { workspace?: WorkspaceConfig; location?: CanvasLocation; onLocationChange: (location: CanvasLocation) => void; onOpenItem?: (itemId: string) => void; onOpenWorkspaces?: () => void }) {
	const workspaceId = location?.workspaceId ?? workspace?.id;
	const fallbackBranch = workspace?.lastSelectedBranch || workspace?.baselineBranch;
	const branch = location?.branch ?? fallbackBranch;
	const canvas = useCanvasState(workspaceId, branch);
	const [branches, setBranches] = useState<string[]>([]);
	const [selectedId, setSelectedId] = useState<string>();
	const [confirmation, setConfirmation] = useState<'reset' | 'remove'>();

	useEffect(() => {
		if (!workspaceId) {
			setBranches([]);
			return;
		}
		void api.workspaceBranches(workspaceId).then((result) => setBranches(result.branches)).catch(() => setBranches(branch ? [branch] : []));
	}, [branch, workspaceId]);

	useEffect(() => {
		if (workspaceId && (location?.workspaceId !== workspaceId || location?.branch !== branch)) {
			onLocationChange({ workspaceId, branch });
		}
	}, [branch, location?.branch, location?.workspaceId, onLocationChange, workspaceId]);

	const title = useMemo(() => canvas.projection?.nodes.find((node) => node.kind === 'workspace')?.workspace?.name ?? workspace?.name ?? 'Canvas', [canvas.projection, workspace?.name]);
	const selectedNode = canvas.projection?.nodes.find((node) => node.id === selectedId);
	const closeWorkbench = () => {
		const focusID = selectedId;
		setSelectedId(undefined);
		if (focusID) requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-canvas-node-id="${focusID.replaceAll('"', '\\"')}"]`)?.focus());
	};
	if (!workspaceId) return <section className="empty-state"><h1>Canvas</h1><p>Select a workspace to arrange plans and terminal sessions.</p></section>;

	return (
		<section className="canvas-page" aria-label="Workspace Canvas">
			<header className="canvas-page-header">
				<div><span className="canvas-eyebrow">Workspace Canvas</span><h1>{title}</h1></div>
				<label>Branch<select aria-label="Canvas branch" value={branch} onChange={(event) => {
					if (canvas.hasUnsavedChanges && !window.confirm('Some positions are not saved yet. Switching branches may lose those moves. Continue?')) return;
					onLocationChange({ workspaceId, branch: event.target.value });
				}}>{Array.from(new Set([branch, ...branches])).filter(Boolean).map((name) => <option key={name} value={name}>{name}</option>)}</select></label>
			</header>
			{canvas.loading && <div className="canvas-state" role="status">Loading Canvas…</div>}
			{canvas.error && <div className="canvas-state error" role="alert">{canvas.error}<button type="button" onClick={() => void canvas.reload()}>Retry</button></div>}
			{canvas.projection && <div className="canvas-state canvas-status-strip" data-testid="canvas-ready">
				<strong>{canvas.projection.nodes.length} placed</strong><span>{canvas.projection.unplaced.length} unplaced</span><span role="status" aria-live="polite">{canvas.saveStatus === 'saving' ? `Saving ${canvas.dirtyCount} position${canvas.dirtyCount === 1 ? '' : 's'}…` : canvas.saveStatus === 'saved' ? 'Saved' : ''}</span>
			</div>}
			{canvas.projection && <div className={`canvas-work-area${selectedNode ? ' workbench-open' : ''}`}><CanvasBoard projection={canvas.projection} conflicts={canvas.conflicts} selectedId={selectedId} onSelect={setSelectedId} onMoveNode={canvas.moveNode} onSaveViewport={canvas.saveViewport} onReloadPosition={(id) => void canvas.reloadPosition(id)} onReapplyPosition={(id) => void canvas.reapplyPosition(id)} onPlaceUnplaced={() => void canvas.placeUnplaced()} onReset={() => setConfirmation('reset')} onRemove={(node) => { setSelectedId(node.id); setConfirmation('remove'); }} /><CanvasWorkbench projection={canvas.projection} selectedNode={selectedNode} onClose={closeWorkbench} onReload={canvas.refresh} onSelectNode={setSelectedId} onPlaceUnplaced={canvas.placeUnplaced} onOpenFullView={(node) => node.plan ? onOpenItem?.(node.plan.itemId) : node.workspace ? onOpenWorkspaces?.() : undefined} /></div>}
			{confirmation === 'reset' && <ConfirmDialog title="Reset Canvas layout?" message="Preview: the workspace returns to the origin and plans and sessions return to the deterministic grid. This changes presentation only; repository entities and terminal processes are untouched." confirmLabel="Reset layout" onCancel={() => setConfirmation(undefined)} onConfirm={() => { canvas.resetPositions(); setConfirmation(undefined); }} />}
			{confirmation === 'remove' && selectedNode && <ConfirmDialog title="Remove node from Canvas?" message={`Remove ${selectedNode.kind} from this layout? The underlying ${selectedNode.kind} and any live terminal process continue unchanged.`} confirmLabel="Remove from Canvas" danger onCancel={() => setConfirmation(undefined)} onConfirm={() => { void canvas.removeNode(selectedNode.id); setSelectedId(undefined); setConfirmation(undefined); }} />}
		</section>
	);
}
