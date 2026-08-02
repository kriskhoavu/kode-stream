import { useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, GitBranch, Play, RefreshCw, Square, TerminalSquare, X } from 'lucide-react';
import { EmbeddedTerminal } from '../ai-session/EmbeddedTerminal';
import { api, ApiError } from '../../lib/api';
import type { CanvasNode, CanvasProjection, EmbeddedAISessionResult, SafeSessionRecord, VerificationJob } from '../../lib/types';

export function CanvasWorkbench({ projection, selectedNode, onClose, onReload, onSelectNode, onPlaceUnplaced, onOpenFullView }: { projection: CanvasProjection; selectedNode?: CanvasNode; onClose: () => void; onReload: () => Promise<unknown> | void; onSelectNode: (id: string) => void; onPlaceUnplaced: () => Promise<unknown> | void; onOpenFullView?: (node: CanvasNode) => void }) {
	const [results, setResults] = useState<Record<string, EmbeddedAISessionResult>>({});
	const [records, setRecords] = useState<SafeSessionRecord[]>([]);
	const [activeTerminalId, setActiveTerminalId] = useState('');
	const [launching, setLaunching] = useState(false);
	const [verifying, setVerifying] = useState(false);
	const [error, setError] = useState('');
	const [mismatch, setMismatch] = useState<Record<string, string>>();
	const pendingKey = useRef('');
	const attaching = useRef(new Set<string>());
	const workspaceNode = projection.nodes.find((node) => node.kind === 'workspace')?.workspace;

	const refreshRecords = async () => {
		const next = await api.aiSessionRecords(projection.layout.workspaceId, projection.layout.branchKey);
		setRecords(next);
		return next;
	};
	useEffect(() => { void refreshRecords().catch(() => setRecords([])); }, [projection.layout.branchKey, projection.layout.workspaceId, projection.nodes.length]);

	useEffect(() => {
		const record = selectedNode?.session?.record;
		if (!record?.live || results[record.id] || attaching.current.has(record.id)) return;
		attaching.current.add(record.id);
		void Promise.all([api.embeddedAISession(record.id), api.embeddedAISessionGrant(record.id)]).then(([session, grant]) => {
			setResults((current) => ({ ...current, [record.id]: { session, grant, record } }));
			setActiveTerminalId(record.id);
		}).catch(() => setError('The live process could not be reattached. Refresh its session state.')).finally(() => attaching.current.delete(record.id));
	}, [results, selectedNode]);

	const placedSessionIDs = useMemo(() => new Set(projection.nodes.filter((node) => node.kind === 'session').map((node) => node.entityRef.sessionId)), [projection.nodes]);
	const unplacedActive = records.filter((record) => record.live && !placedSessionIDs.has(record.id));
	const launchCapability = selectedNode?.plan?.actions['terminal.launch'];
	const canLaunch = selectedNode?.kind === 'plan' && launchCapability?.state === 'available';
	const verificationContext = selectedNode?.workspace ?? (selectedNode?.plan ? workspaceNode : undefined);
	const verificationCapability = selectedNode?.plan?.actions['verification.run'] ?? verificationContext?.actions['verification.run'];

	const launch = async () => {
		if (!selectedNode?.plan || launching || pendingKey.current) return;
		const key = newIdempotencyKey();
		pendingKey.current = key;
		setLaunching(true);
		setError('');
		setMismatch(undefined);
		try {
			const settings = await api.aiSettings();
			if (!settings.defaultProvider) throw new Error('Choose a detected AI provider in Settings first.');
			const result = await api.startEmbeddedAISession(selectedNode.plan.itemId, {
				provider: settings.defaultProvider,
				contextMode: 'card_context',
				expectedWorkspaceId: projection.layout.workspaceId,
				expectedBranch: selectedNode.plan.branch || projection.layout.branchKey,
				observedCommit: selectedNode.plan.commit || selectedNode.entityRef.observedCommit,
				idempotencyKey: key,
				columns: 80,
				rows: 24
			});
			setResults((current) => ({ ...current, [result.session.id]: result }));
			setActiveTerminalId(result.session.id);
			await Promise.all([Promise.resolve(onReload()), refreshRecords()]);
		} catch (caught) {
			if (caught instanceof ApiError && (caught.code === 'terminal_branch_mismatch' || caught.code === 'terminal_revision_mismatch')) setMismatch(caught.details);
			setError(caught instanceof Error ? caught.message : 'Terminal launch failed.');
		} finally {
			pendingKey.current = '';
			setLaunching(false);
		}
	};

	const cancel = async (sessionId: string) => {
		if (!window.confirm('Cancel this terminal process? Its durable session metadata will remain available.')) return;
		try {
			const session = await api.cancelEmbeddedAISession(sessionId);
			setResults((current) => current[sessionId] ? { ...current, [sessionId]: { ...current[sessionId], session } } : current);
			await Promise.all([Promise.resolve(onReload()), refreshRecords()]);
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Could not cancel the terminal process.');
		}
	};

	const runVerification = async () => {
		if (!verificationContext || verificationCapability?.state !== 'available' || verifying) return;
		setVerifying(true);
		setError('');
		try {
			let job = await api.createVerificationJob(verificationContext.id, { profile: 'smoke', trigger: 'canvas' });
			await Promise.resolve(onReload());
			for (let attempt = 0; attempt < 60 && (job.status === 'queued' || job.status === 'running'); attempt += 1) {
				await new Promise((resolve) => setTimeout(resolve, 1000));
				job = await api.verificationJob(verificationContext.id, job.id);
				await Promise.resolve(onReload());
			}
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Verification could not run.');
		} finally {
			setVerifying(false);
		}
	};

	const terminalVisible = (result: EmbeddedAISessionResult) => {
		if (result.session.id !== activeTerminalId) return false;
		if (selectedNode?.kind === 'session') return selectedNode.entityRef.sessionId === result.session.id;
		return selectedNode?.kind === 'plan' && selectedNode.plan?.itemId === result.session.itemId;
	};

	return <aside className={`canvas-workbench${selectedNode ? ' open' : ''}`} aria-label="Canvas Workbench">
		<header><div><span>Workbench</span><strong>{workbenchTitle(selectedNode)}</strong></div>{selectedNode && <button type="button" className="icon-button" aria-label="Close Workbench" onClick={onClose}><X size={16} /></button>}</header>
		{!selectedNode && <div className="canvas-workbench-empty"><TerminalSquare size={22} /><p>Select a workspace, plan, or session to inspect it here.</p></div>}
		{selectedNode && selectedNode.state !== 'resolved' && <section className="canvas-workbench-section canvas-reference-warning" role="status"><h2>{selectedNode.state === 'forbidden' ? 'Access restricted' : 'Reference unavailable'}</h2><p>{selectedNode.state === 'forbidden' ? 'You no longer have permission to view this entity. Cached titles are hidden.' : 'The entity no longer resolves on this workspace branch. Its placement is retained for recovery.'}</p><button type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh reference</button></section>}
		{selectedNode?.workspace && <section className="canvas-workbench-section"><h2>Git summary</h2><dl><dt>Branch</dt><dd>{selectedNode.workspace.branch || 'Unavailable'}</dd><dt>HEAD</dt><dd><code>{shortSHA(selectedNode.workspace.commit)}</code></dd><dt>Working tree</dt><dd>{selectedNode.workspace.git?.conflicted ? 'Conflicted' : selectedNode.workspace.git?.dirty ? `${selectedNode.workspace.git.changes.length} changed files` : 'Clean'}</dd></dl><button type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh Git status</button></section>}
		{selectedNode?.plan && <section className="canvas-workbench-section"><h2>{selectedNode.plan.identifier || 'Plan'}</h2><p>{selectedNode.plan.title}</p><p className="canvas-workbench-branch"><GitBranch size={13} /> {selectedNode.plan.branch} · <code>{shortSHA(selectedNode.plan.commit)}</code></p><button className="primary" type="button" disabled={!canLaunch || launching} onClick={() => void launch()}><Play size={14} /> {launching ? 'Launching…' : 'Launch terminal'}</button>{launchCapability && launchCapability.state !== 'available' && <p className="canvas-capability-message">{launchCapability.message}</p>}</section>}
		{verificationContext && <section className="canvas-workbench-section canvas-verification"><h2>Verification</h2>{verificationContext.verification ? <VerificationSummary job={verificationContext.verification} /> : <p>No verification result in this application run.</p>}<button className="primary" type="button" disabled={verificationCapability?.state !== 'available' || verifying} title={verificationCapability?.state !== 'available' ? verificationCapability?.message : undefined} onClick={() => void runVerification()}><Play size={14} /> {verifying ? 'Verifying…' : 'Run smoke verification'}</button>{verificationCapability && verificationCapability.state !== 'available' && <p className="canvas-capability-message">{verificationCapability.message}</p>}</section>}
		{selectedNode && (selectedNode.workspace || selectedNode.plan) && onOpenFullView && <section className="canvas-workbench-section"><button type="button" onClick={() => onOpenFullView(selectedNode)}>Open full view</button></section>}
		{selectedNode?.session && <SessionDetails record={selectedNode.session.record} onOpen={() => setActiveTerminalId(selectedNode.session!.record.id)} onCancel={() => void cancel(selectedNode.session!.record.id)} />}
		{mismatch && <section className="canvas-branch-mismatch" role="alert"><AlertTriangle size={17} /><div><strong>Checkout changed</strong><p>Expected <code>{mismatch.expectedBranch || mismatch.branch || mismatch.expectedCommit}</code>; current <code>{mismatch.currentBranch || mismatch.currentCommit}</code>.</p><button type="button" onClick={() => void onReload()}><RefreshCw size={13} /> Refresh Canvas and Git status</button></div></section>}
		{error && <p className="canvas-workbench-error" role="alert">{error}</p>}
		{unplacedActive.length > 0 && <section className="canvas-workbench-section canvas-active-unplaced"><h2>Active unplaced sessions</h2>{unplacedActive.map((record) => <button type="button" key={record.id} onClick={() => void Promise.resolve(onPlaceUnplaced()).then(() => onReload()).then(() => onSelectNode(`session:${record.id}`))}><TerminalSquare size={13} /><span>{record.provider}<small>{record.requestedBranch} · {record.state} · place and open</small></span></button>)}</section>}
		<div className="canvas-workbench-terminal-pool">{Object.values(results).map((result) => <EmbeddedTerminal key={result.session.id} initial={result} visible={terminalVisible(result)} mode="side_panel" title={`${result.session.provider} terminal`} subtitle={result.record?.requestedBranch || projection.layout.branchKey} cancelOnClose={false} compact onStartMove={() => {}} onToggleDockMode={() => {}} onToggleMinimize={() => {}} onToggleMaximize={() => {}} onClose={() => setActiveTerminalId('')} onStateChange={(state, exitCode) => setResults((current) => ({ ...current, [result.session.id]: { ...current[result.session.id], session: { ...current[result.session.id].session, state, exitCode } } }))} />)}</div>
	</aside>;
}

