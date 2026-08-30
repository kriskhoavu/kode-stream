import { useEffect, useMemo, useState } from 'react';
import type { CanvasLocation } from '../app/router';
import { useCanvasState } from '../features/canvas/useCanvasState';
import { BranchUpstreamIndicator } from '../features/canvas/BranchUpstreamIndicator';
import { CanvasBoard } from '../features/canvas/CanvasBoard';
import { CanvasWorkbench } from '../features/canvas/CanvasWorkbench';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { BranchCheckoutPicker } from '../features/workstream/BranchCheckoutPicker';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import type { CanvasNode, CanvasSection, WorkspaceConfig } from '../lib/types';
import { FolderGit2, Workflow } from 'lucide-react';
import '../features/canvas/canvas.css';
import { readPreference, readStringPreference, writePreference } from '../shared/preferences/store';

export function CanvasPage({ workspace, location, onLocationChange, onOpenItem, onOpenWorkspaces }: { workspace?: WorkspaceConfig; location?: CanvasLocation; onLocationChange: (location: CanvasLocation) => void; onOpenItem?: (itemId: string) => void; onOpenWorkspaces?: () => void }) {
	const workspaceId = location?.workspaceId ?? workspace?.id;
	const canvas = useCanvasState(workspaceId);
	const branchState = useWorkspaceBranches(workspace ? [workspace] : []);
	const branches = workspace ? branchState.states[workspace.id] : undefined;
	const [selectedId, setSelectedId] = useState<string>();
	const [confirmation, setConfirmation] = useState<'reset' | 'remove'>();
	const [aiSessionDialogOpen, setAISessionDialogOpen] = useState(false);
	const [sections, setSections] = useState<CanvasSection[]>([]);
	const [sectionsLoadedFor, setSectionsLoadedFor] = useState('');
	const [serviceGridColumns, setServiceGridColumns] = useState(1);
	const sectionLayoutID = canvas.projection?.layout.id;
	const defaultServiceGroups = useMemo(() => defaultServiceSections(canvas.projection?.nodes ?? []), [canvas.projection?.nodes]);

	useEffect(() => {
		if (!sectionLayoutID) { setSections([]); setSectionsLoadedFor(''); return; }
		if (sectionsLoadedFor === sectionLayoutID) return;
		try {
			const existing = readPreference<CanvasSection[]>(`canvas-sections:${sectionLayoutID}`, (value) => Array.isArray(value) ? value.filter((entry): entry is CanvasSection => Boolean(entry) && typeof entry === 'object') : undefined, []);
			const initializedKey = `canvas-service-sections-version:${sectionLayoutID}`;
			const defaultSectionVersion = '3';
			if (readStringPreference(initializedKey) !== defaultSectionVersion) {
				setSections([...defaultServiceGroups.sections, ...existing.filter((section) => !isDefaultSection(section))]);
			} else {
				setSections(existing);
			}
			writePreference(initializedKey, defaultSectionVersion);
		} catch { setSections([]); }
		setSectionsLoadedFor(sectionLayoutID);
	}, [canvas.moveNode, defaultServiceGroups, sectionLayoutID, sectionsLoadedFor]);
	useEffect(() => {
		if (!sectionLayoutID || sectionsLoadedFor !== sectionLayoutID) return;
		writePreference(`canvas-sections:${sectionLayoutID}`, sections);
	}, [sectionLayoutID, sections, sectionsLoadedFor]);

	useEffect(() => {
		if (workspaceId && location?.workspaceId !== workspaceId) {
			onLocationChange({ workspaceId });
		}
	}, [location?.workspaceId, onLocationChange, workspaceId]);

	const title = useMemo(() => canvas.projection?.nodes.find((node) => node.kind === 'workspace')?.workspace?.name ?? workspace?.name ?? 'Canvas', [canvas.projection, workspace?.name]);
	const selectedNode = canvas.projection?.nodes.find((node) => node.id === selectedId);
	const workspaceNode = canvas.projection?.nodes.find((node) => node.kind === 'workspace');
	const terminalLaunch = selectedNode?.plan?.actions['terminal.launch'] ?? workspaceNode?.workspace?.actions['terminal.launch'];
	const canOpenAISession = terminalLaunch?.state === 'available';
	const workbenchNode = selectedNode?.session ? undefined : selectedNode;
	const arrangeGroupedSections = (grouping: 'status' | 'service' | 'service_status' | 'service_grid') => {
		if (!canvas.projection) return;
		const columns = grouping === 'service_grid' ? serviceGridColumns === 4 ? 1 : serviceGridColumns + 1 : 1;
		if (grouping === 'service_grid') setServiceGridColumns(columns);
		const arranged = defaultServiceSections(canvas.projection.nodes, grouping !== 'service', columns);
		setSections((current) => [...arranged.sections, ...current.filter((section) => !isDefaultSection(section))]);
		arranged.positions.forEach(({ id, position }) => canvas.moveNode(id, position));
	};
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
		const top = Math.min(...nodes.map((node) => node.position.y)) - 64;
		const right = Math.max(...nodes.map((node) => node.position.x + (node.session && !node.collapsed ? 660 : 245))) + 28;
		const bottom = Math.max(...nodes.map((node) => node.position.y + (node.session && !node.collapsed ? 500 : 116))) + 28;
		setSections((current) => [...current, { id: globalThis.crypto?.randomUUID?.() ?? `section-${Date.now()}`, title, position: { x: left, y: top }, width: right - left, height: bottom - top, nodeIds }]);
	};
	if (!workspaceId) return <section className="empty-state"><h1>Workbench</h1><p>Select a workspace to arrange plans and terminal sessions.</p></section>;

	return (
		<section className="canvas-page" aria-label="Workspace Canvas">
			<header className="page-title workstream-title canvas-page-header">
				<div className="workstream-heading"><div><h1><Workflow size={22} /> Workbench</h1><span><FolderGit2 size={15} /> {title}</span></div></div>
				<BranchUpstreamIndicator git={canvas.projection?.nodes.find((node) => node.kind === 'workspace')?.workspace?.git} />
				{workspace && <BranchCheckoutPicker workspaceId={workspace.id} currentCheckoutBranch={branches?.current ?? canvas.projection?.layout.branchKey ?? workspace.baselineBranch} branches={branches?.branches ?? []} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" disabled={canvas.loading} onSwitched={async () => { setSelectedId(undefined); await canvas.reload(); }} />}
			</header>
			{canvas.loading && <div className="canvas-state" role="status">Loading Canvas…</div>}
			{canvas.error && <div className="canvas-state error" role="alert">{canvas.error}<button type="button" onClick={() => void canvas.reload()}>Retry</button></div>}
			{canvas.projection && <span className="sr-only" role="status" aria-live="polite">{canvas.saveStatus === 'saving' ? `Saving ${canvas.dirtyCount} position${canvas.dirtyCount === 1 ? '' : 's'}…` : canvas.saveStatus === 'saved' ? 'Saved' : ''}</span>}
			{canvas.projection && <div className={`canvas-work-area${workbenchNode ? ' workbench-open' : ''}`}><CanvasBoard projection={canvas.projection} sections={sections} conflicts={canvas.conflicts} selectedId={selectedId} onSelect={setSelectedId} onMoveNode={canvas.moveNode} onSetCollapsed={canvas.setNodeCollapsed} onArrange={arrangeGroupedSections} serviceGridColumns={serviceGridColumns} onSaveViewport={canvas.saveViewport} onReload={canvas.refresh} onReloadPosition={(id) => void canvas.reloadPosition(id)} onReapplyPosition={(id) => void canvas.reapplyPosition(id)} onReset={() => setConfirmation('reset')} onRemove={(node) => { setSelectedId(node.id); setConfirmation('remove'); }} onCreateSection={createSection} onMoveSection={(id, position, delta) => { const section = sections.find((candidate) => candidate.id === id); if (section) canvas.projection?.nodes.filter((node) => section.nodeIds.includes(node.id)).forEach((node) => canvas.moveNode(node.id, { x: node.position.x + delta.x, y: node.position.y + delta.y })); setSections((current) => current.map((candidate) => candidate.id === id ? { ...candidate, position } : id === 'workspace-group' && candidate.id.startsWith('service-') ? { ...candidate, position: { x: candidate.position.x + delta.x, y: candidate.position.y + delta.y } } : candidate)); }} onRemoveSection={(id) => setSections((current) => current.filter((section) => section.id !== id))} onOpenAISession={() => setAISessionDialogOpen(true)} aiSessionDisabled={!canOpenAISession} /><CanvasWorkbench projection={canvas.projection} selectedNode={workbenchNode} aiSessionDialogOpen={aiSessionDialogOpen} onAISessionDialogOpenChange={setAISessionDialogOpen} onClose={closeWorkbench} onReload={canvas.refresh} onSelectNode={setSelectedId} onPlaceSession={canvas.placeSession} onOpenFullView={(node) => node.plan ? onOpenItem?.(node.plan.itemId) : node.workspace ? onOpenWorkspaces?.() : undefined} /></div>}
			{confirmation === 'reset' && <ConfirmDialog title="Reset Canvas layout?" message="Restore the default workspace and service groups, then return plans and terminal sessions to their grouped layout. This changes presentation only; repository entities and terminal processes are untouched." confirmLabel="Reset layout" onCancel={() => setConfirmation(undefined)} onConfirm={() => { canvas.resetPositions(); setServiceGridColumns(1); setSections(defaultServiceGroups.sections); defaultServiceGroups.positions.forEach(({ id, position }) => canvas.moveNode(id, position)); setConfirmation(undefined); }} />}
			{confirmation === 'remove' && selectedNode && <ConfirmDialog title="Remove node from Canvas?" message={`Remove ${selectedNode.kind} from this layout? The underlying ${selectedNode.kind} and any live terminal process continue unchanged.`} confirmLabel="Remove from Canvas" danger onCancel={() => setConfirmation(undefined)} onConfirm={() => { void canvas.removeNode(selectedNode.id); setSelectedId(undefined); setConfirmation(undefined); }} />}
		</section>
	);
}

