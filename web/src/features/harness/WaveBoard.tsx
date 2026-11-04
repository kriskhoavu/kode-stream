import { useCallback, useMemo, useRef, useState } from 'react';
import type { DragEvent } from 'react';
import { Search } from 'lucide-react';
import type { HarnessRun, HarnessTask } from '../../lib/types';
import { FacetMenu, SelectedFilters } from '../../components/FacetMenu';
import type { FacetOption } from '../../components/FacetMenu';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { compareTasks, waitsFor } from './harnessState';
import { DependencyEdges, useCardAnchors } from './DependencyEdges';
import { QueueList } from './QueueList';
import { TaskCard } from './TaskCard';
import { planWaves } from './waves';
import type { HxStatus } from './vocabulary';
import { hxStatuses, hxStatusOrder, taskStatus } from './vocabulary';

type FacetKey = 'assignee' | 'milestone' | 'epic';
type DoneWaves = 'show' | 'collapse' | 'hide';
const facetTitles: Record<FacetKey, string> = { assignee: 'Anyone', milestone: 'Any milestone', epic: 'Any epic' };
const doneWavesPreference = 'harnessDoneWaves';
const criticalPreference = 'harnessShowCritical';

function readDoneWaves(): DoneWaves {
	const value = readStringPreference(doneWavesPreference);
	return value === 'show' || value === 'hide' ? value : 'collapse';
}

function facetValue(task: HarnessTask, key: FacetKey): string {
	if (key === 'assignee') return task.source.assignee ?? '';
	if (key === 'milestone') return task.milestone ?? '';
	return task.epic ?? '';
}

/**
 * The queue as waves: each column holds what can start once everything to
 * its left is done. Cards re-order by dragging within their wave; curves
 * show what waits for what, and the critical path is the longest chain.
 */