function SessionDetails({ record, onOpen, onCancel }: { record: SafeSessionRecord; onOpen: () => void; onCancel: () => void }) {
	const active = record.state === 'starting' || record.state === 'running';
	return <section className="canvas-workbench-section"><h2>Terminal session</h2><dl><dt>Provider</dt><dd>{record.provider}</dd><dt>Branch</dt><dd>{record.requestedBranch}</dd><dt>State</dt><dd>{record.state}</dd>{record.exitCode !== undefined && <><dt>Exit code</dt><dd>{record.exitCode}</dd></>}</dl>{record.live && <button className="primary" type="button" onClick={onOpen}><TerminalSquare size={14} /> Open terminal</button>}{active && <button className="danger-confirm" type="button" onClick={onCancel}><Square size={13} /> Cancel process</button>}{record.state === 'interrupted' && <p>The application restarted without this process. Reconnect is unavailable; launch a new session.</p>}</section>;
}

function VerificationSummary({ job }: { job: VerificationJob }) {
	const freshness = job.freshness || 'inconclusive';
	const historicalPass = job.status === 'passed' && freshness !== 'fresh';
	return <dl className={`verification-summary result-${job.status} freshness-${freshness}`}><dt>Result</dt><dd>{historicalPass ? 'Passed (historical)' : job.status}</dd><dt>Freshness</dt><dd>{freshness}</dd><dt>Verified revision</dt><dd><code>{shortSHA(job.finishFingerprint?.commit || job.startFingerprint?.commit)}</code></dd><dt>Current revision</dt><dd><code>{shortSHA(job.currentFingerprint?.commit)}</code></dd></dl>;
}

function workbenchTitle(node?: CanvasNode) { return node?.workspace?.name || node?.plan?.identifier || node?.session?.record.provider || (node ? node.state === 'forbidden' ? 'Restricted node' : 'Unavailable node' : 'No selection'); }
function shortSHA(value?: string) { return value ? value.slice(0, 8) : 'Unavailable'; }
function newIdempotencyKey() { return globalThis.crypto?.randomUUID?.() ?? `canvas-${Date.now()}-${Math.random().toString(16).slice(2)}`; }
