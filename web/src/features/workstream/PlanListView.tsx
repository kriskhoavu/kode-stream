import { FileText } from 'lucide-react';
import type { MouseEvent } from 'react';
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
 * The plans on the branch as rows in the Task Queue's style, one group per
 * status in board order. A person moves a plan with its status menu, as on
 * the board; a row opens the plan's drawer, its ID the plan's page, or its title where the ID repeats it.
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
		<div className="plan-list hx" aria-busy={loading || undefined}>
			{groups.map((group) => (
				<section key={group.status} className="plan-list-group">
					<h3 className="queue-section-title">{statusLabels[group.status]} <span className="hx-num">{group.rows.length}</span></h3>
					<ol className="hx-queue-list plan-list-rows" aria-label={statusLabels[group.status]}>
						{group.rows.map((item) => {
							const work = workOf(item.id);
							const movable = isItemDraggable(item) && !isDocumentationMetadataSource(item.metadataSource);
							const showKey = Boolean(item.identifier) && item.identifier.toLowerCase() !== item.title.toLowerCase();
							const open = (event: MouseEvent) => { event.stopPropagation(); onOpen(item); };
							const details = [sourceLabel(item, workspace), item.author || item.owner, item.updatedAt ? `Updated ${new Date(item.updatedAt).toLocaleDateString()}` : ''].filter(Boolean).join(' · ');
							return (
								<li key={item.id} data-item-id={item.id} className="hx-row hx-queue-row plan-list-row" aria-busy={pendingIds.has(item.id) || undefined} onClick={() => onPreview(item)}>
									<span className="hx-queue-rank" aria-hidden="true"><FileText size={14} /></span>
									<span className="hx-row-key">
										{showKey && <button type="button" className="hx-key-button" title={`Open ${item.identifier}`} onClick={open}>{item.identifier}</button>}
										{item.scope && <small className="hx-muted">{item.scope}</small>}
									</span>
									<span className="hx-row-title">
										{showKey
											? <span className="hx-row-name">{item.title}</span>
											: <button type="button" className="hx-title-button" onClick={open}>{item.title}</button>}
										{details && <small className="hx-muted">{details}</small>}
									</span>
									<span className="plan-list-harness">{work ? <HxStatusBadge status={work.status} size="sm" /> : <span className="hx-muted">By hand</span>}</span>
									<span className="hx-queue-actions" onClick={(event) => event.stopPropagation()}>
										{movable
											? <StatusMenu value={item.status} onChange={(status) => onMove(item, status)} ariaLabel={`Move ${item.title}`} />
											: <span className="hx-muted">{statusLabels[item.status]}</span>}
									</span>
								</li>
							);
						})}
					</ol>
				</section>
			))}
		</div>
	);
}

function byNewest(a: ItemSummary, b: ItemSummary): number {
	return Date.parse(b.updatedAt ?? '') - Date.parse(a.updatedAt ?? '') || a.title.localeCompare(b.title);
}