function defaultServiceSections(nodes: CanvasNode[], orderByStatus = false, columns = 1) {
	const plans = nodes.filter((node) => node.plan);
	const groups = new Map<string, typeof plans>();
	for (const node of plans) {
		const service = node.plan?.service?.trim() || 'Other';
		groups.set(service, [...(groups.get(service) ?? []), node]);
	}
	const positions: Array<{ id: string; position: { x: number; y: number } }> = [];
	let nextServiceX = 140;
	const serviceSections = Array.from(groups.entries()).sort(([left], [right]) => left.localeCompare(right)).map(([service, group]) => {
		const ordered = [...group].sort((left, right) => orderByStatus ? statusRank(left.plan?.status) - statusRank(right.plan?.status) || (left.plan?.identifier ?? left.id).localeCompare(right.plan?.identifier ?? right.id) : (left.plan?.identifier ?? left.id).localeCompare(right.plan?.identifier ?? right.id));
		const groupColumns = Math.min(columns, Math.max(1, ordered.length));
		const serviceWidth = Math.max(301, groupColumns * 245 + (groupColumns - 1) * 28 + 56);
		const x = nextServiceX;
		nextServiceX += serviceWidth + 160;
		ordered.forEach((node, index) => positions.push({ id: node.id, position: { x: x + (index % groupColumns) * 273, y: 32 + Math.floor(index / groupColumns) * 190 } }));
		return { id: `service-${service.toLowerCase().replaceAll(/[^a-z0-9]+/g, '-')}`, title: formatServiceTitle(service), position: { x: x - 28, y: 0 }, width: serviceWidth, height: Math.max(200, Math.ceil(ordered.length / groupColumns) * 190 + 62), nodeIds: ordered.map((node) => node.id) };
	});
	const workspaceName = nodes.find((node) => node.kind === 'workspace')?.workspace?.name ?? 'Workspace';
	const maxServiceHeight = Math.max(200, ...serviceSections.map((section) => section.height));
	const sessionStartY = maxServiceHeight + 92;
	const sessions = nodes.filter((node) => node.kind === 'session');
	sessions.forEach((node, index) => positions.push({ id: node.id, position: { x: 140 + index * 700, y: sessionStartY } }));
	const sessionRight = sessions.reduce((right, node, index) => Math.max(right, 140 + index * 700 + (node.collapsed ? 245 : 660)), 0);
	const sessionBottom = sessions.reduce((bottom, node) => Math.max(bottom, sessionStartY + (node.collapsed ? 116 : 500)), 0);
	const serviceRight = serviceSections.reduce((right, section) => Math.max(right, section.position.x + section.width), 0);
	const workspaceSection: CanvasSection = { id: 'workspace-group', title: workspaceName, position: { x: 60, y: -82 }, width: Math.max(380, serviceRight - 12, sessionRight - 8), height: Math.max(maxServiceHeight + 126, sessionBottom + 122), nodeIds: nodes.filter((node) => node.kind !== 'workspace').map((node) => node.id) };
	return { sections: [workspaceSection, ...serviceSections], positions };
}

function formatServiceTitle(service: string) { return service.replaceAll(/[-_]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase()); }
function isDefaultSection(section: CanvasSection) { return section.id === 'workspace-group' || section.id.startsWith('service-'); }
function statusRank(status?: string) { return ['unsorted', 'draft', 'in_progress', 'review', 'done'].indexOf(status ?? 'unsorted'); }
