import { useEffect, useMemo, useState } from 'react';
import type { CanvasLocation } from '../app/router';
import { useCanvasState } from '../features/canvas/useCanvasState';
import { CanvasBoard } from '../features/canvas/CanvasBoard';
import { CanvasWorkbench } from '../features/canvas/CanvasWorkbench';
import { ConfirmDialog } from '../components/ConfirmDialog';
import type { CanvasSection, WorkspaceConfig } from '../lib/types';
import { FolderGit2, Workflow } from 'lucide-react';
import '../features/canvas/canvas.css';

export function CanvasPage({ workspace, location, onLocationChange, onOpenItem, onOpenWorkspaces }: { workspace?: WorkspaceConfig; location?: CanvasLocation; onLocationChange: (location: CanvasLocation) => void; onOpenItem?: (itemId: string) => void; onOpenWorkspaces?: () => void }) {
	const workspaceId = location?.workspaceId ?? workspace?.id;
	const canvas = useCanvasState(workspaceId);
	const [selectedId, setSelectedId] = useState<string>();
	const [confirmation, setConfirmation] = useState<'reset' | 'remove'>();
	const [aiSessionDialogOpen, setAISessionDialogOpen] = useState(false);
	const [sections, setSections] = useState<CanvasSection[]>([]);
	const [sectionsLoadedFor, setSectionsLoadedFor] = useState('');
	const sectionLayoutID = canvas.projection?.layout.id;

	useEffect(() => {
		if (!sectionLayoutID) { setSections([]); setSectionsLoadedFor(''); return; }
		try {
			const stored = localStorage.getItem(`canvas-sections:${sectionLayoutID}`);
			setSections(stored ? JSON.parse(stored) as CanvasSection[] : []);
		} catch { setSections([]); }
		setSectionsLoadedFor(sectionLayoutID);
	}, [sectionLayoutID]);
	useEffect(() => {
		if (!sectionLayoutID || sectionsLoadedFor !== sectionLayoutID) return;
		localStorage.setItem(`canvas-sections:${sectionLayoutID}`, JSON.stringify(sections));
	}, [sectionLayoutID, sections, sectionsLoadedFor]);

	useEffect(() => {
		if (workspaceId && location?.workspaceId !== workspaceId) {
			onLocationChange({ workspaceId });
		}
	}, [location?.workspaceId, onLocationChange, workspaceId]);

	const title = useMemo(() => canvas.projection?.nodes.find((node) => node.kind === 'workspace')?.workspace?.name ?? workspace?.name ?? 'Canvas', [canvas.projection, workspace?.name]);
	const selectedNode = canvas.projection?.nodes.find((node) => node.id === selectedId);
	const terminalLaunch = selectedNode?.plan?.actions['terminal.launch'] ?? selectedNode?.workspace?.actions['terminal.launch'];
	const canOpenAISession = Boolean(selectedNode && (selectedNode.plan || selectedNode.workspace) && terminalLaunch?.state === 'available');
	const workbenchNode = selectedNode?.session ? undefined : selectedNode;
	const closeWorkbench = () => {
		const focusID = selectedId;
		setAISessionDialogOpen(false);
		setSelectedId(undefined);
		if (focusID) requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-canvas-node-id="${focusID.replaceAll('"', '\\"')}"]`)?.focus());
	};
	const createSection = (nodeIds: string[]) => {
		const nodes = canvas.projection?.nodes.filter((node) => nodeIds.includes(node.id)) ?? [];
		if (nodes.length < 2) return;
		const title = window.prompt('Section name', 'New section')?.trim();
		if (!title) return;
		const left = Math.min(...nodes.map((node) => node.position.x)) - 28;
		const top = Math.min(...nodes.map((node) => node.position.y)) - 42;
		const right = Math.max(...nodes.map((node) => node.position.x + (node.session && !node.collapsed ? 660 : 245))) + 28;
		const bottom = Math.max(...nodes.map((node) => node.position.y + (node.session && !node.collapsed ? 500 : 116))) + 28;
		setSections((current) => [...current, { id: globalThis.crypto?.randomUUID?.() ?? `section-${Date.now()}`, title, position: { x: left, y: top }, width: right - left, height: bottom - top, nodeIds }]);
	};
	if (!workspaceId) return <section className="empty-state"><h1>Workbench</h1><p>Select a workspace to arrange plans and terminal sessions.</p></section>;

	return (
		<section className="canvas-page" aria-label="Workspace Canvas">
			<header className="page-title workstream-title canvas-page-header">
				<div className="workstream-heading"><div><h1><Workflow size={22} /> Workbench</h1><span><FolderGit2 size={15} /> {title}</span></div></div>
				<span className="branch-context-chip" aria-label={`Checkout: ${canvas.projection?.layout.branchKey ?? 'Loading'}`}><span>Checkout</span><strong>{canvas.projection?.layout.branchKey ?? 'Loading…'}</strong></span>
			</header>
			{canvas.loading && <div className="canvas-state" role="status">Loading Canvas…</div>}
			{canvas.error && <div className="canvas-state error" role="alert">{canvas.error}<button type="button" onClick={() => void canvas.reload()}>Retry</button></div>}
			{canvas.projection && <span className="sr-only" role="status" aria-live="polite">{canvas.saveStatus === 'saving' ? `Saving ${canvas.dirtyCount} position${canvas.dirtyCount === 1 ? '' : 's'}…` : canvas.saveStatus === 'saved' ? 'Saved' : ''}</span>}
			{canvas.projection && <div className={`canvas-work-area${workbenchNode ? ' workbench-open' : ''}`}><CanvasBoard projection={canvas.projection} sections={sections} conflicts={canvas.conflicts} selectedId={selectedId} onSelect={setSelectedId} onMoveNode={canvas.moveNode} onSetCollapsed={canvas.setNodeCollapsed} onArrange={canvas.arrangePlans} onSaveViewport={canvas.saveViewport} onReload={canvas.refresh} onReloadPosition={(id) => void canvas.reloadPosition(id)} onReapplyPosition={(id) => void canvas.reapplyPosition(id)} onReset={() => setConfirmation('reset')} onRemove={(node) => { setSelectedId(node.id); setConfirmation('remove'); }} onCreateSection={createSection} onMoveSection={(id, position, delta) => { const section = sections.find((candidate) => candidate.id === id); if (section) canvas.projection?.nodes.filter((node) => section.nodeIds.includes(node.id)).forEach((node) => canvas.moveNode(node.id, { x: node.position.x + delta.x, y: node.position.y + delta.y })); setSections((current) => current.map((candidate) => candidate.id === id ? { ...candidate, position } : candidate)); }} onRemoveSection={(id) => setSections((current) => current.filter((section) => section.id !== id))} onOpenAISession={() => setAISessionDialogOpen(true)} aiSessionDisabled={!canOpenAISession} /><CanvasWorkbench projection={canvas.projection} selectedNode={workbenchNode} aiSessionDialogOpen={aiSessionDialogOpen} onAISessionDialogOpenChange={setAISessionDialogOpen} onClose={closeWorkbench} onReload={canvas.refresh} onSelectNode={setSelectedId} onPlaceSession={canvas.placeSession} onOpenFullView={(node) => node.plan ? onOpenItem?.(node.plan.itemId) : node.workspace ? onOpenWorkspaces?.() : undefined} /></div>}
			{confirmation === 'reset' && <ConfirmDialog title="Reset Canvas layout?" message="Preview: the workspace returns to the origin and plans and sessions return to the deterministic grid. This changes presentation only; repository entities and terminal processes are untouched." confirmLabel="Reset layout" onCancel={() => setConfirmation(undefined)} onConfirm={() => { canvas.resetPositions(); setConfirmation(undefined); }} />}
			{confirmation === 'remove' && selectedNode && <ConfirmDialog title="Remove node from Canvas?" message={`Remove ${selectedNode.kind} from this layout? The underlying ${selectedNode.kind} and any live terminal process continue unchanged.`} confirmLabel="Remove from Canvas" danger onCancel={() => setConfirmation(undefined)} onConfirm={() => { void canvas.removeNode(selectedNode.id); setSelectedId(undefined); setConfirmation(undefined); }} />}
		</section>
	);
}
