import { useCallback, useEffect, useRef, useState } from 'react';
import { useHorizontalDrag } from '../../components/useHorizontalDrag';
import { GitCompare, GripVertical, Info, PanelRightClose, PanelRightOpen, Play, RefreshCw, TerminalSquare, Ticket } from 'lucide-react';
import { api } from '../../shared/api';
import type { AISessionLaunchResult, AutomationDisplayMode, CanvasNode, CanvasProjection, E2ERunbook, EmbeddedAISessionResult, ItemDetail, ItemMetadataUpdateInput, ItemStatus, ItemVerificationTests, SafeSessionRecord, VerificationJob, VerificationTestSelection, VerifyProfile } from '../../lib/types';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';
import { E2EQualityPanel } from '../e2e-testing/E2EQualityPanel';
import { JiraItemPanel } from '../jira/JiraItemPanel';
import { StatusMenu } from '../../components/StatusMenu';
import { SessionChangesPanel } from '../sessions/SessionChangesPanel';
import { formatSessionDate, isSessionLive, SessionConfirmDialog } from '../sessions/sessionDisplay';

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

function VerificationSummary({ job }: { job: VerificationJob }) {
	const freshness = job.freshness || 'inconclusive';
	const historicalPass = job.status === 'passed' && freshness !== 'fresh';
	return <dl className={`verification-summary result-${job.status} freshness-${freshness}`}><dt>Result</dt><dd>{historicalPass ? 'Passed (historical)' : job.status}</dd><dt>Freshness</dt><dd>{freshness}</dd><dt>Verified revision</dt><dd><code>{shortSHA(job.finishFingerprint?.commit || job.startFingerprint?.commit)}</code></dd><dt>Current revision</dt><dd><code>{shortSHA(job.currentFingerprint?.commit)}</code></dd></dl>;
}

