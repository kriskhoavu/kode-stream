import { useEffect, useMemo, useState } from 'react';
import { api } from '../../shared/api';
import type { CanvasProjection, GlobalSessionRow, HarnessRun, HarnessTask, SafeSessionRecord } from '../../lib/types';
import { SessionBrowser } from '../sessions/SessionBrowser';
import { WorkbenchViewSwitcher } from './WorkbenchViewSwitcher';

export function CanvasSessionsView({ workspaceId, workspaceName, checkoutBranch, projection, onViewChange, onOpenOnBoard, onOpenOnCard, onOpenHarnessRun, onReload }: {
	workspaceId: string;
	workspaceName?: string;
	checkoutBranch?: string;
	projection: CanvasProjection;
	onViewChange: (view: 'canvas' | 'sessions') => void;
	onOpenOnBoard: (sessionId: string) => void;
	onOpenOnCard?: (itemId: string, sessionId: string) => void;
	/** Opens a harness run's live conversation. Without it the harness section is not shown. */
	onOpenHarnessRun?: (runId: string) => void;
	onReload: () => Promise<unknown> | void;
}) {
	const [records, setRecords] = useState<SafeSessionRecord[]>();
	const [error, setError] = useState('');
	const [reloadToken, setReloadToken] = useState(0);

	useEffect(() => {
		const controller = new AbortController();
		setRecords(undefined);
		setError('');
		api.aiSessionRecords(workspaceId, checkoutBranch, controller.signal)
			.then((result) => { if (!controller.signal.aborted) setRecords(result); })
			.catch((caught) => { if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Sessions could not be loaded.'); });
		return () => controller.abort();
	}, [workspaceId, checkoutBranch, reloadToken]);

	const rows = useMemo<GlobalSessionRow[]>(() => (records ?? []).map((record) => ({
		record,
		workspaceName: workspaceName ?? 'Workspace',
		checkoutBranch: checkoutBranch ?? '',
		planTitle: record.planRef ? projection.nodes.find((node) => node.plan?.itemId === record.planRef?.itemId)?.plan?.title : undefined
	})), [checkoutBranch, projection.nodes, records, workspaceName]);
	const liveSessionCount = rows.filter((row) => row.record.live && (row.record.state === 'starting' || row.record.state === 'running')).length;
	const stop = async (row: GlobalSessionRow) => {
		await api.cancelEmbeddedAISession(row.record.id);
		setReloadToken((token) => token + 1);
		await Promise.resolve(onReload());
	};
	const remove = async (row: GlobalSessionRow) => {
		await api.deleteSession(row.record.id);
		setRecords((current) => current?.filter((record) => record.id !== row.record.id));
		await Promise.resolve(onReload());
	};

	return <div className="canvas-work-area canvas-sessions-view">
		<div className="canvas-sessions-toolbar">
			<WorkbenchViewSwitcher view="sessions" sessionCount={liveSessionCount} onChange={onViewChange} />
		</div>
		{onOpenHarnessRun && <HarnessRunsSection workspaceId={workspaceId} reloadToken={reloadToken} onOpen={onOpenHarnessRun} />}
		{error && <div className="canvas-state error" role="alert">{error}<button type="button" onClick={() => setReloadToken((token) => token + 1)}>Retry</button></div>}
		{!error && !records && <div className="canvas-state" role="status">Loading sessions…</div>}
		{!error && records && <SessionBrowser rows={rows} grouping="plan" persistKey={`workbench:${workspaceId}`} onStop={stop} onRemove={remove} onReload={onReload} onOpenOnBoard={(row) => onOpenOnBoard(row.record.id)} onOpenOnCard={onOpenOnCard ? (row) => { if (row.record.planRef) onOpenOnCard(row.record.planRef.itemId, row.record.id); } : undefined} emptyMessage="No sessions for this workspace and branch." />}
	</div>;
}

const harnessRunWords: Record<HarnessRun['state'], string> = { running: 'Working', blocked: 'Waiting on you', passed: 'Done', failed: 'Stopped', cancelled: 'Stopped' };

/**
 * The harness runs of this workspace. They are not sessions: each agent runs
 * headless in a worktree of its own, so nothing here can be attached to a
 * terminal. The link opens the run's live conversation in the Harness.
 */
function HarnessRunsSection({ workspaceId, reloadToken, onOpen }: { workspaceId: string; reloadToken: number; onOpen: (runId: string) => void }) {
	const [rows, setRows] = useState<{ run: HarnessRun; task?: HarnessTask }[]>();
	useEffect(() => {
		const controller = new AbortController();
		Promise.all([api.harnessRuns(controller.signal), api.harnessTasks(controller.signal)])
			.then(([runs, tasks]) => {
				if (controller.signal.aborted) return;
				const byId = new Map(tasks.map((task) => [task.id, task]));
				const order = (run: HarnessRun) => (run.state === 'running' ? 0 : run.state === 'blocked' ? 1 : 2);
				setRows(runs
					.filter((run) => run.workspaceId === workspaceId)
					.sort((a, b) => order(a) - order(b) || Date.parse(b.startedAt) - Date.parse(a.startedAt))
					.slice(0, 8)
					.map((run) => ({ run, task: byId.get(run.taskId) })));
			})
			// The harness may be off or unsupported by the storage; then there is nothing to list.
			.catch(() => { if (!controller.signal.aborted) setRows([]); });
		return () => controller.abort();
	}, [workspaceId, reloadToken]);
	if (!rows?.length) return null;
	const live = rows.filter((row) => row.run.state === 'running').length;
	return <section className="canvas-harness-runs" aria-labelledby="canvas-harness-runs-title">
		<h3 id="canvas-harness-runs-title">Harness{live ? ` · ${live} working` : ''}</h3>
		<p className="canvas-harness-note">Agents the harness runs for this workspace, each in a worktree of its own, without a terminal. Open one to follow what it thinks, calls and answers.</p>
		<ul>
			{rows.map(({ run, task }) => (
				<li key={run.id}>
					<span className={`canvas-harness-state state-${run.state}`}>{harnessRunWords[run.state]}</span>
					<span className="canvas-harness-title">{task ? `${task.source.key} · ${task.title}` : run.id}</span>
					<button type="button" className="secondary canvas-harness-open" onClick={() => onOpen(run.id)}>{run.state === 'running' ? 'Watch' : 'Open'}</button>
				</li>
			))}
		</ul>
	</section>;
}
