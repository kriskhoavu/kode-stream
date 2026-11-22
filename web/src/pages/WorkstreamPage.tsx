import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, DragEvent, RefObject } from 'react';
import { BookmarkPlus, Bot, RotateCw, Search, SlidersHorizontal, Trash2, X } from 'lucide-react';
import { FacetMenu, SelectedFilters } from '../components/FacetMenu';
import { useModalDialog } from '../components/overlay';
import { writePreference } from '../shared/preferences/store';
import { StatusMenu } from '../components/StatusMenu';
import { api, statusLabels, statusOrder } from '../shared/api';
import type { WorkstreamBranchLoadResult, GitStatus, ItemStatus, ItemSummary, JiraIssueState, SavedFilter, SourceStructureSettings, WorkspaceConfig } from '../lib/types';
import { labels } from '../lib/vocabulary';
import { sourcesShownIn } from '../lib/sources';
import { emptyFilters, filterPlans, sourceFacetOptions, sourceLabel } from '../features/workstream/filtering';
import type { FacetOption, FilterKey, Filters } from '../features/workstream/filtering';
import { applyItemStatus, isDropStatus, isItemDraggable } from '../features/workstream/dragAndDrop';
import { BranchCheckoutPicker } from '../features/workstream/BranchCheckoutPicker';
import { MarkdownPreview } from '../features/content-viewer/renderers/MarkdownPreview';
import { canonicalSourceSettingsCard } from '../features/workspaces/sourceSettingsEditor';
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
import '../features/harness/harness.css';
import '../features/workstream/harnessCards.css';
import { ModalLayer } from '../components/ModalLayer';
import { SourceStructureProposalList, SourceStructurePreviewTable, sourceItemsEditorFromResult, applySourceItemsProposal, clearSourceItemsProposal, updateSourceItemsPreviewField, type SourceItemsEditorState, UNSORTED_SELECTION_ID } from '../features/workstream/sourceItemsEditor';
import { type NewWorkItemDraft, emptyNewWorkItemDraft, parseTags, jiraTags, jiraReadme } from '../features/workstream/newWorkItem';
import { type BoardKind, type BoardLayout, boardKinds, boardLayouts, boardKindPreference, layoutPreferences, taskColumnLabels, taskStartColumn, readBoardKind, readBoardLayout } from '../features/workstream/boardPreferences';
import { unique } from '../features/workstream/workstreamUtils';
import { WorkstreamColumn } from '../features/workstream/WorkstreamColumn';
import { PlanCard } from '../features/workstream/PlanCard';
import { PlanPreviewDrawer } from '../features/workstream/plan-drawer/PlanPreviewDrawer';