function CanvasItemInfo({ itemId, node, workspace, onSaved }: { itemId: string; node: CanvasNode; workspace?: CanvasNode['workspace']; onSaved: () => void }) {
	const [detail, setDetail] = useState<ItemDetail>();
	const [draft, setDraft] = useState<ItemMetadataUpdateInput>({});
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		const controller = new AbortController();
		api.item(itemId, controller.signal).then((next) => {
			if (controller.signal.aborted) return;
			setDetail(next);
			setDraft({ title: next.title, scope: next.scope, identifier: next.identifier, status: next.status, owner: next.owner ?? '', tags: next.tags });
		}).catch((caught) => !controller.signal.aborted && setError(caught instanceof Error ? caught.message : 'Item details could not be loaded.'));
		return () => {
			controller.abort();
			if (generationRef.current === generation) generationRef.current += 1;
		};
	}, [itemId]);
	const canvasPlan = node.plan;
	if (!canvasPlan) return null;
	const dirty = detail ? (draft.title !== detail.title || draft.scope !== detail.scope || draft.identifier !== detail.identifier || draft.status !== detail.status || draft.owner !== (detail.owner ?? '') || (draft.tags ?? []).join('\n') !== detail.tags.join('\n')) : false;
	const save = async () => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setSaving(true); setError('');
		try {
			const result = await api.saveMetadata(itemId, { ...draft, expectedRevision: detail?.metadataRevision });
			if (!isCurrent()) return;
			setDetail(result.item);
			setDraft({ title: result.item.title, scope: result.item.scope, identifier: result.item.identifier, status: result.item.status, owner: result.item.owner ?? '', tags: result.item.tags });
			onSaved();
		} catch (caught) { if (isCurrent()) setError(caught instanceof Error ? caught.message : 'Metadata could not be saved.'); } finally { if (isCurrent()) setSaving(false); }
	};
	const loaded = detail;
	return <div className="canvas-item-tab-content"><dl><dt>Workspace</dt><dd>{loaded?.workspaceName || workspace?.name || 'Unavailable'}</dd><dt>Source</dt><dd>{loaded?.scope || canvasPlan.service || 'Other'}</dd><dt>Item</dt><dd>{loaded?.identifier || canvasPlan.identifier}</dd><dt>Branch</dt><dd>{loaded?.branch || canvasPlan.branch || node.entityRef.branchKey || 'Unavailable'}</dd><dt>Status</dt><dd><span className={`canvas-item-status status-${loaded?.status || canvasPlan.status}`}>{(loaded?.status || canvasPlan.status).replaceAll('_', ' ')}</span></dd><dt>Metadata</dt><dd>{loaded?.metadataSource || 'Item'}</dd><dt>Author</dt><dd>{loaded?.author || loaded?.owner || 'Unknown'}</dd><dt>Files</dt><dd>{loaded?.counts.files ?? '—'}</dd></dl>{loaded && <div className="metadata-form canvas-metadata-form"><label>Title<input value={draft.title ?? ''} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} /></label><label>Source<input value={draft.scope ?? ''} onChange={(event) => setDraft((current) => ({ ...current, scope: event.target.value }))} /></label><label>Item<input value={draft.identifier ?? ''} onChange={(event) => setDraft((current) => ({ ...current, identifier: event.target.value }))} /></label><label>Status<StatusMenu value={(draft.status ?? 'draft') as ItemStatus} onChange={(status) => setDraft((current) => ({ ...current, status }))} /></label><label>Owner<input value={draft.owner ?? ''} onChange={(event) => setDraft((current) => ({ ...current, owner: event.target.value }))} /></label><label>Tags<input value={(draft.tags ?? []).join(', ')} onChange={(event) => setDraft((current) => ({ ...current, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label><button className="save-action save-metadata-action" type="button" disabled={!dirty || saving} onClick={() => void save()}>{saving ? 'Saving...' : 'Save Metadata'}</button><div className="tags">{loaded.tags.map((tag) => <span key={tag}>{tag}</span>)}</div></div>}{error && <p className="error" role="alert">{error}</p>}</div>;
}

function CanvasQualityPanel({ itemId, workspaceId, capability, job, verifying, runbooks, onRun, onRunAutomation, onRerun, onRefresh }: { itemId: string; workspaceId?: string; capability?: { state: string; message?: string }; job?: VerificationJob; verifying: boolean; runbooks: { runbooks: E2ERunbook[]; diagnostic?: string }; onRun: (profile: VerifyProfile) => void; onRunAutomation: (input: { environment: string; displayMode: AutomationDisplayMode; selectedSpecs: string[] }) => void; onRerun: () => void; onRefresh: () => void }) {
	const canVerify = capability?.state === 'available';
	const [tests, setTests] = useState<ItemVerificationTests>();
	const [manualSpec, setManualSpec] = useState('');
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		const controller = new AbortController();
		const load = api.itemVerificationTests;
		if (typeof load !== 'function') return;
		load(itemId, controller.signal).then((value) => !controller.signal.aborted && setTests(value)).catch((caught) => !controller.signal.aborted && setError(caught instanceof Error ? caught.message : 'Automation settings could not be loaded.'));
		return () => {
			controller.abort();
			if (generationRef.current === generation) generationRef.current += 1;
		};
	}, [itemId]);
	const selectedSpecs = tests?.selection.selectedSpecs ?? [];
	const environment = tests?.selection.environment || 'local';
	const displayMode: AutomationDisplayMode = tests?.selection.displayMode === 'visible' ? 'visible' : 'silent';
	const saveSelection = async (selection: VerificationTestSelection) => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setBusy(true); setError('');
		try {
			const next = await api.saveItemVerificationTests(itemId, { ...selection, expectedRevision: tests?.selection.revision });
			if (isCurrent()) setTests(next);
		}
		catch (caught) { if (isCurrent()) setError(caught instanceof Error ? caught.message : 'Automation settings could not be saved.'); }
		finally { if (isCurrent()) setBusy(false); }
	};
	const updateSpecs = (specs: string[]) => void saveSelection({ selectedSpecs: specs, environment, displayMode });
	const addManualSpec = () => { const spec = manualSpec.trim(); if (!spec || selectedSpecs.includes(spec)) return; updateSpecs([...selectedSpecs, spec]); setManualSpec(''); };
	return <section className="metadata-callout quality-panel canvas-quality-panel" aria-label="Quality"><div className="verification-header"><strong>Quality</strong>{job && <span className={`verification-trigger-badge ${job.status === 'passed' ? 'success' : job.status === 'failed' ? 'danger' : ''}`}>{job.profile ?? job.mode} · {job.status}</span>}</div><div className="verification-actions"><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('smoke')}>Run smoke verify</button><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('critical')}>Run critical verify</button><button className="secondary" type="button" disabled={!job || verifying} onClick={onRerun}>Re-run latest</button></div>{!workspaceId && <span className="verification-note">No workspace selected.</span>}{capability && !canVerify && <span className="verification-note">{capability.message || 'Verification is unavailable for this workspace.'}</span>}{workspaceId && <div className="automation-test-panel"><div className="quality-section-heading"><strong>Automation</strong><span>Environment and card-linked specs</span></div><label className="repo-field">Automation environment<input value={environment} onChange={(event) => setTests((current) => ({ selection: { selectedSpecs, environment: event.target.value, displayMode }, discoveredSpecs: current?.discoveredSpecs ?? [] }))} onBlur={() => void saveSelection({ selectedSpecs, environment, displayMode })} placeholder="local" /></label><div className="automation-mode-field"><span>Run mode</span><div className="segmented-control automation-mode-toggle" role="tablist" aria-label="Automation run mode"><button type="button" className={displayMode === 'silent' ? 'active' : ''} aria-selected={displayMode === 'silent'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'silent' })}>Silent</button><button type="button" className={displayMode === 'visible' ? 'active' : ''} aria-selected={displayMode === 'visible'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'visible' })}>Visible browser</button></div></div><div className="quality-section-heading compact"><strong>Selected specs</strong></div><div className="automation-spec-list" aria-label="Selected automation specs">{selectedSpecs.length ? selectedSpecs.map((spec) => <span className="automation-spec-chip" key={spec}>{spec}<button type="button" aria-label={`Remove ${spec}`} onClick={() => updateSpecs(selectedSpecs.filter((candidate) => candidate !== spec))}>×</button></span>) : <span className="verification-note">No selected specs.</span>}</div><div className="path-input-row manual-spec-row"><input aria-label="Manual automation spec" value={manualSpec} onChange={(event) => setManualSpec(event.target.value)} placeholder="Repo-relative spec path" /><button className="secondary" type="button" onClick={addManualSpec} disabled={!manualSpec.trim() || busy}>Add spec</button></div><div className="automation-action-row"><button className="secondary automation-run-action" type="button" onClick={() => onRunAutomation({ environment, displayMode, selectedSpecs })} disabled={!canVerify || verifying || busy || selectedSpecs.length === 0}>Run automation tests</button></div><div className="automation-suggestions"><div className="quality-subsection-heading"><strong>Suggested specs</strong><span>From automation-test in plan.yaml</span></div>{tests?.discoveredSpecs?.length ? <div className="automation-discovered-specs" aria-label="Suggested automation specs">{tests.discoveredSpecs.map((spec) => { const selected = selectedSpecs.includes(spec.path); return <article className={selected ? 'automation-suggestion-card selected' : 'automation-suggestion-card'} key={`${spec.path}-${spec.sourcePath ?? ''}`}><div><strong>{spec.path}</strong><span>{spec.runner}{spec.sourcePath ? ` · ${spec.sourcePath}` : ''}</span></div><button className="secondary" type="button" disabled={selected || busy} onClick={() => updateSpecs([...selectedSpecs, spec.path])}>{selected ? 'Selected' : 'Select'}</button></article>; })}</div> : <span className="verification-note">No suggested specs from plan.yaml.</span>}</div></div>}{verifying && <span className="verification-note" role="status">Starting verification...</span>}{job && <><VerificationSummary job={job} />{job.steps?.length ? <div className="verification-steps">{job.steps.map((step) => <span className={`verification-step ${step.status === 'ok' ? 'ok' : 'failed'}`} key={`${step.step}-${step.at}`}>{step.step}: {step.status}</span>)}</div> : null}</>}{workspaceId && <E2EQualityPanel variant="nested" workspaceId={workspaceId} runbooks={runbooks.runbooks} diagnostic={runbooks.diagnostic} onRefresh={onRefresh} />}{error && <p className="error" role="alert">{error}</p>}</section>;
}

