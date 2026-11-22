import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, RefObject } from 'react';
import { BookmarkPlus, Bot, RotateCw, Search, SlidersHorizontal, Trash2, X } from 'lucide-react';
import { FacetMenu, SelectedFilters } from '../components/FacetMenu';
import { useModalDialog } from '../components/overlay';
import { api, statusLabels, statusOrder } from '../shared/api';
import type { ItemStatus, ItemSummary, WorkspaceConfig } from '../lib/types';
import { sourcesShownIn } from '../lib/sources';
import { emptyFilters, filterPlans, sourceFacetOptions, sourceLabel } from '../features/workstream/filtering';
import type { FacetOption, FilterKey, Filters } from '../features/workstream/filtering';
import { applyItemStatus, isDropStatus, isItemDraggable } from '../features/workstream/dragAndDrop';
import { BranchCheckoutPicker } from '../features/workstream/BranchCheckoutPicker';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { type AgentFocus } from '../features/workstream/AgentsPanel';
import { plainError } from '../features/harness/useHarness';
import { useItemSessions } from '../features/workstream/useItemSessions';
import { useBoardHarness } from '../features/workstream/useBoardHarness';
import { isSyntheticItem, joinHarness, queueOrder, taskCards, tasksForWorkspace, workedBy, workedByLabels } from '../features/workstream/harnessJoin';
import { PlanListView } from '../features/workstream/PlanListView';
import { ViewSwitcher } from '../components/ViewSwitcher';
import { NeedsYouStrip } from '../features/workstream/NeedsYouStrip';
import { RunWithHarnessDialog } from '../features/workstream/RunWithHarnessDialog';
import { QueueView } from '../features/workstream/QueueView';
import { useTaskActions } from '../features/workstream/useTaskActions';
import { RunDrawer } from '../features/harness/RunDrawer';
import { WorkspaceOwnSettings } from '../features/workspaces/WorkspaceOwnSettings';
import { canStartTask, startPending } from '../features/harness/harnessState';
import type { HarnessRunTab as HarnessRunTabId } from '../app/router';
import type { HarnessTask } from '../lib/types';
import { NewWorkItemDialog } from '../features/workstream/NewWorkItemDialog';
import { SourceItemsDialog } from '../features/workstream/SourceItemsDialog';
import { useSavedFilters } from '../features/workstream/useSavedFilters';
import { useBoardDrag } from '../features/workstream/useBoardDrag';
import '../features/harness/harness.css';
import '../features/workstream/harnessCards.css';
import { ModalLayer } from '../components/ModalLayer';
import { type NewWorkItemDraft } from '../features/workstream/newWorkItem';
import { useBoardView } from '../features/workstream/useBoardView';
import { useWorkstreamCheckout } from '../features/workstream/useWorkstreamCheckout';
import { useWorkspaceGitInfo } from '../features/workstream/useWorkspaceGitInfo';
import { boardKinds, boardLayouts, taskColumnLabels, taskStartColumn } from '../features/workstream/boardPreferences';
import { unique } from '../features/workstream/workstreamUtils';
import { WorkstreamColumn } from '../features/workstream/WorkstreamColumn';
import { PlanCard } from '../features/workstream/PlanCard';
import { PlanPreviewDrawer } from '../features/workstream/plan-drawer/PlanPreviewDrawer';

