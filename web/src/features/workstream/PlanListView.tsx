import { FileText, Play } from 'lucide-react';
import type { MouseEvent } from 'react';
import type { ItemStatus, ItemSummary, WorkspaceConfig } from '../../lib/types';
import { statusLabels, statusOrder } from '../../shared/api';
import { isDocumentationMetadataSource } from '../../lib/vocabulary';
import { StatusMenu } from '../../components/StatusMenu';
import { formatAgo } from '../harness/vocabulary';
import { useNow } from '../harness/useNow';
import { AgentCluster } from './AgentsPanel';
import type { AgentFocus } from './AgentsPanel';
import { isItemDraggable } from './dragAndDrop';
import { sourceLabel } from './filtering';
import { ByHandChip, HarnessCardChips } from './HarnessCardChips';
import type { CardWork } from './harnessJoin';
import type { ItemAgents } from './useItemSessions';
import './planList.css';

export function PlanListView({ items, workspace, loading, pendingIds, workOf, agentsOf, harnessOn = false, onPreview, onOpen, onMove, onOpenAgent, onRunWithHarness }: {
	items: ItemSummary[];
	workspace?: WorkspaceConfig;
	loading: boolean;
	pendingIds: Set<string>;
	workOf: (id: string) => CardWork | undefined;
	agentsOf?: (id: string) => ItemAgents | undefined;
	harnessOn?: boolean;
	onPreview: (item: ItemSummary) => void;
	onOpen: (item: ItemSummary) => void;
	onMove: (item: ItemSummary, status: ItemStatus) => void;
	onOpenAgent?: (item: ItemSummary, focus: AgentFocus) => void;
	onRunWithHarness?: (item: ItemSummary) => void;
}) {
	const now = useNow(60_000);
	if (loading && items.length === 0) return <div className="plan-list plan-list-empty" aria-busy="true">Loading plans…</div>;
	if (items.length === 0) return <div className="plan-list plan-list-empty">No plans match.</div>;
	const groups = statusOrder
		.map((status) => ({ status, rows: items.filter((item) => item.status === status).sort(byNewest) }))
		.filter((group) => group.rows.length > 0);
	return (
		<div className="plan-list hx" aria-busy={loading || undefined}>
			{groups.map((group) => (
				<section key={group.status} className={`plan-list-group plan-status-${group.status}`}>
					<h3 className="queue-section-title"><span className="plan-status-dot" aria-hidden="true" />{statusLabels[group.status]} <span className="hx-num">{group.rows.length}</span></h3>
					<ol className="hx-queue-list plan-list-rows" aria-label={statusLabels[group.status]}>
						{group.rows.map((item) => {
							const work = workOf(item.id);
							const movable = isItemDraggable(item) && !isDocumentationMetadataSource(item.metadataSource);
							const showKey = Boolean(item.identifier) && item.identifier.toLowerCase() !== item.title.toLowerCase();
							const open = (event: MouseEvent) => { event.stopPropagation(); onOpen(item); };
							const source = sourceLabel(item, workspace);
							const person = item.author || item.owner;
							const tags = item.tags.filter((tag) => tag !== source && tag !== item.scope).slice(0, 3);
							const runnable = onRunWithHarness && !work && item.status !== 'unsorted';
							return (
								<li key={item.id} data-item-id={item.id} className={`hx-row hx-queue-row plan-list-row plan-status-${item.status}`} aria-busy={pendingIds.has(item.id) || undefined} onClick={() => onPreview(item)}>
									<span className="plan-list-icon" aria-hidden="true"><FileText size={15} /></span>
									<span className="plan-list-id-cell">
										{showKey && <button type="button" className="hx-key-button" title={`Open ${item.identifier}`} onClick={open}>{item.identifier}</button>}
										<span className="plan-list-chips">
											{item.scope && <span className="plan-list-chip">{item.scope}</span>}
											{source && <span className="plan-list-chip quiet">{source}</span>}
										</span>
									</span>
									<span className="plan-list-main">
										{showKey
											? <span className="hx-row-name">{item.title}</span>
											: <button type="button" className="hx-title-button" onClick={open}>{item.title}</button>}
										{item.description && <span className="plan-list-description">{item.description}</span>}
										{tags.length > 0 && <span className="plan-list-tags">{tags.map((tag) => <span key={tag}>#{tag}</span>)}</span>}
									</span>
									<span className="plan-list-person">
										<span className="avatar" aria-hidden="true">{(person || '?').slice(0, 1).toUpperCase()}</span>
										<span className="plan-list-person-text">
											<span className={person ? undefined : 'unknown'}>{person || 'No owner'}</span>
											{item.updatedAt && <time dateTime={item.updatedAt} title={new Date(item.updatedAt).toLocaleString()}>Updated {formatAgo(item.updatedAt, now)}</time>}
										</span>
									</span>
									<span className="plan-list-agents" onClick={(event) => event.stopPropagation()}>
										{onOpenAgent && <AgentCluster agents={agentsOf?.(item.id)} title={item.title} onOpen={(focus) => onOpenAgent(item, focus)} />}
									</span>
									<span className="plan-list-harness" onClick={(event) => event.stopPropagation()}>
										{work
											? <HarnessCardChips work={work} />
											: harnessOn && <>
												<ByHandChip />
												{runnable && <button type="button" className="secondary hx-small-button" title="Run with the harness" onClick={() => onRunWithHarness?.(item)}><Play size={12} aria-hidden="true" />Run</button>}
											</>}
									</span>
									<span className="plan-list-status" onClick={(event) => event.stopPropagation()}>
										{movable
											? <StatusMenu value={item.status} onChange={(status) => onMove(item, status)} ariaLabel={`Move ${item.title}`} toned />
											: <span className={`status-pill plan-status-${item.status}`}><span className="plan-status-dot" aria-hidden="true" />{statusLabels[item.status]}</span>}
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
