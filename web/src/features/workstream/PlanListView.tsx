import { FileText } from 'lucide-react';
import type { ItemStatus, ItemSummary, WorkspaceConfig } from '../../lib/types';
import { statusLabels, statusOrder } from '../../shared/api';
import { isDocumentationMetadataSource } from '../../lib/vocabulary';
import { StatusMenu } from '../../components/StatusMenu';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { isItemDraggable } from './dragAndDrop';
import { sourceLabel } from './filtering';
import type { CardWork } from './harnessJoin';
import './planList.css';

/**
 * The plans on the branch as rows, one group per status in board order. A
 * person moves a plan with its status menu, as on the board; a row opens the
 * plan's drawer, its title the plan's page.
 */
export function PlanListView({ items, workspace, loading, pendingIds, workOf, onPreview, onOpen, onMove }: {
	items: ItemSummary[];
	workspace?: WorkspaceConfig;
	loading: boolean;
	pendingIds: Set<string>;
	/** What the harness holds for a plan, when it works on it. */
	workOf: (id: string) => CardWork | undefined;
	onPreview: (item: ItemSummary) => void;
	onOpen: (item: ItemSummary) => void;
	onMove: (item: ItemSummary, status: ItemStatus) => void;
}) {
	if (loading && items.length === 0) return <div className="plan-list plan-list-empty" aria-busy="true">Loading plans…</div>;
	if (items.length === 0) return <div className="plan-list plan-list-empty">No plans match.</div>;
	const groups = statusOrder
		.map((status) => ({ status, rows: items.filter((item) => item.status === status).sort(byNewest) }))
		.filter((group) => group.rows.length > 0);
	return (
		<div className="plan-list" aria-busy={loading || undefined}>
			<table>
				<thead>
					<tr>
						<th scope="col">Plan</th>
						<th scope="col">Status</th>
						<th scope="col">Service</th>
						<th scope="col">Source</th>
						<th scope="col">Owner</th>
						<th scope="col">Updated</th>
						<th scope="col">Harness</th>
					</tr>
				</thead>
				{groups.map((group) => (
					<tbody key={group.status} aria-label={statusLabels[group.status]}>
						<tr className="plan-list-group">
							<th scope="rowgroup" colSpan={7}>{statusLabels[group.status]} <span>{group.rows.length}</span></th>
						</tr>
						{group.rows.map((item) => {
							const work = workOf(item.id);
							const movable = isItemDraggable(item) && !isDocumentationMetadataSource(item.metadataSource);
							const showKey = item.identifier && item.identifier.toLowerCase() !== item.title.toLowerCase();
							return (
								<tr key={item.id} data-item-id={item.id} className="plan-list-row" aria-busy={pendingIds.has(item.id) || undefined} onClick={() => onPreview(item)}>
									<td className="plan-list-title">
										<span className="plan-list-title-inner">
											<FileText size={14} aria-hidden="true" />
											<button type="button" className="plan-list-link" onClick={(event) => { event.stopPropagation(); onOpen(item); }}>{item.title}</button>
											{showKey && <span className="plan-list-key">{item.identifier}</span>}
										</span>
									</td>
									<td onClick={(event) => event.stopPropagation()}>
										{movable
											? <StatusMenu value={item.status} onChange={(status) => onMove(item, status)} ariaLabel={`Move ${item.title}`} />
											: <span className="plan-list-status">{statusLabels[item.status]}</span>}
									</td>
									<td>{item.scope || '—'}</td>
									<td>{sourceLabel(item, workspace) || '—'}</td>
									<td>{item.author || item.owner || '—'}</td>
									<td>{item.updatedAt ? <time dateTime={item.updatedAt}>{new Date(item.updatedAt).toLocaleDateString()}</time> : '—'}</td>
									<td>{work ? <HxStatusBadge status={work.status} size="sm" /> : <span className="plan-list-muted">By hand</span>}</td>
								</tr>
							);
						})}
					</tbody>
				))}
			</table>
		</div>
	);
}

function byNewest(a: ItemSummary, b: ItemSummary): number {
	return Date.parse(b.updatedAt ?? '') - Date.parse(a.updatedAt ?? '') || a.title.localeCompare(b.title);
}