const DRAG_CLICK_SUPPRESSION_MS = 350;
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
  const [items, setPlans] = useState<ItemSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const loadedWorkspaceRef = useRef<string | undefined>(undefined);
  const [error, setError] = useState('');
  const [scanState, setScanState] = useState('');
  const [openFacet, setOpenFacet] = useState<FilterKey | ''>('');
  const [drawerPlanId, setDrawerPlanId] = useState('');
  const [drawerAgent, setDrawerAgent] = useState<AgentFocus>();
  const [newPlanOpen, setNewPlanOpen] = useState(false);
  const [newPlanDraft, setNewPlanDraft] = useState<NewWorkItemDraft>(() => emptyNewWorkItemDraft());
  const [newPlanError, setNewPlanError] = useState('');
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [jiraLookup, setJiraLookup] = useState<JiraIssueState | null>(null);
  const [jiraLookupLoading, setJiraLookupLoading] = useState(false);
  const [savedFilters, setSavedFilters] = useState<SavedFilter[]>([]);
  const [saveFilterOpen, setSaveFilterOpen] = useState(false);
  const [saveFilterName, setSaveFilterName] = useState('');
  const [pendingItemIds, setPendingItemIds] = useState<Set<string>>(() => new Set());
  const [activeItemId, setActiveItemId] = useState('');
  const [createdItemId, setCreatedItemId] = useState('');
  // Holds a just-created item until a reload reports it, so a rescan that has
  // not caught up yet cannot make the new card flash and vanish.
  const pendingCreatedItemRef = useRef<ItemSummary | null>(null);
  const [dragTargetStatus, setDragTargetStatus] = useState<ItemStatus | ''>('');
  const [workspaceGitStatus, setWorkspaceGitStatus] = useState<GitStatus | null>(null);
  const [workspaceBranchCurrent, setWorkspaceBranchCurrent] = useState('');
  const [workspaceBranchList, setWorkspaceBranchList] = useState<string[]>([]);
  const [selectedBranch, setSelectedBranch] = useState('');
  const [branchContext, setBranchContext] = useState<WorkstreamBranchLoadResult | null>(null);
  const { byItem: itemAgents, reload: reloadItemAgents } = useItemSessions(workspace?.id, selectedBranch);
  const harness = useBoardHarness();
  const harnessOn = Boolean(harness.state.loaded && harness.state.status?.enabled && harness.state.status.storageSupported);
  const harnessJoin = useMemo(() => joinHarness(items, harness.state, workspace?.id), [items, harness.state, workspace?.id]);
  // Tasks and plans never share the board: Tasks shows what the harness holds, Plans what the branch holds.
  // Without the harness there are no tasks, so the board shows plans.
  const [boardKind, setBoardKindState] = useState<BoardKind>(() => readBoardKind());
  const setBoardKind = (kind: BoardKind) => { setBoardKindState(kind); writePreference(boardKindPreference, kind); };
  const shownKind: BoardKind = harnessOn ? boardKind : 'plans';
  const [layouts, setLayouts] = useState<Record<BoardKind, BoardLayout>>(() => ({ tasks: readBoardLayout('tasks'), plans: readBoardLayout('plans') }));
  const setLayout = (layout: BoardLayout) => { setLayouts((current) => ({ ...current, [shownKind]: layout })); writePreference(layoutPreferences[shownKind], layout); };
  const shownLayout = layouts[shownKind];
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
  const [sourceItemsLoading, setSourceItemsLoading] = useState(false);
  const [sourceItemsSaving, setSourceItemsSaving] = useState(false);
  const [sourceItemsDirectory, setSourceItemsDirectory] = useState('');
  const [sourceItemsError, setSourceItemsError] = useState('');
  const [sourceItemsEditor, setSourceItemsEditor] = useState<SourceItemsEditorState | null>(null);
	const sourceItemsDialog = useModalDialog(() => setSourceItemsOpen(false), !sourceItemsSaving, sourceItemsOpen);
	const harnessSettingsDialog = useModalDialog(() => { setHarnessSettingsOpen(false); void harness.refresh(); }, true, harnessSettingsOpen);
	const newItemDialog = useModalDialog(() => closeNewPlan(), !creatingPlan, newPlanOpen);
  const suppressPreviewRef = useRef<{ itemId: string; until: number } | null>(null);
  const appliedFocusRef = useRef('');
	// Every asynchronous board workflow is scoped to this generation. A route
	// change/unmount invalidates outstanding responses before they can paint a
	// different workspace.
	const workspaceGenerationRef = useRef(0);
	const activeWorkspaceRef = useRef<string | undefined>(workspace?.id);
	const jiraLookupAbortRef = useRef<AbortController | null>(null);
	const checkoutAbortRef = useRef<AbortController | undefined>(undefined);
	const closeNewPlan = () => {
		jiraLookupAbortRef.current?.abort();
		jiraLookupAbortRef.current = null;
		setJiraLookupLoading(false);
		setNewPlanOpen(false);
		setJiraLookup(null);
		setNewPlanError('');
	};
	useEffect(() => {
		activeWorkspaceRef.current = workspace?.id;
		workspaceGenerationRef.current += 1;
		return () => { workspaceGenerationRef.current += 1; jiraLookupAbortRef.current?.abort(); checkoutAbortRef.current?.abort(); };
	}, [workspace?.id]);
  const text = query;

  const withPendingCreated = (loaded: ItemSummary[]) => {
    const pending = pendingCreatedItemRef.current;
    if (!pending) return loaded;
    if (loaded.some((candidate) => candidate.id === pending.id)) {
      pendingCreatedItemRef.current = null;
      return loaded;
    }
    return [pending, ...loaded];
  };

  // silent keeps the board on screen while it refreshes, so a background reload
  // never blanks the columns the user is already looking at.
  const loadCheckout = async (force = false, { silent = false }: { silent?: boolean } = {}) => {
    if (!workspace) return;
		checkoutAbortRef.current?.abort();
		const controller = new AbortController();
		checkoutAbortRef.current = controller;
		const workspaceID = workspace.id;
		const generation = workspaceGenerationRef.current;
		const active = () => activeWorkspaceRef.current === workspaceID && workspaceGenerationRef.current === generation;
    if (!silent) setLoading(true);
    setError('');
    try {
      const result = await api.loadWorkstreamCheckout(workspace.id, { force }, controller.signal);
			if (controller.signal.aborted || !active()) return;
      setBranchContext(result);
      setSelectedBranch(result.branch);
      setPlans(withPendingCreated(result.items));
    } catch (err) {
			if (controller.signal.aborted) return;
      try {
        const fallbackItems = await api.items(new URLSearchParams({ workspaceId: workspace.id }), controller.signal);
			if (controller.signal.aborted || !active()) return;
        setBranchContext(null);
        setSelectedBranch(workspace.baselineBranch);
        setPlans(fallbackItems);
        setError('');
      } catch {
			if (controller.signal.aborted || !active()) return;
        setError(err instanceof Error ? err.message : 'Failed to load checkout');
        setPlans([]);
      }
    } finally {
		if (!controller.signal.aborted && active()) setLoading(false);
		if (checkoutAbortRef.current === controller) checkoutAbortRef.current = undefined;
    }
  };

  useEffect(() => {
    if (!workspace) {
      setPlans([]);
      setBranchContext(null);
      setSelectedBranch('');
      setLoading(false);
      return;
    }
    // A refreshKey bump is the app refreshing (on window focus, among others),
    // not a new board: reload the same workspace silently so the cards stay.
    const sameWorkspace = loadedWorkspaceRef.current === workspace.id;
    loadedWorkspaceRef.current = workspace.id;
    void loadCheckout(false, { silent: sameWorkspace });
  }, [workspace?.id, refreshKey]);

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

  useEffect(() => {
    if (!newPlanOpen && !sourceItemsOpen) return;
    const previousBodyOverflow = document.body.style.overflow;
    const mainContent = document.querySelector<HTMLElement>('.main-content');
    const previousMainContentOverflow = mainContent?.style.overflow ?? '';
    document.body.style.overflow = 'hidden';
    if (mainContent) mainContent.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previousBodyOverflow;
      if (mainContent) mainContent.style.overflow = previousMainContentOverflow;
    };
  }, [newPlanOpen, sourceItemsOpen]);

  useEffect(() => {
    if (!newPlanOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
			closeNewPlan();
    };
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [newPlanOpen]);

  useEffect(() => {
    const controller = new AbortController();
    api.savedFilters(controller.signal)
      .then((saved) => {
        if (!controller.signal.aborted) setSavedFilters(saved.filter((filter) => !filter.workspaceId || filter.workspaceId === workspace?.id));
      })
      .catch(() => {
        if (!controller.signal.aborted) setSavedFilters([]);
      });
    return () => controller.abort();
  }, [workspace?.id]);

  useEffect(() => {
    if (!workspace) {
      setWorkspaceGitStatus(null);
      setWorkspaceBranchCurrent('');
      setWorkspaceBranchList([]);
      return;
    }
    const controller = new AbortController();
    api.gitStatus(workspace.id, controller.signal)
      .then((status) => {
        if (!controller.signal.aborted) setWorkspaceGitStatus(status);
      })
      .catch(() => {
        if (!controller.signal.aborted) setWorkspaceGitStatus(null);
      });
    api.workspaceBranches(workspace.id, controller.signal)
      .then((response) => {
        if (controller.signal.aborted) return;
        setWorkspaceBranchCurrent(response.current);
        setWorkspaceBranchList(response.branches);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setWorkspaceBranchCurrent('');
        setWorkspaceBranchList([]);
      });
    return () => controller.abort();
  }, [workspace?.id]);

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
  const activeItem = activeItemId ? boardItems.find((item) => item.id === activeItemId) : undefined;

  const scan = async () => {
    if (!workspace) return;
		checkoutAbortRef.current?.abort();
		const controller = new AbortController();
		checkoutAbortRef.current = controller;
		const workspaceID = workspace.id;
		const generation = workspaceGenerationRef.current;
		const active = () => activeWorkspaceRef.current === workspaceID && workspaceGenerationRef.current === generation;
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
      setBranchContext(result);
      setSelectedBranch(result.branch);
      setPlans(result.items);
      onWorkspacesChanged();
    } catch (err) {
		if (!controller.signal.aborted && active()) setScanState(err instanceof Error ? err.message : 'Refresh failed');
	} finally {
		if (checkoutAbortRef.current === controller) checkoutAbortRef.current = undefined;
    }
  };

  const reloadPlans = async (options?: { silent?: boolean }) => {
    if (!workspace) return;
    await loadCheckout(false, options);
  };

  const moveItem = async (itemId: string, status: ItemStatus) => {
    const item = items.find((candidate) => candidate.id === itemId);
    if (!item || !isItemDraggable(item) || !isDropStatus(status) || item.status === status || pendingItemIds.has(itemId)) return;

    const previousStatus = item.status;
		const workspaceID = workspace?.id;
		const generation = workspaceGenerationRef.current;
		const active = () => activeWorkspaceRef.current === workspaceID && workspaceGenerationRef.current === generation;
    setError('');
    setPlans((current) => applyItemStatus(current, itemId, status));
    setPendingItemIds((current) => new Set(current).add(itemId));
    try {
		const result = await api.updateStatus(itemId, { status, expectedRevision: item.metadataRevision });
			if (!active()) return;
      setPlans((current) => current.map((candidate) => candidate.id === itemId ? { ...candidate, ...result.item } : candidate));
      notifyReliabilityChanged();
      await onWorkspacesChanged();
    } catch (err) {
			if (!active()) return;
      setPlans((current) => current.map((candidate) => (
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

  const handleCardDragStart = (event: DragEvent<HTMLElement>, itemId: string) => {
    if (tasksShown) {
      if (!startableTask(itemId)) { event.preventDefault(); return; }
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('text/plain', itemId);
      setActiveItemId(itemId);
      suppressPreviewRef.current = { itemId, until: Number.POSITIVE_INFINITY };
      return;
    }
    const item = items.find((candidate) => candidate.id === itemId);
    if (!item || !isItemDraggable(item) || pendingItemIds.has(itemId)) {
      event.preventDefault();
      return;
    }
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', itemId);
    setActiveItemId(itemId);
    suppressPreviewRef.current = { itemId, until: Number.POSITIVE_INFINITY };
  };

  const finishDrag = (itemId: string) => {
    suppressPreviewRef.current = { itemId, until: window.performance.now() + DRAG_CLICK_SUPPRESSION_MS };
    setActiveItemId('');
    setDragTargetStatus('');
  };

  const handleCardDragEnd = (itemId: string) => {
    finishDrag(itemId);
  };

  const handleColumnDragOver = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
    if (!activeItemId || !isDropStatus(status) || (tasksShown && status !== taskStartColumn)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    if (dragTargetStatus !== status) setDragTargetStatus(status);
  };

  const handleColumnDragLeave = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
    if (dragTargetStatus === status && !event.currentTarget.contains(event.relatedTarget as Node | null)) {
      setDragTargetStatus('');
    }
  };

  const handleColumnDrop = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
    const itemId = event.dataTransfer.getData('text/plain') || activeItemId;
    if (!itemId || !isDropStatus(status)) return;
    if (tasksShown) {
      const task = status === taskStartColumn ? startableTask(itemId) : undefined;
      if (!task) return;
      event.preventDefault();
      setDragTargetStatus('');
      taskActions.start(task);
      return;
    }
    event.preventDefault();
    setDragTargetStatus('');
    void moveItem(itemId, status);
  };

  const fetchJiraIssue = async () => {
    if (!workspace || !newPlanDraft.jiraKey.trim()) return;
		jiraLookupAbortRef.current?.abort();
		const controller = new AbortController();
		jiraLookupAbortRef.current = controller;
		const workspaceID = workspace.id;
		const generation = workspaceGenerationRef.current;
		const active = () => activeWorkspaceRef.current === workspaceID && workspaceGenerationRef.current === generation;
    setJiraLookupLoading(true);
    setNewPlanError('');
    try {
			const result = await api.workspaceJiraIssue(workspace.id, newPlanDraft.jiraKey.trim(), controller.signal);
			if (!active()) return;
      setJiraLookup(result);
      if (result.state === 'available' && result.issue) {
        const issue = result.issue;
        const nextTags = jiraTags(issue).join(', ');
        // Fetching is an explicit request for this ticket's values, so they
        // replace whatever a previous fetch imported. Keeping the old values
        // left the details panel describing a different ticket than the preview.
        setNewPlanDraft((draft) => ({
          ...draft,
          identifier: issue.key,
          title: issue.summary,
          owner: issue.assignee?.displayName || '',
          tags: nextTags
        }));
      }
		} catch (err) {
			if (controller.signal.aborted) return;
			if (!active()) return;
      setJiraLookup(null);
      setNewPlanError(err instanceof Error ? err.message : 'Jira lookup failed');
    } finally {
		if (active()) setJiraLookupLoading(false);
    }
  };

  const createPlan = async () => {
    if (!workspace) return;
    setCreatingPlan(true);
    setNewPlanError('');
    try {
      const source = newPlanDraft.source || sourcesShownIn(workspace, 'workstream')[0] || 'plans';
      const jiraIssue = newPlanDraft.origin === 'jira' && jiraLookup?.state === 'available' ? jiraLookup.issue : undefined;
      const result = await api.createItem({
        workspaceId: workspace.id,
        source,
        scope: newPlanDraft.scope.trim() || source,
        identifier: newPlanDraft.identifier.trim(),
        title: newPlanDraft.title.trim() || undefined,
        status: newPlanDraft.status,
        owner: newPlanDraft.owner.trim() || undefined,
        tags: parseTags(newPlanDraft.tags),
        jiraKey: jiraIssue?.key,
        initialReadme: jiraIssue ? jiraReadme(jiraIssue) : undefined
      });
      notifyReliabilityChanged();
      // Show the new card on the board rather than jumping to the detail page.
      // The refresh is silent so the columns never blank out, and the dialog
      // stays up until the board behind it already holds the item.
      pendingCreatedItemRef.current = result.item;
      setPlans((current) => current.some((candidate) => candidate.id === result.item.id)
        ? current.map((candidate) => (candidate.id === result.item.id ? { ...candidate, ...result.item } : candidate))
        : [result.item, ...current]);
      setCreatedItemId(result.item.id);
      // A new work item is a plan: show the board that holds it.
      if (tasksShown) setBoardKind('plans');
      setNewPlanOpen(false);
      setNewPlanDraft(emptyNewWorkItemDraft());
      setJiraLookup(null);
      void Promise.resolve(onWorkspacesChanged()).then(() => reloadPlans({ silent: true }));
    } catch (err) {
      setNewPlanError(err instanceof Error ? err.message : 'Item creation failed');
    } finally {
      setCreatingPlan(false);
    }
  };

  const loadSourceItemsSettings = async (directory: string) => {
    if (!workspace) return;
		const workspaceID = workspace.id;
		const generation = workspaceGenerationRef.current;
		const active = () => activeWorkspaceRef.current === workspaceID && workspaceGenerationRef.current === generation;
    setSourceItemsLoading(true);
    setSourceItemsError('');
    setSourceItemsDirectory(directory);
    try {
      const result = await api.sourceStructure(workspace.id, directory);
			if (!active()) return;
      setSourceItemsEditor(sourceItemsEditorFromResult(workspace, directory, result));
    } catch (err) {
			if (!active()) return;
      setSourceItemsEditor(null);
      setSourceItemsError(err instanceof Error ? err.message : 'Source settings failed to load');
    } finally {
		if (active()) setSourceItemsLoading(false);
    }
  };

  const openSourceItemsDialog = async () => {
    if (!workspace || sourcesShownIn(workspace, 'workstream').length === 0) {
      onOpenWorkspaces?.();
      return;
    }
    const initialDirectory = preferredSourceForConfiguration;
    setSourceItemsOpen(true);
    void loadSourceItemsSettings(initialDirectory);
  };

  const saveSourceItemsSettings = async () => {
    if (!sourceItemsEditor) return;
    setSourceItemsSaving(true);
    setSourceItemsError('');
    try {
      if (sourceItemsEditor.selectedProposalId === UNSORTED_SELECTION_ID) {
        if (sourceItemsEditor.exists) {
          await api.resetSourceStructure(sourceItemsEditor.workspace.id, sourceItemsEditor.directory);
          setScanState('Source structure reset');
        } else {
          const result = await api.scan(sourceItemsEditor.workspace.id);
          setScanState(`${result.itemCount} items indexed`);
        }
      } else {
        const settings: SourceStructureSettings = {
          version: 1,
          cards: [canonicalSourceSettingsCard(sourceItemsEditor.card), ...sourceItemsEditor.cards.slice(1).map(canonicalSourceSettingsCard)]
        };
        await api.saveSourceStructure(sourceItemsEditor.workspace.id, sourceItemsEditor.directory, settings);
        setScanState('Source structure saved');
      }
      notifyReliabilityChanged();
      setSourceItemsOpen(false);
      await onWorkspacesChanged();
      await reloadPlans();
    } catch (err) {
      setSourceItemsError(err instanceof Error ? err.message : 'Source settings failed to save');
    } finally {
      setSourceItemsSaving(false);
    }
  };

  const resetSourceItemsSettings = async () => {
    if (!sourceItemsEditor) return;
    const confirmed = window.confirm(`Reset Source Items for ${sourceItemsEditor.directory}? This removes the source's rules and scans the source again.`);
    if (!confirmed) return;
    setSourceItemsSaving(true);
    setSourceItemsError('');
    try {
      const result = await api.resetSourceStructure(sourceItemsEditor.workspace.id, sourceItemsEditor.directory);
      notifyReliabilityChanged();
      setSourceItemsEditor(sourceItemsEditorFromResult(sourceItemsEditor.workspace, sourceItemsEditor.directory, result));
      setScanState('Source structure reset');
      await onWorkspacesChanged();
      await reloadPlans();
    } catch (err) {
      setSourceItemsError(err instanceof Error ? err.message : 'Source settings failed to reset');
    } finally {
      setSourceItemsSaving(false);
    }
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

  const saveCurrentFilter = async () => {
    if (!saveFilterName.trim()) return;
    const saved = await api.saveFilter({
      name: saveFilterName.trim(),
      route: '/workstream',
      workspaceId: workspace?.id,
      filters: { filters: { ...filters, branches: [] }, query }
    });
    setSavedFilters((current) => [saved, ...current]);
    setSaveFilterName('');
    setSaveFilterOpen(false);
  };

  const applySavedFilter = (saved: SavedFilter) => {
    const value = saved.filters as { filters?: Partial<Filters>; query?: string };
    const nextFilters = { ...emptyFilters, ...(value.filters ?? {}) };
    nextFilters.branches = [];
    setFilters(nextFilters);
    setQuery(value.query ?? '');
  };

  const deleteSavedFilter = async (id: string) => {
    await api.deleteFilter(id);
    setSavedFilters((current) => current.filter((filter) => filter.id !== id));
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
            <BranchCheckoutPicker workspaceId={workspace.id} currentCheckoutBranch={currentBranch} branches={branchOptions} gitStatus={workspaceGitStatus} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" onSwitched={async () => { await loadCheckout(true); await onWorkspacesChanged(); }} />
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
          <button className="primary" onClick={() => setNewPlanOpen(true)}>
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
              dragTargetStatus={dragTargetStatus}
              onDragOver={handleColumnDragOver}
              onDragLeave={handleColumnDragLeave}
              onDrop={handleColumnDrop}
              onCreate={column === 'unsorted' || tasksShown ? undefined : () => {
                setNewPlanDraft((draft) => ({ ...draft, status: column, source: sourcesShownIn(workspace, 'workstream')[0] ?? '' }));
                setNewPlanOpen(true);
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
                  onDragStart={(event) => handleCardDragStart(event, plan.id)}
                  onDragEnd={() => handleCardDragEnd(plan.id)}
                  onPreview={() => {
                    const suppressed = suppressPreviewRef.current;
                    if (suppressed?.itemId === plan.id && window.performance.now() <= suppressed.until) {
                      return;
                    }
                    suppressPreviewRef.current = null;
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
        <ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !sourceItemsSaving) setSourceItemsOpen(false); }}>
          <div ref={sourceItemsDialog.ref as RefObject<HTMLDivElement>} className="modal-panel source-structure-modal" role="dialog" aria-modal="true" aria-label={labels.sourceStructure}>
            <header>
              <div>
                <h2>{labels.sourceStructure}</h2>
                <span>{workspace.name} / {sourceItemsDirectory || sourcesShownIn(workspace, 'workstream')[0]}</span>
              </div>
              <button data-autofocus className="icon-button" type="button" onClick={() => setSourceItemsOpen(false)} disabled={sourceItemsSaving} aria-label="Close source items">
                <X size={16} />
              </button>
            </header>
            {sourcesShownIn(workspace, 'workstream').length > 1 && (
              <label className="repo-field">
                Source
                <select value={sourceItemsDirectory} onChange={(event) => void loadSourceItemsSettings(event.target.value)} disabled={sourceItemsLoading || sourceItemsSaving}>
                  {sourcesShownIn(workspace, 'workstream').map((directory) => <option key={directory} value={directory}>{directory}</option>)}
                </select>
              </label>
            )}
            <p className="modal-help">Define how this source should be split into board items.</p>
            {sourceItemsLoading && <span className="reliability-muted">Loading source settings...</span>}
            {!sourceItemsLoading && sourceItemsError && <span className="error">{sourceItemsError}</span>}
            {!sourceItemsLoading && sourceItemsEditor && (
              <>
                {!sourceItemsEditor.exists && sourceItemsEditor.mode === 'structured' && (
                  <div className="metadata-callout source-structure-supported">
                    <strong>Built-in structure detected</strong>
                    <span>This source already follows a supported item layout. Saving here creates an optional override.</span>
                  </div>
                )}
                {!sourceItemsEditor.exists && sourceItemsEditor.mode !== 'structured' && (
                  <div className="metadata-callout">
                    <strong>No settings file yet</strong>
                    <span>Saving adds this source's rules to <code>.kode-stream/workspace-settings.yaml</code>.</span>
                  </div>
                )}
                {sourceItemsEditor.warnings.length > 0 && (
                  <div className="plan-warnings">
                    <h3>Warnings</h3>
                    {sourceItemsEditor.warnings.map((warning) => <p key={warning}>{warning}</p>)}
                  </div>
                )}
                <SourceStructureProposalList
                  proposals={sourceItemsEditor.proposals}
                  selectedProposalId={sourceItemsEditor.selectedProposalId}
                  onSelect={(proposal) => applySourceItemsProposal(setSourceItemsEditor, proposal)}
                  onClear={() => clearSourceItemsProposal(setSourceItemsEditor)}
                />
                <SourceStructurePreviewTable
                  preview={sourceItemsEditor.preview}
                  onChangeField={(path, field, value) => updateSourceItemsPreviewField(setSourceItemsEditor, path, field, value)}
                />
              </>
            )}
            <footer className="modal-actions">
              {sourceItemsEditor?.exists && (
                <button className="secondary danger" type="button" onClick={() => void resetSourceItemsSettings()} disabled={sourceItemsSaving || sourceItemsLoading}>
                  Reset config
                </button>
              )}
              <button className="secondary" type="button" onClick={() => setSourceItemsOpen(false)} disabled={sourceItemsSaving}>Cancel</button>
              <button className="primary" type="button" onClick={() => void saveSourceItemsSettings()} disabled={sourceItemsSaving || sourceItemsLoading || !sourceItemsEditor}>
                <SlidersHorizontal size={15} />
                {sourceItemsSaving ? 'Saving...' : sourceItemsEditor?.selectedProposalId === UNSORTED_SELECTION_ID ? 'Scan Unsorted' : 'Save and Scan'}
              </button>
            </footer>
          </div>
        </div></ModalLayer>
      )}
      {newPlanOpen && workspace && (
        <ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !creatingPlan) closeNewPlan(); }}>
          <section ref={newItemDialog.ref} className={newPlanDraft.origin === 'jira' ? 'modal-panel new-work-item-modal jira-mode' : 'modal-panel new-work-item-modal'} role="dialog" aria-modal="true" aria-label="Create new work item">
            <header>
              <h2>New Work Item</h2>
				<button data-autofocus type="button" className="icon-button" onClick={() => {
				closeNewPlan();
              }}><X size={16} /></button>
            </header>
            <div className={newPlanDraft.origin === 'jira' ? 'metadata-form jira-work-item-form' : 'metadata-form'}>
              <div className="segmented-control" role="group" aria-label="Item origin">
                <button
                  type="button"
                  className={newPlanDraft.origin === 'blank' ? 'active' : ''}
                  onClick={() => {
					jiraLookupAbortRef.current?.abort();
					jiraLookupAbortRef.current = null;
					setJiraLookupLoading(false);
					setNewPlanDraft((draft) => ({ ...emptyNewWorkItemDraft(), source: draft.source, origin: 'blank' }));
                    setJiraLookup(null);
                    setNewPlanError('');
                  }}
                >
                  Blank
                </button>
                <button
                  type="button"
                  className={newPlanDraft.origin === 'jira' ? 'active' : ''}
                  onClick={() => {
                    setNewPlanDraft((draft) => ({ ...draft, origin: 'jira' }));
                    setNewPlanError('');
                  }}
                >
                  From Jira
                </button>
              </div>
              {newPlanDraft.origin === 'blank' ? (
                <>
                  <label>Source<select value={newPlanDraft.source || sourcesShownIn(workspace, 'workstream')[0] || ''} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, source: event.target.value }))}>
                    {sourcesShownIn(workspace, 'workstream').map((directory) => <option value={directory} key={directory}>{directory}</option>)}
                  </select></label>
                  <label>Item name<input value={newPlanDraft.identifier} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, identifier: event.target.value }))} placeholder="Any item name" /></label>
                  <label>Status<StatusMenu value={newPlanDraft.status} onChange={(status) => setNewPlanDraft((draft) => ({ ...draft, status }))} /></label>
                </>
              ) : (
                <div className="jira-create-layout">
                  <section className="jira-ticket-column">
                    <div className="jira-column-heading">
                      <strong>Jira ticket</strong>
                      <span>Fetch the source issue and review its context.</span>
                    </div>
                    <label>Source<select value={newPlanDraft.source || sourcesShownIn(workspace, 'workstream')[0] || ''} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, source: event.target.value }))}>
                      {sourcesShownIn(workspace, 'workstream').map((directory) => <option value={directory} key={directory}>{directory}</option>)}
                    </select></label>
                    <div className="jira-intake">
                      <label>Jira key<input
                        value={newPlanDraft.jiraKey}
                        onChange={(event) => {
                          setNewPlanDraft((draft) => ({ ...draft, jiraKey: event.target.value.toUpperCase() }));
                          setJiraLookup(null);
                        }}
                        onKeyDown={(event) => {
                          if (event.key !== 'Enter' || jiraLookupLoading || !newPlanDraft.jiraKey.trim()) return;
                          event.preventDefault();
                          void fetchJiraIssue();
                        }}
                        placeholder="PM-025"
                      /></label>
                      <button type="button" className="secondary" disabled={jiraLookupLoading || !newPlanDraft.jiraKey.trim()} onClick={() => void fetchJiraIssue()}>
                        <Search size={15} />
                        {jiraLookupLoading ? 'Fetching...' : 'Fetch Jira'}
                      </button>
                    </div>
                    {jiraLookup && (
                      <section className={jiraLookup.state === 'available' ? 'jira-issue-preview' : 'jira-issue-preview warning'} aria-label="Jira issue preview">
                        {jiraLookup.issue ? (
                          <>
                            <header className="jira-issue-preview-header">
                              <strong>{jiraLookup.issue.key}: {jiraLookup.issue.summary}</strong>
                              {jiraLookup.issue.browserUrl && <a href={jiraLookup.issue.browserUrl} target="_blank" rel="noreferrer">Open Jira</a>}
                            </header>
                            <div className="jira-issue-preview-body">
                              <strong className="jira-issue-preview-label">Description</strong>
                              {jiraLookup.issue.description
                                ? <div className="jira-issue-preview-description"><MarkdownPreview content={jiraLookup.issue.description} /></div>
                                : <p className="jira-issue-preview-description empty">This Jira ticket has no description.</p>}
                            </div>
                            <dl className="jira-issue-preview-meta">
                              <div><dt>Type</dt><dd>{jiraLookup.issue.issueType || 'Issue'}</dd></div>
                              <div><dt>Status</dt><dd>{jiraLookup.issue.status || 'Unknown'}</dd></div>
                              <div><dt>Assignee</dt><dd>{jiraLookup.issue.assignee?.displayName || 'Unassigned'}</dd></div>
                              {jiraLookup.issue.priority && <div><dt>Priority</dt><dd>{jiraLookup.issue.priority}</dd></div>}
                            </dl>
                            {jiraLookup.issue.attachments.length > 0 && <small>{jiraLookup.issue.attachments.length} attachment{jiraLookup.issue.attachments.length === 1 ? '' : 's'} referenced</small>}
                          </>
                        ) : (
                          <>
                            <strong>{jiraLookup.message || jiraLookup.state}</strong>
                            {jiraLookup.recoveryHint && <span>{jiraLookup.recoveryHint}</span>}
                          </>
                        )}
                      </section>
                    )}
                  </section>
                  <section className="jira-details-column">
                    <div className="jira-column-heading">
                      <strong>Work item details</strong>
                      <span>Review the values imported from Jira before creation.</span>
                    </div>
                    <div className="jira-field-row">
                      <label>Item name<input value={newPlanDraft.identifier} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, identifier: event.target.value }))} placeholder="Jira key or local item name" /></label>
                      <label>Status<StatusMenu value={newPlanDraft.status} onChange={(status) => setNewPlanDraft((draft) => ({ ...draft, status }))} /></label>
                    </div>
                    <label>Title<input value={newPlanDraft.title} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, title: event.target.value }))} placeholder="Jira summary" /></label>
                    <label>Owner<input value={newPlanDraft.owner} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, owner: event.target.value }))} placeholder="Assignee" /></label>
                    <label>Tags<input value={newPlanDraft.tags} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, tags: event.target.value }))} placeholder="story, priority-high" /></label>
                  </section>
                </div>
              )}
            </div>
            {newPlanError && <p className="error">{newPlanError}</p>}
            <footer className="modal-actions">
				<button type="button" className="ghost" onClick={closeNewPlan}>Cancel</button>
              <button
                type="button"
                className="primary"
                disabled={creatingPlan || !newPlanDraft.identifier.trim() || (newPlanDraft.origin === 'jira' && jiraLookup?.state !== 'available')}
                onClick={createPlan}
              >
                {creatingPlan ? 'Creating...' : 'Create Item'}
              </button>
            </footer>
          </section>
        </div></ModalLayer>
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
