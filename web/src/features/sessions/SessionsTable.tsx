import type { GlobalSessionRow } from '../../lib/types';

/**
 * The cross-workspace session table for the Sessions page. This duplicates
 * the Workbench's own session table built in Phase F2 (both read a
 * GlobalSessionRow-shaped list and expose Open/Stop/Changes); the two are
 * meant to be unified onto one shared component in Phase I1 once F2 lands.
 */
export function SessionsTable({ rows, onOpen, onStop, onChanges }: {
	rows: GlobalSessionRow[];
	onOpen: (row: GlobalSessionRow) => void;
	onStop: (row: GlobalSessionRow) => void;
	onChanges: (row: GlobalSessionRow) => void;
}) {
	return (
		<table className="sessions-table">
			<thead>
				<tr>
					<th scope="col">Workspace</th>
					<th scope="col">Branch</th>
					<th scope="col">Plan</th>
					<th scope="col">Provider</th>
					<th scope="col">State</th>
					<th scope="col">Started</th>
					<th scope="col">Duration</th>
					<th scope="col">Changes</th>
					<th scope="col"><span className="sr-only">Actions</span></th>
				</tr>
			</thead>
			<tbody>
				{rows.map((row) => {
					const label = planLabel(row);
					const live = isLive(row);
					return (
						<tr key={row.record.id}>
							<td>{row.workspaceName}</td>
							<td>{row.checkoutBranch || '—'}</td>
							<td>{label}</td>
							<td>{providerLabel(row.record.provider)}</td>
							<td><span className={`session-state session-state-${row.record.state}`}>{stateLabel(row.record.state)}{!live && typeof row.record.exitCode === 'number' ? ` (${row.record.exitCode})` : ''}</span></td>
							<td>{formatStarted(row.record.startedAt)}</td>
							<td>{formatDuration(row.record.startedAt, row.record.endedAt)}</td>
							<td>{typeof row.changesCount === 'number' ? row.changesCount : '—'}</td>
							<td className="sessions-table-actions">
								<button type="button" className="ghost" onClick={() => onOpen(row)} aria-label={`Open ${label} in the Workbench`}>Open</button>
								<button type="button" className="ghost" onClick={() => onChanges(row)} aria-label={`View changes for ${label}`}>Changes</button>
								{live && <button type="button" className="ghost" onClick={() => onStop(row)} aria-label={`Stop ${label}`}>Stop</button>}
							</td>
						</tr>
					);
				})}
			</tbody>
		</table>
	);
}

function isLive(row: GlobalSessionRow): boolean {
	return Boolean(row.record.live) && (row.record.state === 'starting' || row.record.state === 'running');
}

function planLabel(row: GlobalSessionRow): string {
	return row.planTitle || row.record.planRef?.identifier || row.record.planRef?.itemId || row.record.intent;
}

function providerLabel(provider: string): string {
	return ({ codex: 'Codex', claude: 'Claude', copilot: 'Copilot', opencode: 'OpenCode' } as Record<string, string>)[provider] ?? provider;
}

function stateLabel(state: string): string {
	return ({ starting: 'Starting', running: 'Running', exited: 'Exited', cancelled: 'Cancelled', failed: 'Failed', interrupted: 'Interrupted' } as Record<string, string>)[state] ?? state;
}

function formatStarted(startedAt: string): string {
	const date = new Date(startedAt);
	if (Number.isNaN(date.getTime())) return '—';
	return date.toLocaleString();
}

function formatDuration(startedAt: string, endedAt?: string): string {
	const start = new Date(startedAt).getTime();
	if (Number.isNaN(start)) return '—';
	const end = endedAt ? new Date(endedAt).getTime() : Date.now();
	const seconds = Math.max(0, Math.round((end - start) / 1000));
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	const remainingSeconds = seconds % 60;
	if (hours > 0) return `${hours}h ${minutes}m`;
	if (minutes > 0) return `${minutes}m ${remainingSeconds}s`;
	return `${remainingSeconds}s`;
}