export function WorkstreamPage({ workspace, refreshKey, visibleStatuses = statusOrder, focusedItemId, focusedSessionId, onOpenPlan, onOpenHarnessRun, onWorkspacesChanged, onOpenWorkspaces, onOpenReview }: {
  workspace?: WorkspaceConfig;
  refreshKey: number;
  visibleStatuses?: ItemStatus[];
  focusedItemId?: string;
  /** A session to open in the focused card's drawer. */
  focusedSessionId?: string;
  /** Opens the item page; `run` opens it on the harness run, full width. */
  onOpenPlan: (itemId: string, view?: 'run') => void;
  /** Opens a harness run whole, full width: a plan card's on its item page, a Jira ticket's on a page of its own. */
  onOpenHarnessRun?: (runId: string) => void;
  onWorkspacesChanged: () => void | Promise<void>;
  onOpenWorkspaces?: () => void;
  onOpenReview?: () => void;
}) {
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [query, setQuery] = useState('');
  const checkout = useWorkstreamCheckout(workspace, refreshKey);
  const { items, setItems, loading, error, setError, selectedBranch, branchContext, track } = checkout;
  const [scanState, setScanState] = useState('');
  const [openFacet, setOpenFacet] = useState<FilterKey | ''>('');
  const { savedFilters, saveFilterOpen, setSaveFilterOpen, saveFilterName, setSaveFilterName, saveCurrentFilter, applySavedFilter, deleteSavedFilter } = useSavedFilters({ workspaceId: workspace?.id, filters, query, setFilters, setQuery });
  const [drawerPlanId, setDrawerPlanId] = useState('');
  const [drawerAgent, setDrawerAgent] = useState<AgentFocus>();
  /** Open while set: the New Work Item dialog, starting from these values. */
  const [newItemSeed, setNewItemSeed] = useState<Partial<NewWorkItemDraft>>();
  const [pendingItemIds, setPendingItemIds] = useState<Set<string>>(() => new Set());
  const [createdItemId, setCreatedItemId] = useState('');
  const { gitStatus: workspaceGitStatus, currentBranch: workspaceBranchCurrent, branches: workspaceBranchList } = useWorkspaceGitInfo(workspace?.id);
  const { byItem: itemAgents, reload: reloadItemAgents } = useItemSessions(workspace?.id, selectedBranch);
  const harness = useBoardHarness();
  const harnessOn = Boolean(harness.state.loaded && harness.state.status?.enabled && harness.state.status.storageSupported);
  const harnessJoin = useMemo(() => joinHarness(items, harness.state, workspace?.id), [items, harness.state, workspace?.id]);
  // Tasks and plans never share the board: Tasks shows what the harness holds, Plans what the branch holds.
  // Without the harness there are no tasks, so the board shows plans.
  const { setBoardKind, shownKind, shownLayout, setLayout } = useBoardView(harnessOn);
  const tasksShown = shownKind === 'tasks';
  const boardTaskCards = useMemo(() => taskCards(items, harness.state, workspace?.id), [items, harness.state, workspace?.id]);
  const taskWorkById = useMemo(() => new Map(boardTaskCards.map((card) => [card.item.id, card.work])), [boardTaskCards]);
  const boardItems = useMemo(() => tasksShown ? boardTaskCards.map((card) => card.item) : items, [tasksShown, boardTaskCards, items]);
  const workOf = useCallback((id: string) => (tasksShown ? taskWorkById.get(id) : undefined) ?? harnessJoin.byItemId.get(id), [tasksShown, taskWorkById, harnessJoin]);
  const workedByOf = useCallback((item: ItemSummary) => workedBy(harnessJoin.byItemId.get(item.id)), [harnessJoin]);
  const boardTasks = useMemo(() => tasksForWorkspace(harness.state, workspace?.id), [harness.state, workspace?.id]);
  const finishedTasks = useMemo(() => tasksForWorkspace(harness.state, workspace?.id, { finished: true }), [harness.state, workspace?.id]);
  const taskActions = useTaskActions(harness);
  /** A task in the first column that has not started: dragging it to the second starts it. */
  const startableTask = useCallback((id: string) => {
    if (!tasksShown) return undefined;
    const work = taskWorkById.get(id);
    const card = boardTaskCards.find((candidate) => candidate.item.id === id);
    return work && card?.item.status === 'unsorted' && canStartTask(work.task) && !startPending(work.task) && !taskActions.pendingIds.has(work.task.id) ? work.task : undefined;
  }, [tasksShown, taskWorkById, boardTaskCards, taskActions.pendingIds]);
  // A task with no card of its own opens the harness's own drawer.
  const [taskDrawer, setTaskDrawer] = useState<{ taskId: string; tab: HarnessRunTabId } | undefined>();
  // The card being handed to the harness.
  const [runDialogItemId, setRunDialogItemId] = useState('');
  const runDialogItem = runDialogItemId ? boardItems.find((item) => item.id === runDialogItemId) : undefined;
  const openTask = useCallback((task: HarnessTask) => {
    const itemId = Array.from(harnessJoin.byItemId.entries()).find(([, work]) => work.task.id === task.id)?.[0];
    if (itemId && !isSyntheticItem({ id: itemId })) {
      setDrawerPlanId(itemId);
      setDrawerAgent(undefined);
      return;
    }
    setTaskDrawer({ taskId: task.id, tab: task.runId ? 'plan' : 'ticket' });
  }, [harnessJoin]);
  const [sourceItemsOpen, setSourceItemsOpen] = useState(false);
  // The workspace's AI and harness settings, opened from the Tasks board.
  const [harnessSettingsOpen, setHarnessSettingsOpen] = useState(false);
	const harnessSettingsDialog = useModalDialog(() => { setHarnessSettingsOpen(false); void harness.refresh(); }, true, harnessSettingsOpen);
  const appliedFocusRef = useRef('');
	// Every asynchronous board workflow is scoped to this generation. A route
	// change/unmount invalidates outstanding responses before they can paint a
	// different workspace.
  const text = query;

  useEffect(() => {
    if (!focusedItemId || loading) return;
    const focusKey = `${workspace?.id ?? ''}:${focusedItemId}:${focusedSessionId ?? ''}:${selectedBranch}`;
    if (appliedFocusRef.current === focusKey) return;
    const item = items.find((candidate) => candidate.id === focusedItemId);
    if (!item) return;
    appliedFocusRef.current = focusKey;
    setQuery(item.identifier || item.title);
    setDrawerPlanId(item.id);
    setDrawerAgent(focusedSessionId ? { kind: 'session', id: focusedSessionId } : undefined);
  }, [focusedItemId, focusedSessionId, items, loading, selectedBranch, workspace?.id]);

  // Bring the freshly created card into view and let its highlight fade on its own.
  useEffect(() => {
    if (!createdItemId) return;
    const card = Array.from(document.querySelectorAll<HTMLElement>('[data-item-id]'))
      .find((candidate) => candidate.dataset.itemId === createdItemId);
    card?.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' });
    const timer = window.setTimeout(() => setCreatedItemId(''), 2600);
    return () => window.clearTimeout(timer);
  }, [createdItemId]);

  const sourceOptions = useMemo(() => sourceFacetOptions(items, workspace), [items, workspace]);
  const filteredPlans = useMemo(() => filterPlans(boardItems, tasksShown ? { ...filters, workedBy: [] } : filters, text, workspace, workedByOf), [boardItems, tasksShown, filters, text, workspace, workedByOf]);
  // The Tasks list is the Task Queue, narrowed by the same search and filters as the board.
  const shownTaskIds = useMemo(() => new Set(filteredPlans.map((item) => taskWorkById.get(item.id)?.task.id).filter(Boolean)), [filteredPlans, taskWorkById]);
  const services = useMemo(() => unique(items.map((plan) => plan.scope || 'Unknown')), [items]);
  const currentBranch = branchContext?.currentCheckoutBranch || workspaceGitStatus?.branch || workspaceBranchCurrent || workspace?.baselineBranch || 'No branch';
  const branchOptions = useMemo(() => unique([
    ...workspaceBranchList,
    currentBranch,
    workspace?.baselineBranch ?? '',
    selectedBranch
  ].filter((branch) => branch && branch !== 'No branch')), [currentBranch, selectedBranch, workspace?.baselineBranch, workspaceBranchList]);
  const authors = useMemo(() => unique(items.map((plan) => plan.author || plan.owner || 'Unknown')), [items]);
  const facetConfig: { key: FilterKey; title: string; options: FacetOption[] }[] = [
    { key: 'sources', title: 'Source', options: sourceOptions },
    { key: 'scopes', title: 'Service', options: services.map((scope) => ({ value: scope, label: scope })) },
    { key: 'statuses', title: 'Status', options: statusOrder.map((item) => ({ value: item, label: statusLabels[item] })) },
    { key: 'authors', title: 'Authors', options: authors.map((author) => ({ value: author, label: author })) },
    ...(harnessOn && !tasksShown ? [{ key: 'workedBy' as const, title: 'Worked by', options: (['harness', 'hand'] as const).map((value) => ({ value, label: workedByLabels[value] })) }] : [])
  ];
  const activeFilterCount = filters.sources.length
    + filters.scopes.length
    + filters.statuses.length
    + filters.authors.length
    + filters.workedBy.length
    + (text ? 1 : 0);
  const displayedStatuses = useMemo(() => {
    const visible = new Set(visibleStatuses);
    const statuses = statusOrder.filter((status) => visible.has(status));
    return statuses.length > 0 ? statuses : statusOrder;
  }, [visibleStatuses]);
  const boardStyle = useMemo(() => ({
    gridTemplateColumns: displayedStatuses
      // Both boards put a narrow configure column after the first one: source items on Plans, AI and harness on Tasks.
      .flatMap((status) => status === 'unsorted' && (!tasksShown || workspace) ? ['minmax(200px, 1fr)', '44px'] : ['minmax(200px, 1fr)'])
      .join(' ')
  }) as CSSProperties, [displayedStatuses, tasksShown, workspace]);
  const grouped = useMemo(() => {
    const map = new Map<ItemStatus, ItemSummary[]>();
    displayedStatuses.forEach((item) => map.set(item, []));
    filteredPlans.forEach((plan) => map.get(plan.status)?.push(plan));
    return map;
  }, [displayedStatuses, filteredPlans]);
  const preferredSourceForConfiguration = useMemo(() => {
    if (!workspace) return '';
    const unsortedSources = new Set(
      items
        .filter((item) => item.status === 'unsorted')
        .map((item) => sourceLabel(item, workspace))
        .filter(Boolean)
    );
    const firstWorkspaceUnsortedSource = sourcesShownIn(workspace, 'workstream').find((source) => unsortedSources.has(source));
    if (firstWorkspaceUnsortedSource) return firstWorkspaceUnsortedSource;
    return Array.from(unsortedSources)[0] ?? sourcesShownIn(workspace, 'workstream')[0] ?? '';
  }, [items, workspace]);
  const drag = useBoardDrag({
    tasksShown,
    canDragPlan: (itemId) => { const item = items.find((candidate) => candidate.id === itemId); return Boolean(item && isItemDraggable(item) && !pendingItemIds.has(itemId)); },
    startableTask,
    onStartTask: (task) => taskActions.start(task),
    onMovePlan: (itemId, status) => void moveItem(itemId, status)
  });
  const activeItemId = drag.activeItemId;
  const activeItem = activeItemId ? boardItems.find((item) => item.id === activeItemId) : undefined;

  const scan = async () => {
    if (!workspace) return;
		const { controller, active, finish } = checkout.begin();
		const workspaceID = workspace.id;
    // On the Tasks board, Refresh also asks the workspace's sources (Jira) at once, rather than at the next poll.
    const pollScouts = tasksShown && harnessOn ? harness.client.pollScouts : undefined;
    setScanState(pollScouts ? 'Checking Jira' : 'Refreshing');
    try {
      const [result, polled] = await Promise.all([
        api.loadWorkstreamCheckout(workspace.id, { force: true }, controller.signal),
        pollScouts ? pollScouts(workspaceID).then(async (answer) => { await harness.refresh(); return sourcesChecked(answer.sources ?? []); }, (caught: unknown) => { const message = plainError(caught, 'the check failed'); return message.includes('has no harness sources') ? '' : `Jira: ${message}`; }) : Promise.resolve('')
      ]);
			if (controller.signal.aborted || !active()) return;
      notifyReliabilityChanged();
      setScanState(polled || `${result.itemCount} items indexed`);
      checkout.apply(result);
      onWorkspacesChanged();
    } catch (err) {
		if (!controller.signal.aborted && active()) setScanState(err instanceof Error ? err.message : 'Refresh failed');
	} finally {
		finish();
    }
  };

  const reloadPlans = async (options?: { silent?: boolean }) => {
    if (!workspace) return;
    await checkout.loadCheckout(false, options);
  };

  // Show the new card on the board rather than jumping to the detail page.
  // The refresh is silent so the columns never blank out.
  const showCreatedItem = (item: ItemSummary) => {
    checkout.rememberCreated(item);
    setCreatedItemId(item.id);
    // A new work item is a plan: show the board that holds it.
    if (tasksShown) setBoardKind('plans');
    setNewItemSeed(undefined);
    void Promise.resolve(onWorkspacesChanged()).then(() => reloadPlans({ silent: true }));
  };

  const moveItem = async (itemId: string, status: ItemStatus) => {
    const item = items.find((candidate) => candidate.id === itemId);
    if (!item || !isItemDraggable(item) || !isDropStatus(status) || item.status === status || pendingItemIds.has(itemId)) return;

    const previousStatus = item.status;
		const workspaceID = workspace?.id;
		const active = track(workspaceID);
    setError('');
    setItems((current) => applyItemStatus(current, itemId, status));
    setPendingItemIds((current) => new Set(current).add(itemId));
    try {
		const result = await api.updateStatus(itemId, { status, expectedRevision: item.metadataRevision });
			if (!active()) return;
      setItems((current) => current.map((candidate) => candidate.id === itemId ? { ...candidate, ...result.item } : candidate));
      notifyReliabilityChanged();
      await onWorkspacesChanged();
    } catch (err) {
			if (!active()) return;
      setItems((current) => current.map((candidate) => (
        candidate.id === itemId && candidate.status === status ? { ...candidate, status: previousStatus } : candidate
      )));
      setError(err instanceof Error ? err.message : 'Status update failed');
    } finally {
			if (!active()) return;
      setPendingItemIds((current) => {
        const next = new Set(current);
        next.delete(itemId);
        return next;
      });
    }
  };

  const openSourceItemsDialog = () => {
    if (!workspace || sourcesShownIn(workspace, 'workstream').length === 0) {
      onOpenWorkspaces?.();
      return;
    }
    setSourceItemsOpen(true);
  };

  const toggleFilter = (key: FilterKey, value: string) => {
    setFilters((current) => {
      const values = current[key];
      return {
        ...current,
        [key]: values.includes(value) ? values.filter((item) => item !== value) : [...values, value]
      };
    });
  };

  if (!workspace && !loading) {
    return (
      <section className="empty-state">
        <h1>Workstream</h1>
        <p>Register a local Git workspace to create a workstream view.</p>
      </section>
    );
  }

  return (
    <section className="workstream-page">
      <div className="page-title workstream-title with-views">
        <h1 className="sr-only">Workstream</h1>
        {harnessOn ? <ViewSwitcher label="Show on the board" options={boardKinds} value={shownKind} onChange={setBoardKind} /> : <span />}
        {workspace && (
          <div className="workspace-context" aria-label="Workspace context">
            <BranchCheckoutPicker workspaceId={workspace.id} currentCheckoutBranch={currentBranch} branches={branchOptions} gitStatus={workspaceGitStatus} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" onSwitched={async () => { await checkout.loadCheckout(true); await onWorkspacesChanged(); }} />
            {onOpenReview && <button className="secondary" type="button" onClick={onOpenReview}>Review branch</button>}
            {sourcesShownIn(workspace, 'workstream').slice(0, 3).map((directory) => (
              <span key={directory}>{directory}</span>
            ))}
          </div>
        )}
        </div>
      <div className="board-toolbar">
        <label className="filter-input plan-search">
          <Search size={15} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search items..." />
        </label>
        <div className="board-toolbar-actions">
          <button className="secondary" onClick={scan}>
            <RotateCw size={16} /> Refresh
          </button>
          <button className="primary" onClick={() => setNewItemSeed({})}>
            + New Work Item
          </button>
          <span className="scan-state">{scanState}</span>
        </div>
      </div>
      <div className="workstream-filter-row">
      <div className="facet-bar">
        {facetConfig.map((facet) => (
          <FacetMenu
            key={facet.key}
            title={facet.title}
            options={facet.options}
            selected={filters[facet.key]}
            open={openFacet === facet.key}
            onOpen={() => setOpenFacet(openFacet === facet.key ? '' : facet.key)}
            onClose={() => setOpenFacet('')}
            onToggle={(value) => toggleFilter(facet.key, value)}
            onClear={() => setFilters((current) => ({ ...current, [facet.key]: [] }))}
          />
        ))}
      </div>
      <div className="saved-filter-bar">
        <ViewSwitcher label="Board view" options={boardLayouts} value={shownLayout} onChange={setLayout} iconOnly />
        <button className="secondary" type="button" onClick={() => setSaveFilterOpen((open) => !open)}><BookmarkPlus size={15} /> Save view</button>
        {saveFilterOpen && (
          <div className="save-filter-form">
            <input value={saveFilterName} onChange={(event) => setSaveFilterName(event.target.value)} placeholder="View name" />
            <button className="primary" type="button" disabled={!saveFilterName.trim()} onClick={() => void saveCurrentFilter()}>Save</button>
          </div>
        )}
        {savedFilters.map((saved) => (
          <span className="saved-filter" key={saved.id}>
            <button type="button" onClick={() => applySavedFilter(saved)}>{saved.name}</button>
            <button type="button" onClick={() => void deleteSavedFilter(saved.id)} aria-label={`Delete ${saved.name}`} title={`Delete ${saved.name}`}><Trash2 size={12} /></button>
          </span>
        ))}
      </div>
      </div>
      <SelectedFilters facets={facetConfig} filters={filters} onRemove={toggleFilter} />
      <div className="filter-summary">
        <span>{filteredPlans.length} of {boardItems.length} {tasksShown ? 'tasks' : 'items'}</span>
        {activeFilterCount > 0 && <span>{activeFilterCount} active filters</span>}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      {harnessOn && <NeedsYouStrip state={harness.state} onApprove={harness.approve} onReject={harness.reject} onOpenTask={openTask} />}
      {tasksShown && shownLayout === 'list' && <QueueView tasks={boardTasks.filter((task) => shownTaskIds.has(task.id))} finished={finishedTasks.filter((task) => shownTaskIds.has(task.id))} harness={harness} onOpenTask={openTask} />}
      {!tasksShown && shownLayout === 'list' && (
        <PlanListView
          items={filteredPlans}
          workspace={workspace}
          loading={loading}
          pendingIds={pendingItemIds}
          workOf={workOf}
          agentsOf={(id) => itemAgents.get(id)}
          harnessOn={harnessOn}
          onPreview={(item) => { setDrawerPlanId(item.id); setDrawerAgent(undefined); }}
          onOpen={(item) => onOpenPlan(item.id)}
          onMove={(item, status) => void moveItem(item.id, status)}
          onOpenAgent={(item, focus) => { setDrawerPlanId(item.id); setDrawerAgent(focus); }}
          onRunWithHarness={harnessOn ? (item) => { if (!harnessJoin.byItemId.has(item.id)) setRunDialogItemId(item.id); } : undefined}
        />
      )}
      {runDialogItem && <RunWithHarnessDialog item={runDialogItem} workspace={workspace} harness={harness} onClose={() => setRunDialogItemId('')} onStarted={() => { setRunDialogItemId(''); void harness.refresh(); }} />}
      {taskDrawer && (
        <RunDrawer state={harness.state} client={harness.client} taskId={taskDrawer.taskId} tab={taskDrawer.tab} queue={queueOrder(harness.state, workspace?.id)} onTabChange={(tab) => setTaskDrawer({ ...taskDrawer, tab })} onClose={() => setTaskDrawer(undefined)} onOpenDetails={onOpenHarnessRun} onCancel={harness.cancelRun} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onPin={taskActions.pin} onMove={taskActions.move} onStart={taskActions.start} onReset={harness.resetTask} onRemove={(task) => harness.removeTask(task).then(() => setTaskDrawer(undefined))} onSetQuestions={harness.setQuestions} askDefault={harness.state.status?.askQuestions ?? true} />
      )}
      {shownLayout === 'board' && <div className={tasksShown ? 'workstream-board task-board' : 'workstream-board'} style={boardStyle} aria-busy={loading}>
        {displayedStatuses.map((column) => (
          <Fragment key={column}>
            <WorkstreamColumn
              status={column}
              itemCount={grouped.get(column)?.length ?? 0}
              loading={loading}
              label={tasksShown ? taskColumnLabels[column] : undefined}
              dragActive={Boolean(activeItem) && (!tasksShown || column === taskStartColumn)}
              dragTargetStatus={drag.dragTargetStatus}
              onDragOver={drag.onColumnDragOver}
              onDragLeave={drag.onColumnDragLeave}
              onDrop={drag.onColumnDrop}
              onCreate={column === 'unsorted' || tasksShown ? undefined : () => {
                setNewItemSeed({ status: column, source: sourcesShownIn(workspace, 'workstream')[0] ?? '' });
              }}
            >
              {!loading && grouped.get(column)?.map((plan) => (
                <PlanCard
                  key={plan.id}
                  item={plan}
                  workspace={workspace}
                  pending={pendingItemIds.has(plan.id)}
                  active={activeItemId === plan.id}
                  created={createdItemId === plan.id}
                  onDragStart={(event) => drag.onCardDragStart(event, plan.id)}
                  onDragEnd={() => drag.onCardDragEnd(plan.id)}
                  onPreview={() => {
                    if (!drag.previewAllowed(plan.id)) return;
                    setDrawerPlanId(plan.id);
                    setDrawerAgent(undefined);
                  }}
                  agents={itemAgents.get(plan.id)}
                  work={workOf(plan.id)}
                  harnessOn={harnessOn}
                  autoStatus={tasksShown}
                  startable={Boolean(startableTask(plan.id))}
                  onRunWithHarness={harnessOn && !tasksShown && !harnessJoin.byItemId.has(plan.id) ? () => setRunDialogItemId(plan.id) : undefined}
                  onOpenTask={isSyntheticItem(plan) && workOf(plan.id) ? () => openTask(workOf(plan.id)!.task) : undefined}
                  onOpenAgent={(focus) => {
                    setDrawerPlanId(plan.id);
                    setDrawerAgent(focus);
                  }}
                  onOpen={() => onOpenPlan(plan.id)}
                  onMove={(status) => moveItem(plan.id, status)}
                />
              ))}
            </WorkstreamColumn>
            {column === 'unsorted' && tasksShown && workspace && (
              <button className="workstream-separator" type="button" onClick={() => setHarnessSettingsOpen(true)} title="Configure AI and harness">
                <span className="separator-arrow">▶</span>
                <span className="separator-count">{grouped.get('unsorted')?.length ?? 0}</span>
                <span className="separator-label">Configure AI & harness</span>
                <Bot size={15} />
              </button>
            )}
            {column === 'unsorted' && !tasksShown && (
              <button className="workstream-separator" type="button" onClick={() => void openSourceItemsDialog()} title="Configure source items">
                <span className="separator-arrow">▶</span>
                <span className="separator-count">{grouped.get('unsorted')?.length ?? 0}</span>
                <span className="separator-label">Configure source items</span>
                <SlidersHorizontal size={15} />
              </button>
            )}
          </Fragment>
        ))}
      </div>}
      {drawerPlanId && (
        <PlanPreviewDrawer
          itemId={drawerPlanId}
          refreshKey={refreshKey}
          workspaceId={workspace?.id}
          agents={itemAgents.get(drawerPlanId)}
          agentFocus={drawerAgent}
          onAgentFocus={setDrawerAgent}
          onAgentsChanged={reloadItemAgents}
          work={harnessJoin.byItemId.get(drawerPlanId)}
          harness={harness}
          onRunWithHarness={harnessOn ? () => setRunDialogItemId(drawerPlanId) : undefined}
          onClose={() => { setDrawerPlanId(''); setDrawerAgent(undefined); }}
          onOpenFull={() => onOpenPlan(drawerPlanId)}
          onOpenRun={() => onOpenPlan(drawerPlanId, 'run')}
          onChanged={async () => {
            await onWorkspacesChanged();
            await reloadPlans();
          }}
        />
      )}
      {harnessSettingsOpen && workspace && (
        <ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) { setHarnessSettingsOpen(false); void harness.refresh(); } }}>
          <div ref={harnessSettingsDialog.ref as RefObject<HTMLDivElement>} className="modal-panel workspace-settings-modal" role="dialog" aria-modal="true" aria-label="AI and harness">
            <header>
              <div>
                <h2>AI and harness</h2>
                <span>{workspace.name}</span>
              </div>
              <button data-autofocus className="icon-button" type="button" onClick={() => { setHarnessSettingsOpen(false); void harness.refresh(); }} aria-label="Close AI and harness">
                <X size={16} />
              </button>
            </header>
            <WorkspaceOwnSettings workspace={workspace} />
          </div>
        </div></ModalLayer>
      )}
      {sourceItemsOpen && workspace && (
        <SourceItemsDialog workspace={workspace} initialDirectory={preferredSourceForConfiguration} onClose={() => setSourceItemsOpen(false)} onSaved={async (message, done) => {
          setScanState(message);
          if (done) setSourceItemsOpen(false);
          await onWorkspacesChanged();
          await reloadPlans();
        }} />
      )}
      {newItemSeed && workspace && (
        <NewWorkItemDialog workspace={workspace} seed={newItemSeed} onClose={() => setNewItemSeed(undefined)} onCreated={showCreatedItem} />
      )}
    </section>
  );
}

/** What a Refresh on the Tasks board says the sources returned. */
export function sourcesChecked(sources: { found: number; added: number; lastError?: string }[]): string {
  const failed = sources.find((source) => source.lastError);
  if (failed) return `Jira: ${failed.lastError}`;
  const found = sources.reduce((sum, source) => sum + source.found, 0);
  const added = sources.reduce((sum, source) => sum + source.added, 0);
  return `Jira checked: ${found} found, ${added} new`;
}
