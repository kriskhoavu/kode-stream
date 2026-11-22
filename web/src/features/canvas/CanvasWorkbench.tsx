import { useCallback, useEffect, useRef, useState } from 'react';
import { useHorizontalDrag } from '../../components/useHorizontalDrag';
import { GitCompare, GripVertical, Info, PanelRightClose, PanelRightOpen, Play, RefreshCw, TerminalSquare, Ticket } from 'lucide-react';
import { api } from '../../shared/api';
import type { AISessionLaunchResult, AutomationDisplayMode, CanvasNode, CanvasProjection, E2ERunbook, EmbeddedAISessionResult, VerificationJob, VerifyProfile } from '../../lib/types';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';
import { JiraItemPanel } from '../jira/JiraItemPanel';
import { SessionChangesPanel } from '../sessions/SessionChangesPanel';
import { CanvasItemInfo } from './CanvasItemInfo';
import { CanvasQualityPanel, VerificationSummary, shortSHA } from './CanvasQualityPanel';
import { SessionTerminalSummary } from './SessionTerminalSummary';

type WorkbenchTab = 'info' | 'jira' | 'quality' | 'terminal' | 'changes';

export function CanvasWorkbench({ projection, selectedNode, aiSessionDialogOpen = false, onAISessionDialogOpenChange, onReload, onSelectNode, onPlaceSession, onOpenFullView }: { projection: CanvasProjection; selectedNode?: CanvasNode; aiSessionDialogOpen?: boolean; onAISessionDialogOpenChange?: (open: boolean) => void; onReload: () => Promise<unknown> | void; onSelectNode: (id: string) => void; onPlaceSession: (sessionId: string) => Promise<unknown> | void; onOpenFullView?: (node: CanvasNode) => void }) {
	const [rightPanelTab, setRightPanelTab] = useState<WorkbenchTab>('info');
	const [collapsed, setCollapsed] = useState(false);
	const [panelWidth, setPanelWidth] = useState(410);
	const [verifying, setVerifying] = useState(false);
	const [verificationJob, setVerificationJob] = useState<VerificationJob>();
	const [e2eRunbooks, setE2ERunbooks] = useState<{ runbooks: E2ERunbook[]; diagnostic?: string }>({ runbooks: [] });
	const [error, setError] = useState('');
	const pendingKey = useRef('');
	const verificationPollRef = useRef(0);
	const verificationAbortRef = useRef<AbortController | undefined>(undefined);
	const runbooksAbortRef = useRef<AbortController | undefined>(undefined);
	const workbenchRef = useRef<HTMLElement>(null);
	const startDrag = useHorizontalDrag();
	const mountedRef = useRef(false);
	const workspaceNode = projection.nodes.find((node) => node.kind === 'workspace')?.workspace;

	const launchCapability = selectedNode?.plan?.actions['terminal.launch'] ?? selectedNode?.workspace?.actions['terminal.launch'];
	const verificationContext = selectedNode?.workspace ?? (selectedNode?.plan ? workspaceNode : undefined);
	const verificationCapability = selectedNode?.plan?.actions['verification.run'] ?? verificationContext?.actions['verification.run'];
	const itemId = selectedNode?.plan?.itemId;
	const refreshE2ERunbooks = useCallback(async () => {
		runbooksAbortRef.current?.abort();
		const controller = new AbortController();
		runbooksAbortRef.current = controller;
		if (!itemId) {
			setE2ERunbooks({ runbooks: [] });
			return;
		}
		try {
			const runbooks = await api.itemE2ERunbooks(itemId, controller.signal);
			if (!controller.signal.aborted) setE2ERunbooks(runbooks);
		} catch (caught) {
			if (!controller.signal.aborted) setE2ERunbooks({ runbooks: [], diagnostic: caught instanceof Error ? caught.message : 'E2E coverage could not be loaded.' });
		}
	}, [itemId]);

	useEffect(() => {
		verificationAbortRef.current?.abort();
		verificationPollRef.current += 1;
		setRightPanelTab(selectedNode?.session ? 'terminal' : 'info');
		setCollapsed(false);
		setVerificationJob(undefined);
		void refreshE2ERunbooks();
		return () => { verificationAbortRef.current?.abort(); verificationPollRef.current += 1; runbooksAbortRef.current?.abort(); };
	}, [selectedNode?.id, itemId, refreshE2ERunbooks]);
	useEffect(() => {
		mountedRef.current = true;
		return () => { mountedRef.current = false; };
	}, []);

	const waitForVerification = async (workspaceId: string, job: VerificationJob, pollID: number, signal: AbortSignal) => {
		let current = job;
		for (let attempt = 0; attempt < 60 && (current.status === 'queued' || current.status === 'running'); attempt += 1) {
			if (!await abortableDelay(1000, signal) || verificationPollRef.current !== pollID) return;
			current = await api.verificationJob(workspaceId, current.id, signal);
			if (signal.aborted || verificationPollRef.current !== pollID) return;
			setVerificationJob(current);
			await Promise.resolve(onReload());
		}
	};

	const completeEmbeddedLaunch = async (result: EmbeddedAISessionResult) => {
		if (pendingKey.current) return;
		pendingKey.current = result.session.id;
		setError('');
		try {
			await Promise.resolve(onReload());
			if (!mountedRef.current) return;
			await Promise.resolve(onPlaceSession(result.session.id));
			if (!mountedRef.current) return;
			onSelectNode(`session:${result.session.id}`);
		} catch (caught) {
			if (mountedRef.current) setError(caught instanceof Error ? caught.message : 'AI session could not be added to the canvas.');
		} finally {
			pendingKey.current = '';
		}
	};
	const sessionLaunched = (result: AISessionLaunchResult | EmbeddedAISessionResult) => {
		if ('session' in result) {
			void completeEmbeddedLaunch(result);
			return;
		}
		setError('');
	};

	const runVerification = async (profile: VerifyProfile) => {
		if (!verificationContext || verificationCapability?.state !== 'available' || verifying) return;
		setVerifying(true);
		setError('');
		verificationAbortRef.current?.abort();
		const controller = new AbortController();
		verificationAbortRef.current = controller;
		const pollID = ++verificationPollRef.current;
		try {
			const job = await api.createVerificationJob(verificationContext.id, { profile, trigger: 'manual_checkpoint', terminalMode: 'embedded' }, controller.signal);
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			setVerificationJob(job);
			await Promise.resolve(onReload());
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			await waitForVerification(verificationContext.id, job, pollID, controller.signal);
		} catch (caught) {
			if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Verification could not run.');
		} finally {
			if (!controller.signal.aborted && verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const rerunVerification = async () => {
		const job = verificationJob ?? verificationContext?.verification;
		if (!verificationContext || !job || verifying) return;
		setVerifying(true);
		setError('');
		verificationAbortRef.current?.abort();
		const controller = new AbortController();
		verificationAbortRef.current = controller;
		const pollID = ++verificationPollRef.current;
		try {
			const next = await api.rerunVerificationJob(verificationContext.id, job.id, undefined, controller.signal);
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			setVerificationJob(next);
			await Promise.resolve(onReload());
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			await waitForVerification(verificationContext.id, next, pollID, controller.signal);
		} catch (caught) {
			if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Verification could not be rerun.');
		} finally {
			if (!controller.signal.aborted && verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const runAutomationVerification = async (input: { environment: string; displayMode: AutomationDisplayMode; selectedSpecs: string[] }) => {
		if (!verificationContext || verificationCapability?.state !== 'available' || verifying || input.selectedSpecs.length === 0) return;
		setVerifying(true);
		setError('');
		verificationAbortRef.current?.abort();
		const controller = new AbortController();
		verificationAbortRef.current = controller;
		const pollID = ++verificationPollRef.current;
		try {
			const job = await api.createVerificationJob(verificationContext.id, { mode: 'automation', ...input, trigger: 'manual_checkpoint', terminalMode: 'embedded' }, controller.signal);
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			setVerificationJob(job);
			await Promise.resolve(onReload());
			if (controller.signal.aborted || verificationPollRef.current !== pollID) return;
			await waitForVerification(verificationContext.id, job, pollID, controller.signal);
		} catch (caught) {
			if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Automation verification could not run.');
		} finally {
			if (!controller.signal.aborted && verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const startResize = (event: React.PointerEvent<HTMLButtonElement>) => {
		const startingWidth = panelWidth;
		let latestWidth = startingWidth;
		startDrag(event, {
			move: (deltaX) => {
				latestWidth = Math.min(520, Math.max(220, startingWidth - deltaX));
				workbenchRef.current?.style.setProperty('--canvas-workbench-width', `${latestWidth}px`);
			},
			end: () => setPanelWidth(latestWidth),
		});
	};
	if (!selectedNode) return aiSessionDialogOpen && workspaceNode ? <AISessionLaunchDialog workspaceTarget={{ workspaceId: workspaceNode.id }} embeddedLaunchGuards={{ expectedWorkspaceId: projection.layout.workspaceId, expectedBranch: workspaceNode.branch || projection.layout.branchKey, observedCommit: workspaceNode.commit, idempotencyKey: newIdempotencyKey() }} onClose={() => onAISessionDialogOpenChange?.(false)} onLaunched={sessionLaunched} /> : null;

	return <aside ref={workbenchRef} className={collapsed ? 'canvas-workbench open collapsed' : 'canvas-workbench open'} style={{ '--canvas-workbench-width': `${panelWidth}px` } as React.CSSProperties} aria-label="Canvas Workbench">
		<header className="panel-header"><h2><Info size={16} /> Workbench</h2><div className="canvas-workbench-header-actions">{!collapsed && selectedNode && (selectedNode.workspace || selectedNode.plan) && onOpenFullView && <button className="primary canvas-workbench-view-details" type="button" onClick={() => onOpenFullView(selectedNode)}>View details</button>}<button type="button" className="icon-button" aria-label={collapsed ? 'Expand workbench' : 'Collapse workbench'} title={collapsed ? 'Expand workbench' : 'Collapse workbench'} onClick={() => setCollapsed((value) => !value)}>{collapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}</button></div></header>
		{!collapsed && <>
		{selectedNode && selectedNode.state !== 'resolved' && <section className="canvas-workbench-section canvas-reference-warning" role="status"><h2>{selectedNode.state === 'forbidden' ? 'Access restricted' : 'Reference unavailable'}</h2><p>{selectedNode.state === 'forbidden' ? 'You no longer have permission to view this entity. Cached titles are hidden.' : 'The entity no longer resolves on this workspace branch. Its placement is retained for recovery.'}</p><button type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh reference</button></section>}
		{selectedNode?.workspace && rightPanelTab === 'info' && <section className="canvas-workbench-section"><h2>Git summary</h2><dl><dt>Branch</dt><dd>{selectedNode.workspace.branch || 'Unavailable'}</dd><dt>HEAD</dt><dd><code>{shortSHA(selectedNode.workspace.commit)}</code></dd><dt>Working tree</dt><dd>{selectedNode.workspace.git?.conflicted ? 'Conflicted' : selectedNode.workspace.git?.dirty ? `${selectedNode.workspace.git.changes.length} changed files` : 'Clean'}</dd></dl><button className="secondary" type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh Git status</button>{launchCapability && launchCapability.state !== 'available' && <p className="canvas-capability-message">{launchCapability.message}</p>}</section>}
		{selectedNode?.workspace && verificationContext && rightPanelTab === 'info' && <section className="canvas-workbench-section canvas-verification"><h2>Verification</h2>{verificationJob ?? verificationContext.verification ? <VerificationSummary job={verificationJob ?? verificationContext.verification!} /> : <p>No verification result in this application run.</p>}<button className="primary" type="button" disabled={verificationCapability?.state !== 'available' || verifying} title={verificationCapability?.state !== 'available' ? verificationCapability?.message : undefined} onClick={() => void runVerification('smoke')}><Play size={14} /> {verifying ? 'Verifying…' : 'Run smoke verification'}</button></section>}
		{selectedNode?.plan && <section className="canvas-item-panel" aria-label="Workbench"><div className="side-panel-tabs" role="tablist" aria-label="Workbench tabs"><button type="button" className={rightPanelTab === 'info' ? 'active' : ''} aria-selected={rightPanelTab === 'info'} onClick={() => setRightPanelTab('info')}><Info size={14} /> Info</button><button type="button" className={rightPanelTab === 'jira' ? 'active' : ''} aria-selected={rightPanelTab === 'jira'} onClick={() => setRightPanelTab('jira')}><Ticket size={14} /> Jira</button><button type="button" className={rightPanelTab === 'quality' ? 'active' : ''} aria-selected={rightPanelTab === 'quality'} onClick={() => setRightPanelTab('quality')}><GitCompare size={14} /> Quality</button></div>{rightPanelTab === 'info' && <CanvasItemInfo itemId={selectedNode.plan.itemId} node={selectedNode} workspace={verificationContext} onSaved={() => void onReload()} />}{rightPanelTab === 'jira' && <JiraItemPanel itemId={selectedNode.plan.itemId} />}{rightPanelTab === 'quality' && <CanvasQualityPanel itemId={selectedNode.plan.itemId} workspaceId={verificationContext?.id} capability={verificationCapability} job={verificationJob ?? verificationContext?.verification} verifying={verifying} runbooks={e2eRunbooks} onRun={runVerification} onRunAutomation={runAutomationVerification} onRerun={rerunVerification} onRefresh={() => void refreshE2ERunbooks()} />}</section>}
		{selectedNode?.session && <section className="canvas-item-panel" aria-label="Workbench"><div className="side-panel-tabs" role="tablist" aria-label="Workbench tabs"><button type="button" className={rightPanelTab === 'terminal' ? 'active' : ''} aria-selected={rightPanelTab === 'terminal'} onClick={() => setRightPanelTab('terminal')}><TerminalSquare size={14} /> Terminal</button><button type="button" className={rightPanelTab === 'changes' ? 'active' : ''} aria-selected={rightPanelTab === 'changes'} onClick={() => setRightPanelTab('changes')}><GitCompare size={14} /> Changes</button></div>{rightPanelTab === 'terminal' && <SessionTerminalSummary record={selectedNode.session.record} onReload={onReload} />}{rightPanelTab === 'changes' && <SessionChangesPanel session={selectedNode.session.record} />}</section>}
		{error && <p className="canvas-workbench-error" role="alert">{error}</p>}
		<button className="panel-resize-handle panel-resize-handle-right canvas-workbench-resize-handle" type="button" aria-label="Resize Workbench panel" onPointerDown={startResize}><GripVertical size={16} /></button>
		</>}
		{aiSessionDialogOpen && (selectedNode.plan || selectedNode.workspace) && <AISessionLaunchDialog itemId={selectedNode.plan?.itemId} workspaceTarget={selectedNode.workspace ? { workspaceId: selectedNode.workspace.id } : undefined} embeddedLaunchGuards={{ expectedWorkspaceId: projection.layout.workspaceId, expectedBranch: selectedNode.plan?.branch || selectedNode.workspace?.branch || projection.layout.branchKey, observedCommit: selectedNode.plan?.commit || selectedNode.workspace?.commit || selectedNode.entityRef.observedCommit, idempotencyKey: newIdempotencyKey() }} onClose={() => onAISessionDialogOpenChange?.(false)} onLaunched={sessionLaunched} />}
	</aside>;
}

function abortableDelay(milliseconds: number, signal: AbortSignal) {
	if (signal.aborted) return Promise.resolve(false);
	return new Promise<boolean>((resolve) => {
		let settled = false;
		let timer = 0;
		const finish = (completed: boolean) => {
			if (settled) return;
			settled = true;
			window.clearTimeout(timer);
			signal.removeEventListener('abort', abort);
			resolve(completed);
		};
		const abort = () => finish(false);
		timer = window.setTimeout(() => finish(true), milliseconds);
		signal.addEventListener('abort', abort, { once: true });
	});
}
function newIdempotencyKey() { return globalThis.crypto?.randomUUID?.() ?? `canvas-${Date.now()}-${Math.random().toString(16).slice(2)}`; }
