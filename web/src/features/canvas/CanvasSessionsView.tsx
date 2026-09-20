import { useEffect, useMemo, useState } from 'react';
import { api } from '../../shared/api';
import type { CanvasProjection, GlobalSessionRow, SafeSessionRecord } from '../../lib/types';
import { SessionBrowser } from '../sessions/SessionBrowser';
import { WorkbenchViewSwitcher } from './WorkbenchViewSwitcher';

export function CanvasSessionsView({ workspaceId, workspaceName, checkoutBranch, projection, onViewChange, onOpenOnBoard, onReload }: {
	workspaceId: string;
	workspaceName?: string;
	checkoutBranch?: string;
	projection: CanvasProjection;
	onViewChange: (view: 'canvas' | 'sessions') => void;
	onOpenOnBoard: (sessionId: string) => void;
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
		{error && <div className="canvas-state error" role="alert">{error}<button type="button" onClick={() => setReloadToken((token) => token + 1)}>Retry</button></div>}
		{!error && !records && <div className="canvas-state" role="status">Loading sessions…</div>}
		{!error && records && <SessionBrowser rows={rows} grouping="plan" persistKey={`workbench:${workspaceId}`} onStop={stop} onRemove={remove} onReload={onReload} onOpenOnBoard={(row) => onOpenOnBoard(row.record.id)} emptyMessage="No sessions for this workspace and branch." />}
	</div>;
}
