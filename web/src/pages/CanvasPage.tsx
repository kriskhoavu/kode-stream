import { useEffect, useMemo, useState } from 'react';
import type { CanvasLocation } from '../app/router';
import { useCanvasState } from '../features/canvas/useCanvasState';
import { api } from '../lib/api';
import type { WorkspaceConfig } from '../lib/types';
import '../features/canvas/canvas.css';

export function CanvasPage({ workspace, location, onLocationChange }: { workspace?: WorkspaceConfig; location?: CanvasLocation; onLocationChange: (location: CanvasLocation) => void }) {
	const workspaceId = location?.workspaceId ?? workspace?.id;
	const fallbackBranch = workspace?.lastSelectedBranch || workspace?.baselineBranch;
	const branch = location?.branch ?? fallbackBranch;
	const canvas = useCanvasState(workspaceId, branch);
	const [branches, setBranches] = useState<string[]>([]);

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
			{canvas.projection && <div className="canvas-state" data-testid="canvas-ready">
				<strong>{canvas.projection.nodes.length} placed</strong><span>{canvas.projection.unplaced.length} unplaced</span>{canvas.dirtyCount > 0 && <span aria-live="polite">Saving {canvas.dirtyCount} position{canvas.dirtyCount === 1 ? '' : 's'}…</span>}
			</div>}
		</section>
	);
}