/** Session node Terminal tab: the durable session summary and Stop; the live terminal itself stays in the Canvas node, never duplicated here. */
function SessionTerminalSummary({ record, onReload }: { record: SafeSessionRecord; onReload: () => Promise<unknown> | void }) {
	const [confirming, setConfirming] = useState(false);
	const [stopping, setStopping] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		return () => { if (generationRef.current === generation) generationRef.current += 1; };
	}, [record.id]);
	const live = isSessionLive(record);
	const stop = async () => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setStopping(true);
		setError('');
		try {
			await api.cancelEmbeddedAISession(record.id);
			await Promise.resolve(onReload());
			if (!isCurrent()) return;
			setConfirming(false);
		} catch (caught) {
			if (isCurrent()) setError(caught instanceof Error ? caught.message : 'The session could not be stopped.');
		} finally {
			if (isCurrent()) setStopping(false);
		}
	};
	return <div className="canvas-item-tab-content canvas-session-summary">
		<dl>
			<dt>Provider</dt><dd>{record.provider}</dd>
			<dt>Branch</dt><dd>{record.requestedBranch}</dd>
			<dt>State</dt><dd className={`canvas-session-state state-${record.state}`}>{record.state}</dd>
			<dt>Started</dt><dd>{formatSessionDate(record.startedAt)}</dd>
			{record.exitCode !== undefined && <><dt>Exit code</dt><dd>{record.exitCode}</dd></>}
		</dl>
		{live && <button className="danger-confirm" type="button" onClick={() => setConfirming(true)}>{`Stop ${record.provider} session`}</button>}
		{error && !confirming && <p className="error" role="alert">{error}</p>}
		{confirming && <SessionConfirmDialog title="Stop session?" message={`Stop the ${record.provider} session on ${record.requestedBranch}? Its durable session metadata will remain available.`} confirmLabel="Stop session" busyLabel="Stopping…" busy={stopping} error={error} onCancel={() => setConfirming(false)} onConfirm={() => void stop()} />}
	</div>;
}

function shortSHA(value?: string) { return value ? value.slice(0, 8) : 'Unavailable'; }
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
