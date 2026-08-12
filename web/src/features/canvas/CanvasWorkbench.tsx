import { useCallback, useEffect, useRef, useState } from 'react';
import { GitCompare, GripVertical, Info, PanelRightClose, PanelRightOpen, Play, RefreshCw, Ticket } from 'lucide-react';
import { api } from '../../shared/api';
import type { AISessionLaunchResult, AutomationDisplayMode, CanvasNode, CanvasProjection, E2ERunbook, EmbeddedAISessionResult, ItemDetail, ItemMetadataUpdateInput, ItemStatus, ItemVerificationTests, VerificationJob, VerificationTestSelection, VerifyProfile } from '../../lib/types';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';
import { E2EQualityPanel } from '../e2e-testing/E2EQualityPanel';
import { JiraItemPanel } from '../jira/JiraItemPanel';
import { StatusMenu } from '../../components/StatusMenu';

export function CanvasWorkbench({ projection, selectedNode, aiSessionDialogOpen = false, onAISessionDialogOpenChange, onClose, onReload, onSelectNode, onPlaceSession, onOpenFullView }: { projection: CanvasProjection; selectedNode?: CanvasNode; aiSessionDialogOpen?: boolean; onAISessionDialogOpenChange?: (open: boolean) => void; onClose: () => void; onReload: () => Promise<unknown> | void; onSelectNode: (id: string) => void; onPlaceSession: (sessionId: string) => Promise<unknown> | void; onOpenFullView?: (node: CanvasNode) => void }) {
	const [rightPanelTab, setRightPanelTab] = useState<'info' | 'jira' | 'quality'>('info');
	const [collapsed, setCollapsed] = useState(false);
	const [panelWidth, setPanelWidth] = useState(410);
	const [verifying, setVerifying] = useState(false);
	const [verificationJob, setVerificationJob] = useState<VerificationJob>();
	const [e2eRunbooks, setE2ERunbooks] = useState<{ runbooks: E2ERunbook[]; diagnostic?: string }>({ runbooks: [] });
	const [error, setError] = useState('');
	const pendingKey = useRef('');
	const verificationPollRef = useRef(0);
	const workbenchRef = useRef<HTMLElement>(null);
	const workspaceNode = projection.nodes.find((node) => node.kind === 'workspace')?.workspace;

	const launchCapability = selectedNode?.plan?.actions['terminal.launch'] ?? selectedNode?.workspace?.actions['terminal.launch'];
	const verificationContext = selectedNode?.workspace ?? (selectedNode?.plan ? workspaceNode : undefined);
	const verificationCapability = selectedNode?.plan?.actions['verification.run'] ?? verificationContext?.actions['verification.run'];
	const itemId = selectedNode?.plan?.itemId;

	const refreshE2ERunbooks = useCallback(async () => {
		if (!itemId) {
			setE2ERunbooks({ runbooks: [] });
			return;
		}
		try {
			setE2ERunbooks(await api.itemE2ERunbooks(itemId));
		} catch (caught) {
			setE2ERunbooks({ runbooks: [], diagnostic: caught instanceof Error ? caught.message : 'E2E coverage could not be loaded.' });
		}
	}, [itemId]);

	useEffect(() => {
		verificationPollRef.current += 1;
		setRightPanelTab('info');
		setCollapsed(false);
		setVerificationJob(undefined);
		void refreshE2ERunbooks();
		return () => { verificationPollRef.current += 1; };
	}, [itemId, refreshE2ERunbooks]);

	const waitForVerification = async (job: VerificationJob, pollID: number) => {
		let current = job;
		for (let attempt = 0; attempt < 60 && (current.status === 'queued' || current.status === 'running'); attempt += 1) {
			await new Promise((resolve) => setTimeout(resolve, 1000));
			if (verificationPollRef.current !== pollID) return;
			current = await api.verificationJob(verificationContext!.id, current.id);
			if (verificationPollRef.current !== pollID) return;
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
			await Promise.resolve(onPlaceSession(result.session.id));
			onSelectNode(`session:${result.session.id}`);
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'AI session could not be added to the canvas.');
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
		const pollID = ++verificationPollRef.current;
		try {
			let job = await api.createVerificationJob(verificationContext.id, { profile, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
			setVerificationJob(job);
			await Promise.resolve(onReload());
			await waitForVerification(job, pollID);
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Verification could not run.');
		} finally {
			if (verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const rerunVerification = async () => {
		const job = verificationJob ?? verificationContext?.verification;
		if (!verificationContext || !job || verifying) return;
		setVerifying(true);
		setError('');
		const pollID = ++verificationPollRef.current;
		try {
			const next = await api.rerunVerificationJob(verificationContext.id, job.id);
			setVerificationJob(next);
			await Promise.resolve(onReload());
			await waitForVerification(next, pollID);
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Verification could not be rerun.');
		} finally {
			if (verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const runAutomationVerification = async (input: { environment: string; displayMode: AutomationDisplayMode; selectedSpecs: string[] }) => {
		if (!verificationContext || verificationCapability?.state !== 'available' || verifying || input.selectedSpecs.length === 0) return;
		setVerifying(true);
		setError('');
		const pollID = ++verificationPollRef.current;
		try {
			const job = await api.createVerificationJob(verificationContext.id, { mode: 'automation', ...input, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
			setVerificationJob(job);
			await Promise.resolve(onReload());
			await waitForVerification(job, pollID);
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Automation verification could not run.');
		} finally {
			if (verificationPollRef.current === pollID) setVerifying(false);
		}
	};
	const startResize = (event: React.PointerEvent<HTMLButtonElement>) => {
		event.preventDefault();
		const startX = event.clientX;
		const startingWidth = panelWidth;
		let latestWidth = startingWidth;
		const onPointerMove = (moveEvent: PointerEvent) => {
			const nextWidth = Math.min(520, Math.max(220, startingWidth - (moveEvent.clientX - startX)));
			latestWidth = nextWidth;
			workbenchRef.current?.style.setProperty('--canvas-workbench-width', `${nextWidth}px`);
		};
		const onPointerUp = () => {
			document.body.classList.remove('is-resizing-panel');
			window.removeEventListener('pointermove', onPointerMove);
			window.removeEventListener('pointerup', onPointerUp);
			setPanelWidth(latestWidth);
		};
		document.body.classList.add('is-resizing-panel');
		window.addEventListener('pointermove', onPointerMove);
		window.addEventListener('pointerup', onPointerUp);
	};
	if (!selectedNode) return aiSessionDialogOpen && workspaceNode ? <AISessionLaunchDialog workspaceTarget={{ workspaceId: workspaceNode.id }} embeddedLaunchGuards={{ expectedWorkspaceId: projection.layout.workspaceId, expectedBranch: workspaceNode.branch || projection.layout.branchKey, observedCommit: workspaceNode.commit, idempotencyKey: newIdempotencyKey() }} onClose={() => onAISessionDialogOpenChange?.(false)} onLaunched={sessionLaunched} /> : null;

	return <aside ref={workbenchRef} className={collapsed ? 'canvas-workbench open collapsed' : 'canvas-workbench open'} style={{ '--canvas-workbench-width': `${panelWidth}px` } as React.CSSProperties} aria-label="Canvas Workbench">
		<header className="panel-header"><h2><Info size={16} /> Workbench</h2><div className="canvas-workbench-header-actions">{!collapsed && selectedNode && (selectedNode.workspace || selectedNode.plan) && onOpenFullView && <button className="primary canvas-workbench-view-details" type="button" onClick={() => onOpenFullView(selectedNode)}>View details</button>}<button type="button" className="icon-button" aria-label={collapsed ? 'Expand workbench' : 'Collapse workbench'} title={collapsed ? 'Expand workbench' : 'Collapse workbench'} onClick={() => setCollapsed((value) => !value)}>{collapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}</button></div></header>
		{!collapsed && <>
		{selectedNode && selectedNode.state !== 'resolved' && <section className="canvas-workbench-section canvas-reference-warning" role="status"><h2>{selectedNode.state === 'forbidden' ? 'Access restricted' : 'Reference unavailable'}</h2><p>{selectedNode.state === 'forbidden' ? 'You no longer have permission to view this entity. Cached titles are hidden.' : 'The entity no longer resolves on this workspace branch. Its placement is retained for recovery.'}</p><button type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh reference</button></section>}
		{selectedNode?.workspace && <section className="canvas-workbench-section"><h2>Git summary</h2><dl><dt>Branch</dt><dd>{selectedNode.workspace.branch || 'Unavailable'}</dd><dt>HEAD</dt><dd><code>{shortSHA(selectedNode.workspace.commit)}</code></dd><dt>Working tree</dt><dd>{selectedNode.workspace.git?.conflicted ? 'Conflicted' : selectedNode.workspace.git?.dirty ? `${selectedNode.workspace.git.changes.length} changed files` : 'Clean'}</dd></dl><button className="secondary" type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh Git status</button>{launchCapability && launchCapability.state !== 'available' && <p className="canvas-capability-message">{launchCapability.message}</p>}</section>}
		{selectedNode?.workspace && verificationContext && <section className="canvas-workbench-section canvas-verification"><h2>Verification</h2>{verificationJob ?? verificationContext.verification ? <VerificationSummary job={verificationJob ?? verificationContext.verification!} /> : <p>No verification result in this application run.</p>}<button className="primary" type="button" disabled={verificationCapability?.state !== 'available' || verifying} title={verificationCapability?.state !== 'available' ? verificationCapability?.message : undefined} onClick={() => void runVerification('smoke')}><Play size={14} /> {verifying ? 'Verifying…' : 'Run smoke verification'}</button></section>}
		{selectedNode?.plan && <section className="canvas-item-panel" aria-label="Workbench"><div className="side-panel-tabs" role="tablist" aria-label="Workbench tabs"><button type="button" className={rightPanelTab === 'info' ? 'active' : ''} aria-selected={rightPanelTab === 'info'} onClick={() => setRightPanelTab('info')}><Info size={14} /> Info</button><button type="button" className={rightPanelTab === 'jira' ? 'active' : ''} aria-selected={rightPanelTab === 'jira'} onClick={() => setRightPanelTab('jira')}><Ticket size={14} /> Jira</button><button type="button" className={rightPanelTab === 'quality' ? 'active' : ''} aria-selected={rightPanelTab === 'quality'} onClick={() => setRightPanelTab('quality')}><GitCompare size={14} /> Quality</button></div>{rightPanelTab === 'info' && <CanvasItemInfo itemId={selectedNode.plan.itemId} node={selectedNode} workspace={verificationContext} onSaved={() => void onReload()} />}{rightPanelTab === 'jira' && <JiraItemPanel itemId={selectedNode.plan.itemId} />}{rightPanelTab === 'quality' && <CanvasQualityPanel itemId={selectedNode.plan.itemId} workspaceId={verificationContext?.id} capability={verificationCapability} job={verificationJob ?? verificationContext?.verification} verifying={verifying} runbooks={e2eRunbooks} onRun={runVerification} onRunAutomation={runAutomationVerification} onRerun={rerunVerification} onRefresh={() => void refreshE2ERunbooks()} />}</section>}
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
	useEffect(() => {
		let active = true;
		api.item(itemId).then((next) => {
			if (!active) return;
			setDetail(next);
			setDraft({ title: next.title, scope: next.scope, identifier: next.identifier, status: next.status, owner: next.owner ?? '', tags: next.tags });
		}).catch((caught) => active && setError(caught instanceof Error ? caught.message : 'Item details could not be loaded.'));
		return () => { active = false; };
	}, [itemId]);
	const canvasPlan = node.plan;
	if (!canvasPlan) return null;
	const dirty = detail ? (draft.title !== detail.title || draft.scope !== detail.scope || draft.identifier !== detail.identifier || draft.status !== detail.status || draft.owner !== (detail.owner ?? '') || (draft.tags ?? []).join('\n') !== detail.tags.join('\n')) : false;
	const save = async () => {
		setSaving(true); setError('');
		try {
			const result = await api.saveMetadata(itemId, { ...draft, expectedRevision: detail?.metadataRevision });
			setDetail(result.item);
			setDraft({ title: result.item.title, scope: result.item.scope, identifier: result.item.identifier, status: result.item.status, owner: result.item.owner ?? '', tags: result.item.tags });
			onSaved();
		} catch (caught) { setError(caught instanceof Error ? caught.message : 'Metadata could not be saved.'); } finally { setSaving(false); }
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
	useEffect(() => {
		let active = true;
		const load = api.itemVerificationTests;
		if (typeof load !== 'function') return;
		load(itemId).then((value) => active && setTests(value)).catch((caught) => active && setError(caught instanceof Error ? caught.message : 'Automation settings could not be loaded.'));
		return () => { active = false; };
	}, [itemId]);
	const selectedSpecs = tests?.selection.selectedSpecs ?? [];
	const environment = tests?.selection.environment || 'local';
	const displayMode: AutomationDisplayMode = tests?.selection.displayMode === 'visible' ? 'visible' : 'silent';
	const saveSelection = async (selection: VerificationTestSelection) => {
		setBusy(true); setError('');
		try { setTests(await api.saveItemVerificationTests(itemId, { ...selection, expectedRevision: tests?.selection.revision })); }
		catch (caught) { setError(caught instanceof Error ? caught.message : 'Automation settings could not be saved.'); }
		finally { setBusy(false); }
	};
	const updateSpecs = (specs: string[]) => void saveSelection({ selectedSpecs: specs, environment, displayMode });
	const addManualSpec = () => { const spec = manualSpec.trim(); if (!spec || selectedSpecs.includes(spec)) return; updateSpecs([...selectedSpecs, spec]); setManualSpec(''); };
	return <section className="metadata-callout quality-panel canvas-quality-panel" aria-label="Quality"><div className="verification-header"><strong>Quality</strong>{job && <span className={`verification-trigger-badge ${job.status === 'passed' ? 'success' : job.status === 'failed' ? 'danger' : ''}`}>{job.profile ?? job.mode} · {job.status}</span>}</div><div className="verification-actions"><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('smoke')}>Run smoke verify</button><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('critical')}>Run critical verify</button><button className="secondary" type="button" disabled={!job || verifying} onClick={onRerun}>Re-run latest</button></div>{!workspaceId && <span className="verification-note">No workspace selected.</span>}{capability && !canVerify && <span className="verification-note">{capability.message || 'Verification is unavailable for this workspace.'}</span>}{workspaceId && <div className="automation-test-panel"><div className="quality-section-heading"><strong>Automation</strong><span>Environment and card-linked specs</span></div><label className="repo-field">Automation environment<input value={environment} onChange={(event) => setTests((current) => ({ selection: { selectedSpecs, environment: event.target.value, displayMode }, discoveredSpecs: current?.discoveredSpecs ?? [] }))} onBlur={() => void saveSelection({ selectedSpecs, environment, displayMode })} placeholder="local" /></label><div className="automation-mode-field"><span>Run mode</span><div className="segmented-control automation-mode-toggle" role="tablist" aria-label="Automation run mode"><button type="button" className={displayMode === 'silent' ? 'active' : ''} aria-selected={displayMode === 'silent'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'silent' })}>Silent</button><button type="button" className={displayMode === 'visible' ? 'active' : ''} aria-selected={displayMode === 'visible'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'visible' })}>Visible browser</button></div></div><div className="quality-section-heading compact"><strong>Selected specs</strong></div><div className="automation-spec-list" aria-label="Selected automation specs">{selectedSpecs.length ? selectedSpecs.map((spec) => <span className="automation-spec-chip" key={spec}>{spec}<button type="button" aria-label={`Remove ${spec}`} onClick={() => updateSpecs(selectedSpecs.filter((candidate) => candidate !== spec))}>×</button></span>) : <span className="verification-note">No selected specs.</span>}</div><div className="path-input-row manual-spec-row"><input aria-label="Manual automation spec" value={manualSpec} onChange={(event) => setManualSpec(event.target.value)} placeholder="Repo-relative spec path" /><button className="secondary" type="button" onClick={addManualSpec} disabled={!manualSpec.trim() || busy}>Add spec</button></div><div className="automation-action-row"><button className="secondary automation-run-action" type="button" onClick={() => onRunAutomation({ environment, displayMode, selectedSpecs })} disabled={!canVerify || verifying || busy || selectedSpecs.length === 0}>Run automation tests</button></div><div className="automation-suggestions"><div className="quality-subsection-heading"><strong>Suggested specs</strong><span>From automation-test in plan.yaml</span></div>{tests?.discoveredSpecs?.length ? <div className="automation-discovered-specs" aria-label="Suggested automation specs">{tests.discoveredSpecs.map((spec) => { const selected = selectedSpecs.includes(spec.path); return <article className={selected ? 'automation-suggestion-card selected' : 'automation-suggestion-card'} key={`${spec.path}-${spec.sourcePath ?? ''}`}><div><strong>{spec.path}</strong><span>{spec.runner}{spec.sourcePath ? ` · ${spec.sourcePath}` : ''}</span></div><button className="secondary" type="button" disabled={selected || busy} onClick={() => updateSpecs([...selectedSpecs, spec.path])}>{selected ? 'Selected' : 'Select'}</button></article>; })}</div> : <span className="verification-note">No suggested specs from plan.yaml.</span>}</div></div>}{verifying && <span className="verification-note" role="status">Starting verification...</span>}{job && <><VerificationSummary job={job} />{job.steps?.length ? <div className="verification-steps">{job.steps.map((step) => <span className={`verification-step ${step.status === 'ok' ? 'ok' : 'failed'}`} key={`${step.step}-${step.at}`}>{step.step}: {step.status}</span>)}</div> : null}</>}{workspaceId && <E2EQualityPanel variant="nested" workspaceId={workspaceId} runbooks={runbooks.runbooks} diagnostic={runbooks.diagnostic} onRefresh={onRefresh} />}{error && <p className="error" role="alert">{error}</p>}</section>;
}

function shortSHA(value?: string) { return value ? value.slice(0, 8) : 'Unavailable'; }
function newIdempotencyKey() { return globalThis.crypto?.randomUUID?.() ?? `canvas-${Date.now()}-${Math.random().toString(16).slice(2)}`; }