export function WaveBoard({ tasks, allTasks, runs, pendingIds, onOpenTask, onPin, onMove, onRefresh }: {
	tasks: HarnessTask[];
	allTasks: Record<string, HarnessTask>;
	runs: Record<string, HarnessRun>;
	pendingIds: Set<string>;
	onOpenTask: (task: HarnessTask) => void;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	/** Move `task` above `before` in its wave; no `before` means last. */
	onMove: (task: HarnessTask, before?: HarnessTask) => void;
	onRefresh: () => void;
}) {
	const [mode, setMode] = useState<'items' | 'epics'>('items');
	const [groupBy, setGroupBy] = useState<'waves' | 'priority'>('waves');
	const [filters, setFilters] = useState<Record<FacetKey, string[]>>({ assignee: [], milestone: [], epic: [] });
	const [openFacet, setOpenFacet] = useState<FacetKey | ''>('');
	const [query, setQuery] = useState('');
	const [statuses, setStatuses] = useState<Set<HxStatus>>(new Set());
	const [doneWaves, setDoneWavesState] = useState<DoneWaves>(readDoneWaves);
	const [showCritical, setShowCriticalState] = useState(() => readStringPreference(criticalPreference) !== 'false');
	const [hidden, setHidden] = useState<Set<number>>(new Set());
	const [toggled, setToggled] = useState<Set<number>>(new Set());
	const [focusId, setFocusId] = useState<string | undefined>();
	const [drag, setDrag] = useState<{ id: string; wave: number } | undefined>();
	const [hint, setHint] = useState('');
	const boardRef = useRef<HTMLDivElement | null>(null);

	const setDoneWaves = (value: DoneWaves) => { setDoneWavesState(value); writePreference(doneWavesPreference, value); };
	const setShowCritical = (value: boolean) => { setShowCriticalState(value); writePreference(criticalPreference, String(value)); };

	const plan = useMemo(() => planWaves(tasks), [tasks]);
	const statusOf = useMemo(() => new Map(tasks.map((task) => {
		const waits = waitsFor(task, allTasks);
		return [task.id, { status: taskStatus(task, waits), waits }] as const;
	})), [tasks, allTasks]);

	const facetOptions = useMemo(() => {
		const options = {} as Record<FacetKey, FacetOption[]>;
		(Object.keys(facetTitles) as FacetKey[]).forEach((key) => {
			const values = [...new Set(tasks.map((task) => facetValue(task, key)).filter(Boolean))].sort();
			options[key] = values.map((value) => ({ value, label: value }));
		});
		return options;
	}, [tasks]);

	const faceted = useMemo(() => {
		const needle = query.trim().toLowerCase();
		return tasks.filter((task) => (Object.keys(filters) as FacetKey[]).every((key) => filters[key].length === 0 || filters[key].includes(facetValue(task, key)))
			&& (!needle || `${task.source.key} ${task.title} ${task.epic ?? ''}`.toLowerCase().includes(needle)));
	}, [filters, query, tasks]);

	const counts = useMemo(() => {
		const result = new Map<HxStatus, number>();
		faceted.forEach((task) => {
			const status = statusOf.get(task.id)?.status ?? 'not_started';
			result.set(status, (result.get(status) ?? 0) + 1);
		});
		return result;
	}, [faceted, statusOf]);

	const visible = useMemo(() => faceted.filter((task) => statuses.size === 0 || statuses.has(statusOf.get(task.id)?.status ?? 'not_started')), [faceted, statusOf, statuses]);
	const visibleIds = useMemo(() => new Set(visible.map((task) => task.id)), [visible]);

	const columns = plan.waves.map((ids, wave) => {
		const all = ids.map((id) => allTasks[id] ?? tasks.find((task) => task.id === id)).filter((task): task is HarnessTask => Boolean(task));
		const done = all.filter((task) => ['done', 'manual', 'skipped'].includes(statusOf.get(task.id)?.status ?? '')).length;
		const allDone = all.length > 0 && done === all.length;
		const collapsedByDefault = allDone && doneWaves === 'collapse';
		const collapsed = toggled.has(wave) ? !collapsedByDefault : collapsedByDefault;
		const isHidden = hidden.has(wave) || (allDone && doneWaves === 'hide');
		return { wave, cards: all.filter((task) => visibleIds.has(task.id)).sort(compareTasks), total: all.length, done, collapsed, hidden: isHidden };
	});

	const layoutKey = [mode, groupBy, ...columns.map((column) => `${column.wave}:${column.collapsed}:${column.hidden}:${column.cards.map((card) => card.id).join(',')}`)].join('|');
	const { anchors, register } = useCardAnchors(boardRef, layoutKey);
	const liveIds = useMemo(() => new Set(tasks.filter((task) => statusOf.get(task.id)?.status === 'working').map((task) => task.id)), [tasks, statusOf]);

	const toggleFilter = (key: FacetKey, value: string) => setFilters((current) => ({ ...current, [key]: current[key].includes(value) ? current[key].filter((item) => item !== value) : [...current[key], value] }));
	const toggleStatus = (status: HxStatus) => setStatuses((current) => {
		const next = new Set(current);
		if (next.has(status)) next.delete(status); else next.add(status);
		return next;
	});

	const onDragStart = useCallback((event: DragEvent<HTMLElement>, task: HarnessTask) => {
		event.dataTransfer.effectAllowed = 'move';
		event.dataTransfer.setData('text/plain', task.id);
		setDrag({ id: task.id, wave: plan.waveOf.get(task.id) ?? 0 });
		setHint('');
	}, [plan]);
	const onDragEnd = useCallback(() => setDrag(undefined), []);
	const onHover = useCallback((id: string | undefined) => setFocusId(id), []);

	const dragOver = (event: DragEvent<HTMLElement>, wave: number) => {
		if (!drag) return;
		if (drag.wave !== wave) {
			setHint('A card only moves up or down its own wave: it still waits for the cards to its left.');
			return;
		}
		event.preventDefault();
		event.dataTransfer.dropEffect = 'move';
		if (hint) setHint('');
	};
	const drop = (event: DragEvent<HTMLElement>, wave: number) => {
		if (!drag || drag.wave !== wave) return;
		event.preventDefault();
		const task = allTasks[drag.id];
		const targetId = (event.target as HTMLElement).closest<HTMLElement>('[data-task-id]')?.dataset.taskId;
		setDrag(undefined);
		if (!task || targetId === task.id) return;
		onMove(task, targetId ? allTasks[targetId] : undefined);
	};

	const shownStatuses = hxStatusOrder.filter((status) => (counts.get(status) ?? 0) > 0);
	const hiddenColumns = columns.filter((column) => column.hidden && column.total > 0);

	return (
		<div className="hx-board">
			<div className="hx-board-filters">
				<div className="hx-seg hx-seg-small" role="group" aria-label="Show">
					<button type="button" className={mode === 'items' ? 'on' : undefined} aria-pressed={mode === 'items'} onClick={() => setMode('items')}>Items</button>
					<button type="button" className={mode === 'epics' ? 'on' : undefined} aria-pressed={mode === 'epics'} onClick={() => setMode('epics')}>Epics</button>
				</div>
				{(Object.keys(facetTitles) as FacetKey[]).map((key) => (
					<FacetMenu key={key} title={facetTitles[key]} options={facetOptions[key]} selected={filters[key]} open={openFacet === key}
						onOpen={() => setOpenFacet(openFacet === key ? '' : key)} onClose={() => setOpenFacet('')}
						onToggle={(value) => toggleFilter(key, value)} onClear={() => setFilters((current) => ({ ...current, [key]: [] }))} />
				))}
				<label className="filter-input hx-find">
					<Search size={15} aria-hidden="true" />
					<span className="sr-only">Find a task</span>
					<input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find…" />
				</label>
				<div className="hx-seg hx-seg-small" role="group" aria-label="Group by">
					<button type="button" className={groupBy === 'waves' ? 'on' : undefined} aria-pressed={groupBy === 'waves'} onClick={() => setGroupBy('waves')}>Waves</button>
					<button type="button" className={groupBy === 'priority' ? 'on' : undefined} aria-pressed={groupBy === 'priority'} onClick={() => setGroupBy('priority')}>Priority</button>
				</div>
				<span className="hx-board-count hx-muted hx-num">{visible.length} of {tasks.length}</span>
				<button type="button" className="secondary hx-small-button" onClick={onRefresh}>Refresh</button>
			</div>
			<SelectedFilters facets={(Object.keys(facetTitles) as FacetKey[]).map((key) => ({ key, title: facetTitles[key], options: facetOptions[key] }))} filters={filters} onRemove={toggleFilter} />

			<div className="hx-board-statuses">
				{shownStatuses.map((status) => (
					<button key={status} type="button" className={statuses.has(status) ? 'hx-stat on' : 'hx-stat'} aria-pressed={statuses.has(status)} onClick={() => toggleStatus(status)}>
						<i className={`hx-swatch hx-tone-${hxStatuses[status].tone} ${status === 'not_started' || status === 'manual' ? 'outline' : ''}`} aria-hidden="true" />
						{hxStatuses[status].label} <span className="hx-num">{counts.get(status)}</span>
					</button>
				))}
				<span className="hx-board-spacer" />
				{groupBy === 'waves' && mode === 'items' && (
					<button type="button" className={showCritical ? 'hx-stat on' : 'hx-stat'} aria-pressed={showCritical} onClick={() => setShowCritical(!showCritical)}>
						<i className="hx-path-swatch" aria-hidden="true" />Critical path
					</button>
				)}
				{groupBy === 'waves' && (
					<div className="hx-seg hx-seg-small" role="group" aria-label="Done waves">
						{(['show', 'collapse', 'hide'] as DoneWaves[]).map((value) => (
							<button key={value} type="button" className={doneWaves === value ? 'on' : undefined} aria-pressed={doneWaves === value} onClick={() => setDoneWaves(value)}>{value === 'show' ? 'Show' : value === 'collapse' ? 'Collapse' : 'Hide'}</button>
						))}
						<span className="hx-seg-label">Done waves</span>
					</div>
				)}
			</div>

			{hiddenColumns.length > 0 && groupBy === 'waves' && (
				<div className="hx-hidden-columns">
					<span className="hx-muted">Hidden:</span>
					{hiddenColumns.map((column) => (
						<button key={column.wave} type="button" className="hx-chip" onClick={() => {
							setHidden((current) => { const next = new Set(current); next.delete(column.wave); return next; });
							if (column.done === column.total && doneWaves === 'hide') setDoneWaves('collapse');
						}}>Wave {column.wave} · Show</button>
					))}
				</div>
			)}

			{hint && <p className="hx-board-hint" role="status">{hint}</p>}

			{groupBy === 'priority'
				? <QueueList tasks={[...visible].sort(compareTasks)} allTasks={allTasks} onPin={onPin} onOpen={onOpenTask} />
				: (
					<div className="hx-board-scroll">
						<div className="hx-board-inner" ref={boardRef}>
							{mode === 'items' && <DependencyEdges edges={plan.edges} anchors={anchors.map} width={anchors.width} height={anchors.height} criticalEdges={plan.criticalEdges} showCritical={showCritical} focusId={focusId} liveIds={liveIds} />}
							{columns.filter((column) => !column.hidden && column.total > 0).map((column) => (
								<section
									key={column.wave}
									className={['hx-wave', column.collapsed ? 'collapsed' : '', drag && drag.wave === column.wave ? 'drop-enabled' : ''].filter(Boolean).join(' ')}
									aria-label={`Wave ${column.wave}, ${column.done} of ${column.total} done`}
									onDragOver={(event) => dragOver(event, column.wave)}
									onDrop={(event) => drop(event, column.wave)}
								>
									<header className="hx-wave-head">
										<h2>Wave {column.wave} <small className="hx-num">{column.done}/{column.total}</small></h2>
										<span className="hx-wave-actions">
											<button type="button" className="hx-link" aria-expanded={!column.collapsed} onClick={() => setToggled((current) => { const next = new Set(current); if (next.has(column.wave)) next.delete(column.wave); else next.add(column.wave); return next; })}>{column.collapsed ? 'Show' : 'Collapse'}</button>
											<button type="button" className="hx-link" onClick={() => setHidden((current) => new Set(current).add(column.wave))}>Hide</button>
										</span>
									</header>
									{column.collapsed
										? <p className="hx-wave-collapsed hx-muted">{column.done === column.total ? `All ${column.total} done` : `${column.cards.length} cards`}</p>
										: mode === 'epics'
											? <EpicSummaries cards={column.cards} statusOf={statusOf} />
											: column.cards.map((task) => {
												const meta = statusOf.get(task.id);
												return (
													<TaskCard
														key={task.id}
														task={task}
														status={meta?.status ?? 'not_started'}
														waitsFor={meta?.waits ?? []}
														run={task.runId ? runs[task.runId] : undefined}
														critical={showCritical && plan.critical.has(task.id)}
														pending={pendingIds.has(task.id)}
														highlighted={Boolean(focusId) && focusId !== task.id && plan.edges.some(([from, to]) => (from === focusId && to === task.id) || (to === focusId && from === task.id))}
														registerAnchor={register}
														onOpen={onOpenTask}
														onPin={onPin}
														onHover={onHover}
														onDragStart={onDragStart}
														onDragEnd={onDragEnd}
													/>
												);
											})}
									{!column.collapsed && column.cards.length === 0 && <p className="hx-wave-collapsed hx-muted">No cards match.</p>}
								</section>
							))}
						</div>
					</div>
				)}

			{groupBy === 'waves' && (
				<p className="hx-legend hx-board-legend">
					<span>Columns are waves: a card waits for everything joined to it on its left.</span>
					{showCritical && <span className="hx-path-note">Orange rail = critical path: the longest chain of waiting, also marked “Critical” on the card.</span>}
					<span className="hx-legend-hint">Drag a card up or down its wave to put it first. Pinned cards keep their place.</span>
				</p>
			)}
		</div>
	);
}

function EpicSummaries({ cards, statusOf }: { cards: HarnessTask[]; statusOf: Map<string, { status: HxStatus; waits: string[] }> }) {
	const byEpic = new Map<string, HarnessTask[]>();
	cards.forEach((task) => byEpic.set(task.epic ?? 'No epic', [...(byEpic.get(task.epic ?? 'No epic') ?? []), task]));
	return (
		<>
			{[...byEpic].map(([epic, items]) => {
				const working = items.filter((task) => statusOf.get(task.id)?.status === 'working').length;
				const waiting = items.filter((task) => statusOf.get(task.id)?.status === 'waiting').length;
				return (
					<div className="hx-task hx-epic" key={epic}>
						<span className="hx-task-title">{epic}</span>
						<span className="hx-task-r3">{items.length} {items.length === 1 ? 'task' : 'tasks'}{working ? ` · ${working} working` : ''}{waiting ? ` · ${waiting} waiting on you` : ''}</span>
						<span className="hx-task-r3">{items.map((task) => task.source.key).join(', ')}</span>
					</div>
				);
			})}
		</>
	);
}
