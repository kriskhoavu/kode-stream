import { useEffect, useMemo, useState } from 'react';
import { api } from '../../shared/api';
import type { CanvasProjection, GlobalSessionRow, HarnessRunDetail, SafeSessionRecord } from '../../lib/types';
import { useBoardHarness } from '../workstream/useBoardHarness';
import type { HarnessSessionRow } from '../sessions/agentSessions';
import { HarnessSessionDetail } from '../sessions/HarnessSessionDetail';
import { SessionBrowser } from '../sessions/SessionBrowser';

/** How many of a workspace's runs list their agents: the newest, and every one still working. */
const harnessRunLimit = 12;

export function CanvasSessionsView({ workspaceId, workspaceName, checkoutBranch, projection, onOpenOnBoard, onOpenOnCard, onOpenHarnessRun, onReload }: {
	workspaceId: string;
	workspaceName?: string;
	checkoutBranch?: string;
	/** The canvas, once it has loaded: it names each session's plan. Sessions show without it. */
	projection?: CanvasProjection;
	onOpenOnBoard: (sessionId: string) => void;
	onOpenOnCard?: (itemId: string, sessionId: string) => void;
	/** Opens a harness run's page. Without it harness sessions are not listed. */
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

	const rows = useMemo<GlobalSessionRow[]>(() => (records ?? []).map((record) => {
		const plan = record.planRef ? projection?.nodes.find((node) => node.plan?.itemId === record.planRef?.itemId)?.plan : undefined;
		return { record, workspaceName: workspaceName ?? 'Workspace', checkoutBranch: checkoutBranch ?? '', planTitle: plan?.title, planKey: plan?.identifier };
	}), [checkoutBranch, projection?.nodes, records, workspaceName]);
	const harness = useHarnessSessions(workspaceId, Boolean(onOpenHarnessRun));
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
{error && <div className="canvas-state error" role="alert">{error}<button type="button" onClick={() => setReloadToken((token) => token + 1)}>Retry</button></div>}
		{!error && !records && <div className="canvas-state" role="status">Loading sessions…</div>}
		{!error && records && <SessionBrowser rows={rows} harness={harness.rows} renderHarnessDetail={(row) => <HarnessSessionDetail row={row} client={harness.controller.client} onOpenRun={onOpenHarnessRun} onStopRun={harness.controller.cancelRun} decide={{ approve: harness.controller.approve, reject: harness.controller.reject, decisions: harness.controller.state.decisions }} />} grouping="plan" persistKey={`workbench:${workspaceId}`} onStop={stop} onRemove={remove} onReload={onReload} onOpenOnBoard={(row) => onOpenOnBoard(row.record.id)} onOpenOnCard={onOpenOnCard ? (row) => { if (row.record.planRef) onOpenOnCard(row.record.planRef.itemId, row.record.id); } : undefined} emptyMessage="No sessions for this workspace and branch." />}
	</div>;
}

/**
 * The harness's agents in this workspace, as harness sessions: one row per
 * ticket, holding the ticket's newest runs and their agents, live from the
 * harness's event feed. Undefined while the harness is off, so the list then
 * holds interactive sessions only.
 */
function useHarnessSessions(workspaceId: string, wanted: boolean): { rows?: HarnessSessionRow[]; controller: ReturnType<typeof useBoardHarness> } {
	const controller = useBoardHarness();
	const { state, loadRun } = controller;
	const on = wanted && Boolean(state.status?.enabled && state.status.storageSupported);
	const runs = useMemo(() => Object.values(state.runs)
		.filter((run) => run.workspaceId === workspaceId)
		.sort((a, b) => Number(b.state === 'running') - Number(a.state === 'running') || Date.parse(b.startedAt) - Date.parse(a.startedAt))
		.slice(0, harnessRunLimit), [state.runs, workspaceId]);
	const revisions = runs.map((run) => `${run.id}:${run.revision}`).join(',');

	// Each run's agents come with its detail; read it again whenever the run moves on.
	useEffect(() => {
		if (!on) return;
		for (const run of runs) {
			const detail = state.details[run.id];
			if (!detail || detail.run.revision !== run.revision) void loadRun(run.id);
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [on, revisions]);

	const rows = useMemo(() => {
		if (!on) return undefined;
		const tickets = new Map<string, HarnessSessionRow>();
		for (const run of runs) {
			const detail = state.details[run.id];
			if (!detail) continue;
			const task = state.tasks[run.taskId] ?? detail.task;
			const key = task?.source.key ?? run.id;
			const current: HarnessRunDetail = { ...detail, run };
			const ticket = tickets.get(key);
			if (ticket) ticket.runs.push(current);
			else tickets.set(key, { key, task, runs: [current] });
		}
		// `runs` is already working first, then newest; a ticket's runs keep that order.
		return [...tickets.values()];
	}, [on, runs, state.details, state.tasks]);
	return { rows, controller };
}
