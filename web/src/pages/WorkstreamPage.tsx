import { Fragment, memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, Dispatch, DragEvent, MouseEvent, MutableRefObject, PointerEvent as ReactPointerEvent, RefObject, SetStateAction } from 'react';
import { BookmarkPlus, Bot, ChevronDown, Columns3, ListOrdered, PanelRightClose, PanelRightOpen, Code2, FileText, Filter, GitBranch, GripVertical, Info, RefreshCw, RotateCw, Search, SlidersHorizontal, Ticket, Trash2, X } from 'lucide-react';
import { FacetMenu, SelectedFilters } from '../components/FacetMenu';
import { FileMenu } from '../components/FileMenu';
import { useModalDialog } from '../components/overlay';
import { useResizableDrawer } from '../components/useResizableDrawer';
import { readStringPreference, writePreference } from '../shared/preferences/store';
import { RecentGitActivity } from '../components/RecentGitActivity';
import { StatusMenu } from '../components/StatusMenu';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { JiraItemPanel } from '../features/jira/JiraItemPanel';
import { ApiError, api, statusLabels, statusOrder } from '../shared/api';
import type {
  WorkstreamBranchLoadResult,
  FileContent,
  FileNode,
  GitStatus,
  GitActivityEntry,
  ItemDetail,
  ItemMetadataUpdateInput,
  ItemStatus,
  ItemSummary,
  JiraIssue,
  JiraIssueState,
  SavedFilter,
  SourceSettingsResult,
  SourceStructureCard,
  SourceStructurePreview,
  SourceStructureProposal,
  SourceStructureSettings,
  WorkspaceConfig
} from '../lib/types';
import { isDocumentationMetadataSource, labels, metadataSourceLabel as genericMetadataSourceLabel } from '../lib/vocabulary';
import { sourcesShownIn } from '../lib/sources';
import { emptyFilters, filterPlans, sourceFacetOptions, sourceLabel } from '../features/workstream/filtering';
import type { FacetOption, FilterKey, Filters } from '../features/workstream/filtering';
import { applyItemStatus, isDropStatus, isItemDraggable } from '../features/workstream/dragAndDrop';
import { BranchCheckoutPicker } from '../features/workstream/BranchCheckoutPicker';
import { MarkdownPreview } from '../features/content-viewer/renderers/MarkdownPreview';
import { lastPathSegment, previewPathSegments } from '../features/workspaces/sourceSettings';
import { canonicalSourceSettingsCard, normalizeSourceSettingsCard, sourceSettingsEditorFromResult, UNSORTED_SOURCE_SELECTION_ID, type SourceSettingsEditorModel } from '../features/workspaces/sourceSettingsEditor';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { AgentCluster, AgentPane, AgentsTab, agentFocusKey, useHeldAgentSessions, type AgentFocus, type AgentPaneHarness, type AgentsMode } from '../features/workstream/AgentsPanel';
import { HarnessRunTab } from '../features/workstream/HarnessRunTab';
import { takeOverByHand } from '../features/workstream/takeOver';
import { plainError, type HarnessController } from '../features/harness/useHarness';
import { useItemSessions, type ItemAgents } from '../features/workstream/useItemSessions';
import { useBoardHarness } from '../features/workstream/useBoardHarness';
import { isSyntheticItem, joinHarness, queueOrder, taskCards, tasksForWorkspace, workedBy, workedByLabels, type CardWork } from '../features/workstream/harnessJoin';
import { PlanListView } from '../features/workstream/PlanListView';
import { ViewSwitcher, type ViewSwitcherOption } from '../components/ViewSwitcher';
import { ByHandChip, HarnessCardChips } from '../features/workstream/HarnessCardChips';
import { NeedsYouStrip } from '../features/workstream/NeedsYouStrip';
import { RunWithHarnessDialog } from '../features/workstream/RunWithHarnessDialog';
import { QueueView } from '../features/workstream/QueueView';
import { useTaskActions } from '../features/workstream/useTaskActions';
import { RunDrawer } from '../features/harness/RunDrawer';
import { canStartTask, startPending } from '../features/harness/harnessState';
import type { HarnessRunTab as HarnessRunTabId } from '../app/router';
import type { HarnessTask } from '../lib/types';
import '../features/harness/harness.css';
import '../features/workstream/harnessCards.css';

type DrawerTab = 'preview' | 'raw' | 'diff';
/** What the board shows: the harness's tasks, whose columns follow their runs, or the plans on the branch, moved by hand. */
type BoardKind = 'tasks' | 'plans';
/** How it shows them: cards in status columns, or rows. */
type BoardLayout = 'board' | 'list';
const boardKinds: ViewSwitcherOption<BoardKind>[] = [{ id: 'tasks', label: 'Tasks', icon: Ticket }, { id: 'plans', label: 'Plans', icon: FileText }];
const boardLayouts: ViewSwitcherOption<BoardLayout>[] = [{ id: 'board', label: 'Board', icon: Columns3 }, { id: 'list', label: 'List', icon: ListOrdered }];
const boardKindPreference = 'workstreamBoardKind';
const layoutPreferences: Record<BoardKind, string> = { tasks: 'workstreamTaskLayout', plans: 'workstreamPlanLayout' };
/** The Tasks board's first column: a task nobody has started. */
const taskColumnLabels: Partial<Record<ItemStatus, string>> = { unsorted: 'To do' };
/** The one column a task may be dragged to: dropping a task that has not started there starts it. */
const taskStartColumn: ItemStatus = 'draft';
function readBoardKind(): BoardKind {
  return readStringPreference(boardKindPreference) === 'plans' ? 'plans' : 'tasks';
}
function readBoardLayout(kind: BoardKind): BoardLayout {
  const value = readStringPreference(layoutPreferences[kind]);
  if (value === 'board' || value === 'list') return value;
  // The Task Queue view the board had before is the Tasks list.
  return kind === 'tasks' && readStringPreference('workstreamBoardView') === 'queue' ? 'list' : 'board';
}
type DrawerSideTab = 'info' | 'git' | 'jira' | 'agents';
type DrawerFileOption = { id: string; path: string; label: string };
const DRAG_CLICK_SUPPRESSION_MS = 350;
const UNSORTED_SELECTION_ID = UNSORTED_SOURCE_SELECTION_ID;

type SourceItemsEditorState = SourceSettingsEditorModel & {
  workspace: WorkspaceConfig;
};

type NewItemOrigin = 'blank' | 'jira';

type NewWorkItemDraft = {
  origin: NewItemOrigin;
  source: string;
  scope: string;
  identifier: string;
  title: string;
  status: ItemStatus;
  owner: string;
  tags: string;
  jiraKey: string;
};

const emptyNewWorkItemDraft = (): NewWorkItemDraft => ({
  origin: 'blank',
  source: '',
  scope: '',
  identifier: '',
  title: '',
  status: 'draft',
  owner: '',
  tags: '',
  jiraKey: ''
});

export { filterPlans };

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
  const [sourceItemsLoading, setSourceItemsLoading] = useState(false);
  const [sourceItemsSaving, setSourceItemsSaving] = useState(false);
  const [sourceItemsDirectory, setSourceItemsDirectory] = useState('');
  const [sourceItemsError, setSourceItemsError] = useState('');
  const [sourceItemsEditor, setSourceItemsEditor] = useState<SourceItemsEditorState | null>(null);
	const sourceItemsDialog = useModalDialog(() => setSourceItemsOpen(false), !sourceItemsSaving, sourceItemsOpen);
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
  const sourceMode = branchContext?.sourceMode ?? 'working_tree';
  const displayedStatuses = useMemo(() => {
    const visible = new Set(visibleStatuses);
    const statuses = statusOrder.filter((status) => visible.has(status));
    return statuses.length > 0 ? statuses : statusOrder;
  }, [visibleStatuses]);
  const boardStyle = useMemo(() => ({
    gridTemplateColumns: displayedStatuses
      .flatMap((status) => status === 'unsorted' && !tasksShown ? ['minmax(200px, 1fr)', '44px'] : ['minmax(200px, 1fr)'])
      .join(' ')
  }) as CSSProperties, [displayedStatuses, tasksShown]);
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
      {sourceItemsOpen && workspace && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !sourceItemsSaving) setSourceItemsOpen(false); }}>
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
        </div>
      )}
      {newPlanOpen && workspace && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !creatingPlan) closeNewPlan(); }}>
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
        </div>
      )}
    </section>
  );
}

function parseTags(value: string): string[] {
  return Array.from(new Set(value.split(',').map((tag) => tag.trim()).filter(Boolean)));
}

function slugTag(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

function jiraTags(issue: JiraIssue): string[] {
  const tags = [...issue.labels];
  const issueType = slugTag(issue.issueType);
  const priority = slugTag(issue.priority ?? '');
  if (issueType) tags.unshift(issueType);
  if (priority) tags.unshift(`priority-${priority}`);
  return Array.from(new Set(tags.filter(Boolean)));
}

function jiraReadme(issue: JiraIssue): string {
  const lines = [
    `# ${issue.key}: ${issue.summary}`,
    '',
    '## Jira Context',
    '',
    `- Key: ${issue.key}`,
    `- Status: ${issue.status || 'Unknown'}`,
    `- Type: ${issue.issueType || 'Unknown'}`,
    `- Priority: ${issue.priority || 'Unspecified'}`,
    `- Assignee: ${issue.assignee?.displayName || 'Unassigned'}`,
    `- Reporter: ${issue.reporter?.displayName || 'Unknown'}`,
    issue.browserUrl ? `- Jira: ${issue.browserUrl}` : '',
    '',
    '## Description',
    '',
    issue.description?.trim() || '_No Jira description available._'
  ].filter((line) => line !== '');
  if (issue.attachments.length > 0) {
    lines.push('', '## Attachments', '');
    for (const attachment of issue.attachments) {
      lines.push(`- ${attachment.filename} (${attachment.mediaType || 'unknown'}, ${attachment.sizeBytes.toLocaleString()} bytes)`);
    }
  }
  return `${lines.join('\n')}\n`;
}

function SourceStructureProposalList({
  proposals,
  selectedProposalId,
  onSelect,
  onClear
}: {
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  onSelect: (proposal: SourceStructureProposal) => void;
  onClear: () => void;
}) {
  if (proposals.length === 0) return null;
  return (
    <section className="source-proposals" aria-label="Source structure proposals">
      <div className="source-structure-section-heading">
        <strong>Suggested structures</strong>
        <span>Choose a structure, or keep the source unsorted.</span>
      </div>
      <div className="source-proposal-grid">
        <button className={selectedProposalId === UNSORTED_SELECTION_ID ? 'source-proposal-card active' : 'source-proposal-card'} type="button" onClick={onClear}>
          <strong>Unsorted</strong>
          <span>Keep this source as one unstructured item in the Unsorted lane.</span>
        </button>
        {proposals.map((proposal) => {
          const selected = selectedProposalId === proposal.id;
          return (
            <button className={selected ? 'source-proposal-card active' : 'source-proposal-card'} type="button" key={proposal.id} onClick={() => onSelect(proposal)}>
              <strong>{proposal.label}</strong>
              <span>{proposal.summary}</span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

function SourceStructurePreviewTable({ preview, onChangeField }: {
  preview: SourceStructurePreview[];
  onChangeField: (path: string, field: 'item' | 'title' | 'status', value: string) => void;
}) {
  const [mode, setMode] = useState<'table' | 'tree'>('table');
  return (
    <section className="source-preview" aria-label="Source structure preview">
      <div className="source-structure-section-heading">
        <strong>Item mapping</strong>
        <div className="source-preview-heading-actions">
          <span>{preview.length === 0 ? 'No matching card directories yet.' : `${preview.length} sample cards`}</span>
          {preview.length > 0 && (
            <button
              type="button"
              className="source-preview-mode-toggle"
              onClick={() => setMode((current) => current === 'table' ? 'tree' : 'table')}
            >
              {mode === 'table' ? 'Tree view' : 'Table view'}
            </button>
          )}
        </div>
      </div>
      {preview.length > 0 && mode === 'table' && (
        <div className="source-preview-table">
          <div className="source-preview-row heading">
            <span>Path</span>
            <span>Source</span>
            <span>Item</span>
            <span>Title</span>
            <span>Status</span>
          </div>
          {preview.map((row) => (
            <div className="source-preview-row" key={row.path}>
              <span title={row.path}>{row.path}</span>
              <span>{row.source ?? row.scope}</span>
              <span><input value={row.item ?? row.identifier ?? ''} onChange={(event) => onChangeField(row.path, 'item', event.target.value)} /></span>
              <span><input value={row.title ?? ''} onChange={(event) => onChangeField(row.path, 'title', event.target.value)} /></span>
              <span>
                <select value={row.status ?? 'draft'} onChange={(event) => onChangeField(row.path, 'status', event.target.value)}>
                  <option value="unsorted">Unsorted</option>
                  <option value="draft">Planning</option>
                  <option value="in_progress">In Progress</option>
                  <option value="review">Review</option>
                  <option value="done">Done</option>
                </select>
              </span>
            </div>
          ))}
        </div>
      )}
      {preview.length > 0 && mode === 'tree' && <SourcePreviewTree preview={preview} />}
    </section>
  );
}

type PreviewTreeNode = {
  name: string;
  path: string;
  row?: SourceStructurePreview;
  children: PreviewTreeNode[];
};

function SourcePreviewTree({ preview }: { preview: SourceStructurePreview[] }) {
  return (
    <div className="source-preview-tree" role="tree" aria-label="Source item tree preview">
      {buildSourcePreviewTree(preview).map((node) => <SourcePreviewTreeNodeView key={node.path} node={node} />)}
    </div>
  );
}

function SourcePreviewTreeNodeView({ node }: { node: PreviewTreeNode }) {
  return (
    <div className="source-preview-tree-node" role="treeitem" aria-label={node.path}>
      <span className="source-preview-tree-label">{node.name}</span>
      {node.row && (
        <small>
          item: {node.row.item ?? node.row.identifier} - title: {node.row.title} - status: {node.row.status}
        </small>
      )}
      {node.children.length > 0 && (
        <div className="source-preview-tree-children" role="group">
          {node.children.map((child) => <SourcePreviewTreeNodeView key={child.path} node={child} />)}
        </div>
      )}
    </div>
  );
}

function buildSourcePreviewTree(preview: SourceStructurePreview[]): PreviewTreeNode[] {
  type MutableTreeNode = PreviewTreeNode & { childMap: Map<string, MutableTreeNode> };
  const roots = new Map<string, MutableTreeNode>();
  for (const row of preview) {
    const segments = row.path.split('/').filter(Boolean);
    let pathSoFar = '';
    let scope = roots;
    let currentNode: MutableTreeNode | null = null;
    for (const segment of segments) {
      pathSoFar = pathSoFar ? `${pathSoFar}/${segment}` : segment;
      if (!scope.has(segment)) {
        scope.set(segment, { name: segment, path: pathSoFar, children: [], childMap: new Map() });
      }
      currentNode = scope.get(segment) ?? null;
      scope = currentNode?.childMap ?? new Map();
    }
    if (currentNode) currentNode.row = row;
  }

  const toImmutable = (nodes: Map<string, MutableTreeNode>): PreviewTreeNode[] => Array.from(nodes.values())
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { numeric: true, sensitivity: 'base' }))
    .map((node) => ({
      name: node.name,
      path: node.path,
      row: node.row,
      children: toImmutable(node.childMap)
    }));

  return toImmutable(roots);
}

function sourceItemsEditorFromResult(workspace: WorkspaceConfig, directory: string, result: SourceSettingsResult): SourceItemsEditorState {
  return { workspace, ...sourceSettingsEditorFromResult(directory, result) };
}

function applySourceItemsProposal(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>,
  proposal: SourceStructureProposal
) {
  setSourceItemsEditor((current) => {
    if (!current) return current;
    return {
      ...current,
      card: normalizeSourceSettingsCard(proposal.card, current.directory),
			cards: [normalizeSourceSettingsCard(proposal.card, current.directory), ...current.cards.slice(1)],
      selectedProposalId: proposal.id,
      preview: proposal.preview
    };
  });
}

function clearSourceItemsProposal(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>
) {
  setSourceItemsEditor((current) => current ? {
    ...current,
    selectedProposalId: UNSORTED_SELECTION_ID,
    preview: current.unsortedPreview
  } : current);
}

function updateSourceItemsPreviewField(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>,
  path: string,
  field: 'item' | 'title' | 'status',
  value: string
) {
  setSourceItemsEditor((current) => {
    if (!current) return current;
    const normalized = value.trim();
    const nextCard = { ...current.card, fields: { ...current.card.fields } };
    let nextPreview: SourceStructurePreview[] = current.preview.map((row) => ({
      ...row,
      item: row.path === path && field === 'item' ? value : row.item,
      identifier: row.path === path && field === 'item' ? value : row.identifier,
      title: row.path === path && field === 'title' ? value : row.title,
      status: row.path === path && field === 'status' ? value as SourceStructurePreview['status'] : row.status
    }));
    if (field === 'item') {
      nextCard.fields.item = normalized;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, true);
      if (suggestedTemplate) {
        nextCard.fields.item = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? normalized : row.item ?? row.identifier);
          return { ...row, item: resolved, identifier: resolved };
        });
      }
    }
    if (field === 'title') {
      nextCard.fields.title = value;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, false);
      if (suggestedTemplate) {
        nextCard.fields.title = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? value : row.title);
          return { ...row, item: row.item, identifier: row.identifier, title: resolved };
        });
      }
    }
    if (field === 'status') {
      nextCard.fields.status = value;
    }

    return {
      ...current,
      selectedProposalId: undefined,
      card: nextCard,
      preview: nextPreview
    };
  });
}

function suggestTemplateFromValue(directory: string, pathPattern: string, rowPath: string, value: string, allowMultiSegment: boolean): string | null {
  if (!value) return null;
  const captures = pathPatternCaptures(directory, pathPattern, rowPath);
  if (!captures) return null;
  const segments = value.split('/').map((segment) => segment.trim()).filter(Boolean);
  if (segments.length === 0) return null;
  if (!allowMultiSegment && segments.length > 1) return null;

  const used = new Set<string>();
  const templateSegments: string[] = [];
  for (const segment of segments) {
    const options = Object.entries(captures)
      .filter(([name, value]) => value === segment && !used.has(name));
    if (options.length !== 1) return null;
    used.add(options[0][0]);
    templateSegments.push(`{${options[0][0]}}`);
  }
  return templateSegments.join('/');
}

function pathPatternCaptures(directory: string, pathPattern: string, rowPath: string): Record<string, string> | null {
  const patternSegments = pathPattern.split('/').map((segment) => segment.trim()).filter(Boolean);
  const rowSegments = previewPathSegments(rowPath, directory);
  if (patternSegments.length === 0 || patternSegments.length !== rowSegments.length) return null;

  const captures: Record<string, string> = {};
  for (let index = 0; index < patternSegments.length; index += 1) {
    const patternSegment = patternSegments[index];
    const rowSegment = rowSegments[index];
    const variable = patternSegment.match(/^\{([A-Za-z][A-Za-z0-9_]*)\}$/)?.[1];
    if (variable) {
      captures[variable] = rowSegment;
      continue;
    }
    if (patternSegment !== rowSegment) return null;
  }
  return captures;
}

function renderTemplateWithCaptures(template: string, captures: Record<string, string>): string {
  return Object.entries(captures).reduce((result, [name, value]) => result.replaceAll(`{${name}}`, value), template).trim();
}

function WorkstreamColumn({ status, label, itemCount, loading, dragActive, dragTargetStatus, onDragOver, onDragLeave, onDrop, onCreate, children }: {
  status: ItemStatus;
  /** The column's name where it is not the status's own. */
  label?: string;
  itemCount: number;
  loading: boolean;
  dragActive: boolean;
  dragTargetStatus: ItemStatus | '';
  onDragOver: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onDragLeave: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onDrop: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onCreate?: () => void;
  children: React.ReactNode;
}) {
  const droppable = dragActive && isDropStatus(status);
  const classes = ['workstream-column', status];
  if (dragActive && droppable) classes.push('drop-enabled');
  if (dragTargetStatus === status && droppable) classes.push('drop-target');

  return (
    <div
      className={classes.join(' ')}
      data-status={status}
      onDragOver={(event) => onDragOver(event, status)}
      onDragLeave={(event) => onDragLeave(event, status)}
      onDrop={(event) => onDrop(event, status)}
    >
      <header>
        <h2>{label ?? statusLabels[status]}</h2>
        <span>{itemCount}</span>
        <Filter size={14} />
      </header>
      <div className="card-stack">
        {loading && Array.from({ length: 3 }).map((_, index) => <div className="plan-card skeleton" key={index} />)}
        {children}
        {!loading && itemCount === 0 && <div className="column-empty">No items</div>}
      </div>
      {onCreate && <button className="new-plan-column-button" type="button" onClick={onCreate}>+ New item</button>}
    </div>
  );
}

const PlanCard = memo(function PlanCard({ item: plan, workspace, pending, active, created, agents, work, harnessOn = false, autoStatus = false, startable = false, onRunWithHarness, onOpenTask, onDragStart, onDragEnd, onPreview, onOpen, onMove, onOpenAgent }: {
  item: ItemSummary;
  workspace?: WorkspaceConfig;
  pending: boolean;
  active: boolean;
  created: boolean;
  agents?: ItemAgents;
  /** What the harness holds for this card, when it works on it. */
  work?: CardWork;
  /** True once the board knows the harness is on, so a card the harness does not hold can say "By hand". */
  harnessOn?: boolean;
  /** True on the Tasks board, where a card's column follows its run and a person never moves it. */
  autoStatus?: boolean;
  /** A task not yet started: it may be dragged to the next column, which starts it. */
  startable?: boolean;
  /** Hands the card to the harness; offered on a card it does not hold. */
  onRunWithHarness?: () => void;
  /** Opens the harness's task behind a ticket card that no plan holds yet. */
  onOpenTask?: () => void;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
  onPreview: () => void;
  onOpen: () => void;
  onMove: (status: ItemStatus) => void;
  onOpenAgent: (focus: AgentFocus) => void;
}) {
  const source = sourceLabel(plan, workspace);
  const docs = isDocumentationMetadataSource(plan.metadataSource);
  const showItem = plan.identifier.toLowerCase() !== plan.title.toLowerCase();
  const description = plan.description;
  // A tag equal to the source badge is redundant on any card, not just a docs one:
  // the default source-structure card tags every item with its own source name, so
  // wiki items arrived as metadataSource "workspace-settings" and printed it twice.
  const tags = (docs ? plan.tags.filter((tag) => tag !== plan.scope && tag !== plan.identifier) : plan.tags).filter((tag) => tag !== source);
  const synthetic = isSyntheticItem(plan);
  // The card's key opens it where the card shows one; else its title does.
  const keyOpens = !docs && (synthetic || showItem);
  const draggable = startable || (isItemDraggable(plan) && !pending && !synthetic && !autoStatus);
  const navigate = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (synthetic) onOpenTask?.(); else onOpen();
  };
  const preview = () => { if (synthetic) onOpenTask?.(); else onPreview(); };
  const classes = ['plan-card'];
  if (docs) classes.push('docs-plan');
  if (work) classes.push('harness-card');
  if (work?.task.needsYou || work?.waiting.length) classes.push('needs-you-card');
  if (work?.status === 'working' || work?.status === 'starting') classes.push('working-card');
  if (synthetic) classes.push('ticket-card');
  else if (!docs) classes.push('plan-kind-card');
  if (synthetic && !work?.run) classes.push('no-plan');
  if (draggable) classes.push('draggable');
  if (active) classes.push('dragging');
  if (pending) classes.push('move-pending');
  if (created) classes.push('just-created');
  return (
    <article
      data-item-id={plan.id}
      className={classes.join(' ')}
      draggable={draggable}
      aria-busy={pending}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={preview}
      role="button"
      tabIndex={0}
      onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        preview();
      }
      }}
    >
      {!docs && (
        <div className="card-kind-row">
          <span className={synthetic ? 'card-kind ticket' : 'card-kind plan'} title={synthetic ? 'A Jira ticket the harness holds. No plan on this branch yet.' : undefined}>
            {synthetic ? <Ticket size={13} aria-hidden="true" /> : <FileText size={13} aria-hidden="true" />}
            <span>{synthetic ? 'Jira ticket' : 'Plan'}</span>
            {keyOpens && <><span aria-hidden="true">·</span><button type="button" className="card-kind-key" title={`Open ${plan.identifier}`} onPointerDown={(event) => event.stopPropagation()} onClick={navigate}>{plan.identifier}</button></>}
          </span>
          {!synthetic && <AgentCluster agents={agents} title={plan.title} onOpen={onOpenAgent} />}
        </div>
      )}
      <div className="plan-card-title">
        {keyOpens
          ? <strong className="plan-card-heading">{plan.title}</strong>
          : <button type="button" className="plan-card-link plan-card-heading" onPointerDown={(event) => event.stopPropagation()} onClick={navigate}>{plan.title}</button>}
        {docs && source && <span className="card-badges"><span className="source-badge docs">{source}</span></span>}
      </div>
      {docs && showItem && <span className="plan-card-identifier">{plan.identifier}</span>}
      {work ? <HarnessCardChips work={work} /> : harnessOn && !docs && plan.status !== 'unsorted' && (
        <div className="plan-card-harness plan-card-harness-chips">
          <ByHandChip />
          {onRunWithHarness && <button type="button" className="secondary plan-card-run" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); onRunWithHarness(); }}>Run with the harness</button>}
        </div>
      )}
      {description && <p>{description}</p>}
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {tags.length > 0 && <div className="tags">{tags.slice(0, 3).map((tag: string) => <span key={tag}>{tag}</span>)}</div>}
      {plan.status !== 'unsorted' && !synthetic && !autoStatus && !isDocumentationMetadataSource(plan.metadataSource) && (
        <StatusMenu value={plan.status} onChange={onMove} ariaLabel="Move item status" />
      )}
    </article>
  );
});

function PlanPreviewDrawer({ itemId, refreshKey, workspaceId, agents, agentFocus, onAgentFocus, onAgentsChanged, work, harness, onRunWithHarness, onClose, onOpenFull, onOpenRun, onChanged }: { itemId: string; refreshKey: number; workspaceId?: string; agents?: ItemAgents; agentFocus?: AgentFocus; onAgentFocus: (focus: AgentFocus | undefined) => void; onAgentsChanged: () => Promise<unknown> | void; work?: CardWork; harness: HarnessController; /** Offered while the harness is on and does not hold this card. */ onRunWithHarness?: () => void; onClose: () => void; onOpenFull: () => void; onOpenRun: () => void; onChanged: () => void | Promise<void> }) {
  const [agentsMode, setAgentsMode] = useState<AgentsMode>('run');
  const harnessRun = work?.run ? <HarnessRunTab work={work} harness={harness} onOpenConversation={(id) => onAgentFocus({ kind: 'harness', id })} onOpenPlan={(id) => onAgentFocus({ kind: 'harness-plan', id })} onOpenFull={onOpenRun} onTakeOver={() => void takeOverByHand(work, harness).then(({ agentRunId }) => { if (agentRunId) { onAgentFocus({ kind: 'harness', id: agentRunId }); } else { setAgentsMode('sessions'); onAgentFocus({ kind: 'launch' }); } })} /> : undefined;
  const paneHarness: AgentPaneHarness = { client: harness.client, run: work?.run, detail: work?.run ? harness.state.details[work.run.id] : undefined, reload: () => (work?.run ? harness.loadRun(work.run.id) : Promise.resolve()) };
  const [plan, setPlan] = useState<ItemDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [editorContent, setEditorContent] = useState('');
  const [savedContent, setSavedContent] = useState('');
  const [savingFile, setSavingFile] = useState(false);
  const [autoSaveState, setAutoSaveState] = useState<'idle' | 'pending' | 'saving' | 'saved' | 'error'>('idle');
  const [metadataDraft, setMetadataDraft] = useState<ItemMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<DrawerTab>('preview');
  const [sideTab, setSideTab] = useState<DrawerSideTab>(() => agentFocus ? 'agents' : 'info');
  // Collapsed, the Work Item panel shrinks to a rail of tab icons and the main
  // pane takes its width; the choice is remembered across drawers.
  const [workItemCollapsed, setWorkItemCollapsed] = useState(() => readStoredToggle('workstream.drawer.workItemCollapsed'));
  const collapseWorkItem = (collapsed: boolean) => {
    setWorkItemCollapsed(collapsed);
    writePreference('workstream.drawer.workItemCollapsed', collapsed ? '1' : '0');
  };
  const openSideTab = (tab: DrawerSideTab) => {
    setSideTab(tab);
    if (workItemCollapsed) collapseWorkItem(false);
  };
  // Live sessions opened in this drawer stay connected until it closes or
  // moves to another card, then go to the terminal dock.
  const { heldLiveSessionIds, release: releaseHeldSession } = useHeldAgentSessions(agents, agentFocus, itemId);
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
  const [gitActivity, setGitActivity] = useState<GitActivityEntry[]>([]);
  const [gitActivityLoading, setGitActivityLoading] = useState(false);
  const [gitActivityRefreshKey, setGitActivityRefreshKey] = useState(0);
  const [gitMessage, setGitMessage] = useState('');
  const [selectedGitPaths, setSelectedGitPaths] = useState<string[]>([]);
  const [branchName, setBranchName] = useState('');
  const [gitBusy, setGitBusy] = useState('');
  const [gitActivityOpen, setGitActivityOpen] = useState(() => readStoredToggle('workstream.drawer.gitActivityOpen'));
  const [error, setError] = useState('');
  const { width, style: drawerStyle, startResize } = useResizableDrawer({ initial: 1120 });
  const autoSaveTimerRef = useRef<number | null>(null);
  // The Raw editor grows with the document so the whole file shows, as the
  // preview does, and the drawer scrolls instead of a small box.
  const rawEditorRef = useRef<HTMLTextAreaElement | null>(null);
  const autoSaveSettledTimerRef = useRef<number | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const drawerGenerationRef = useRef(0);
  const fileLoadAbortRef = useRef<AbortController | null>(null);
  const autoSaveRefreshAbortRef = useRef<AbortController | null>(null);
  const compact = width < 700;
  const dirtyFile = file !== null && editorContent !== savedContent;
  const activityPath = plan?.itemPath || '';
  const fileOptions = useMemo(() => flattenFileOptions(files), [files]);
  const dirtyMetadata = Boolean(plan) && (
    (metadataDraft.title ?? '') !== (plan?.title ?? '') ||
    (metadataDraft.scope ?? '') !== (plan?.scope ?? '') ||
    (metadataDraft.identifier ?? '') !== (plan?.identifier ?? '') ||
    (metadataDraft.status ?? '') !== (plan?.status ?? '') ||
    (metadataDraft.owner ?? '') !== (plan?.owner ?? '') ||
    (metadataDraft.tags ?? []).join('\n') !== (plan?.tags ?? []).join('\n')
  );

  const clearDrawerAutoSaveTimers = () => {
    clearTimeoutRef(autoSaveTimerRef);
    clearTimeoutRef(autoSaveSettledTimerRef);
    clearTimeoutRef(autoSaveRefreshTimerRef);
  };

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      // Escape typed into an agent's terminal belongs to the agent (it
      // interrupts Claude, for one); it must not close the drawer.
      if (event.target instanceof Element && event.target.closest('.agent-pane-body')) return;
      closeDrawer();
    };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [agentFocus, agents, dirtyFile, editorContent, file, onClose, savedContent]);

  useLayoutEffect(() => {
    const editor = rawEditorRef.current;
    if (tab !== 'raw' || !editor) return;
    editor.style.height = 'auto';
    editor.style.height = `${editor.scrollHeight + 2}px`;
  }, [tab, editorContent, width]);

  useEffect(() => {
    if (agentFocus) setSideTab('agents');
  }, [agentFocusKey(agentFocus)]);

  useEffect(() => {
    const controller = new AbortController();
    const generation = ++drawerGenerationRef.current;
    fileLoadAbortRef.current?.abort();
    autoSaveRefreshAbortRef.current?.abort();
    setPlan(null);
    setFile(null);
    setFiles([]);
    setDiff('');
    setGitStatus(null);
    setError('');
    api.item(itemId, controller.signal).then((payload) => {
      if (controller.signal.aborted) return;
      setPlan(payload);
      api.gitStatus(payload.workspaceId, controller.signal).then((status) => {
        if (!controller.signal.aborted) setGitStatus(status);
      }).catch(() => {
        if (!controller.signal.aborted) setGitStatus(null);
      });
    }).catch((err: Error) => {
      if (!controller.signal.aborted) setError(err.message);
    });
    api.files(itemId, undefined, controller.signal).then(async (tree) => {
      if (controller.signal.aborted) return;
      setFiles(tree);
      const previewFile = preferredPreviewFile(tree);
      if (previewFile) {
        try {
          const content = await api.file(itemId, previewFile.id, undefined, controller.signal);
          if (!controller.signal.aborted) {
            setFile(content);
            setEditorContent(content.content);
            setSavedContent(content.content);
            setAutoSaveState('idle');
          }
        } catch (err) {
          if (!controller.signal.aborted) setError(err instanceof Error ? err.message : 'File failed to load');
        }
      }
    }).catch((err: Error) => {
      if (!controller.signal.aborted) setError(err.message);
    });
    api.diff(itemId, controller.signal).then((payload) => {
      if (!controller.signal.aborted) setDiff(payload.diff || 'No local changes.');
    }).catch(() => {
      if (!controller.signal.aborted) setDiff('No diff available.');
    });
    return () => {
      controller.abort();
      if (drawerGenerationRef.current === generation) drawerGenerationRef.current += 1;
      fileLoadAbortRef.current?.abort();
      autoSaveRefreshAbortRef.current?.abort();
      clearDrawerAutoSaveTimers();
    };
  }, [itemId]);

  // A refreshKey bump is the app refreshing (on window focus, among others),
  // not a new item: refresh the item, its files, diff and git status in place,
  // and leave the open file and any unsaved edit alone.
  const refreshKeyLoadedRef = useRef(refreshKey);
  useEffect(() => {
    if (refreshKeyLoadedRef.current === refreshKey) return;
    refreshKeyLoadedRef.current = refreshKey;
    const controller = new AbortController();
    const generation = drawerGenerationRef.current;
    const isCurrent = () => !controller.signal.aborted && drawerGenerationRef.current === generation;
    api.item(itemId, controller.signal).then((payload) => {
      if (!isCurrent()) return;
      // An unchanged item keeps its identity, so the Info tab's draft survives.
      setPlan((current) => sameJSON(current, payload) ? current : payload);
      void loadGitStatus(payload.workspaceId, generation);
    }).catch(() => undefined);
    api.files(itemId, undefined, controller.signal).then((tree) => { if (isCurrent()) setFiles((current) => sameJSON(current, tree) ? current : tree); }).catch(() => undefined);
    api.diff(itemId, controller.signal).then((payload) => { if (isCurrent()) setDiff(payload.diff || 'No local changes.'); }).catch(() => undefined);
    return () => controller.abort();
  }, [refreshKey]);

  useEffect(() => {
    if (!plan) return;
    setMetadataDraft({
      title: plan.title,
      scope: plan.scope,
      identifier: plan.identifier,
      status: plan.status,
      owner: plan.owner ?? '',
      tags: plan.tags
    });
  }, [plan]);

  useEffect(() => {
    if (!file) {
      setAutoSaveState('idle');
      return;
    }
    if (editorContent === savedContent) {
      if (autoSaveState === 'pending') setAutoSaveState('idle');
      return;
    }
    if (savingFile) {
      setAutoSaveState('pending');
      return;
    }
    clearTimeoutRef(autoSaveTimerRef);
    setAutoSaveState('pending');
    autoSaveTimerRef.current = window.setTimeout(() => {
      void saveDrawerFile(file, editorContent);
    }, 900);
    return () => clearTimeoutRef(autoSaveTimerRef);
  }, [file, editorContent, savedContent, savingFile]);

  const saveDrawerFileNow = async () => {
    if (!file || editorContent === savedContent) return true;
    return saveDrawerFile(file, editorContent);
  };

  const saveDrawerFile = async (targetFile: FileContent, content: string) => {
    const generation = drawerGenerationRef.current;
    const activeItemId = itemId;
    const workspaceId = plan?.workspaceId;
    const isCurrent = () => drawerGenerationRef.current === generation;
    clearTimeoutRef(autoSaveTimerRef);
    clearTimeoutRef(autoSaveSettledTimerRef);
    setSavingFile(true);
    setAutoSaveState('saving');
    setError('');
    try {
      const updated = await api.saveFile(activeItemId, targetFile.id, { content, expectedHash: targetFile.hash });
      notifyReliabilityChanged();
      if (!isCurrent()) return false;
      setFile(updated);
      setSavedContent(content);
      setAutoSaveState('saved');
      autoSaveSettledTimerRef.current = window.setTimeout(() => setAutoSaveState('idle'), 1400);
      clearTimeoutRef(autoSaveRefreshTimerRef);
      autoSaveRefreshTimerRef.current = window.setTimeout(() => {
        if (!isCurrent()) return;
        autoSaveRefreshAbortRef.current?.abort();
        const controller = new AbortController();
        autoSaveRefreshAbortRef.current = controller;
        api.diff(activeItemId, controller.signal)
          .then((payload) => {
            if (!controller.signal.aborted && isCurrent()) setDiff(payload.diff || 'No local changes.');
          })
          .catch(() => {
            if (!controller.signal.aborted && isCurrent()) setDiff('No diff available.');
          });
        if (workspaceId) {
          api.gitStatus(workspaceId, controller.signal)
            .then((status) => {
              if (!controller.signal.aborted && isCurrent()) setGitStatus(status);
            })
            .catch(() => {
              if (!controller.signal.aborted && isCurrent()) setGitStatus(null);
            });
        }
      }, 600);
      return true;
    } catch (err) {
      if (!isCurrent()) return false;
      setError(operationErrorMessage(err, 'File save failed'));
      setAutoSaveState('error');
      return false;
    } finally {
      if (isCurrent()) setSavingFile(false);
    }
  };

  const loadDrawerFile = async (fileId: string) => {
    fileLoadAbortRef.current?.abort();
    const controller = new AbortController();
    fileLoadAbortRef.current = controller;
    const generation = drawerGenerationRef.current;
    try {
      const content = await api.file(itemId, fileId, undefined, controller.signal);
      if (controller.signal.aborted || drawerGenerationRef.current !== generation) return;
      setFile(content);
      setEditorContent(content.content);
      setSavedContent(content.content);
      setAutoSaveState('idle');
    } catch (err) {
      if (!controller.signal.aborted && drawerGenerationRef.current === generation) setError(err instanceof Error ? err.message : 'File failed to load');
    } finally {
      if (fileLoadAbortRef.current === controller) fileLoadAbortRef.current = null;
    }
  };

  const selectDrawerFile = async (fileId: string) => {
    if (!fileId || fileId === file?.id) return;
    if (dirtyFile && !(await saveDrawerFileNow())) return;
    await loadDrawerFile(fileId);
  };

  const closeDrawer = () => {
    if (!dirtyFile) {
      onClose();
      return;
    }
    void saveDrawerFileNow().then((saved) => {
      if (saved) onClose();
    });
  };

  const openFullDetails = () => {
    if (!dirtyFile) {
      onOpenFull();
      return;
    }
    void saveDrawerFileNow().then((saved) => {
      if (saved) onOpenFull();
    });
  };

  const loadGitStatus = async (workspaceId: string, generation = drawerGenerationRef.current) => {
    try {
      const status = await api.gitStatus(workspaceId);
      if (drawerGenerationRef.current === generation) setGitStatus(status);
    } catch {
      if (drawerGenerationRef.current === generation) setGitStatus(null);
    }
  };

  useEffect(() => {
    if (!plan) {
      setGitActivity([]);
      setGitActivityLoading(false);
      return;
    }
    const controller = new AbortController();
    setGitActivityLoading(true);
    api.gitActivity(plan.workspaceId, { path: activityPath || undefined, limit: 8, signal: controller.signal })
      .then((activity) => {
        if (!controller.signal.aborted) setGitActivity(activity);
      })
      .catch(() => {
        if (!controller.signal.aborted) setGitActivity([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setGitActivityLoading(false);
      });
    return () => controller.abort();
  }, [plan?.workspaceId, activityPath, gitActivityRefreshKey]);

  const saveMetadata = async () => {
    if (!plan) return;
    const generation = drawerGenerationRef.current;
    const isCurrent = () => drawerGenerationRef.current === generation;
    setSavingMetadata(true);
    setError('');
    try {
		const result = await api.saveMetadata(itemId, { ...metadataDraft, expectedRevision: plan.metadataRevision });
      notifyReliabilityChanged();
      if (!isCurrent()) return;
      setPlan(result.item);
      await loadGitStatus(plan.workspaceId, generation);
      if (!isCurrent()) return;
      await onChanged();
    } catch (err) {
      if (isCurrent()) setError(operationErrorMessage(err, 'Metadata save failed'));
    } finally {
      if (isCurrent()) setSavingMetadata(false);
    }
  };

  const runGitOperation = async (operation: 'fetch' | 'pull' | 'push') => {
    if (!plan) return;
    const generation = drawerGenerationRef.current;
    const isCurrent = () => drawerGenerationRef.current === generation;
    setGitBusy(operation);
    setError('');
    try {
      const result = operation === 'fetch'
        ? await api.gitFetch(plan.workspaceId)
        : operation === 'pull'
			? await api.gitPull(plan.workspaceId, {})
          : await api.gitPush(plan.workspaceId);
      notifyReliabilityChanged();
      if (!isCurrent()) return;
      setGitStatus(result.status);
      if (operation === 'pull') {
        await onChanged();
      }
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      if (isCurrent()) setError(operationErrorMessage(err, `${operation} failed`));
    } finally {
      if (isCurrent()) setGitBusy('');
    }
  };

  const toggleGitPath = (path: string) => {
    setSelectedGitPaths((current) => current.includes(path) ? current.filter((item) => item !== path) : [...current, path]);
  };

  const commitSelectedPaths = async () => {
    if (!plan) return;
    const generation = drawerGenerationRef.current;
    const isCurrent = () => drawerGenerationRef.current === generation;
    setGitBusy('commit');
    setError('');
    try {
      const result = await api.gitCommit(plan.workspaceId, { message: gitMessage, paths: selectedGitPaths });
      notifyReliabilityChanged();
      if (!isCurrent()) return;
      setGitStatus(result.status);
      setGitActivityRefreshKey((current) => current + 1);
      setGitMessage('');
      setSelectedGitPaths([]);
      await onChanged();
      if (!isCurrent()) return;
      api.diff(itemId).then((payload) => {
        if (isCurrent()) setDiff(payload.diff || 'No local changes.');
      }).catch(() => {
        if (isCurrent()) setDiff('No diff available.');
      });
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      if (isCurrent()) setError(operationErrorMessage(err, 'Commit failed'));
    } finally {
      if (isCurrent()) setGitBusy('');
    }
  };

  const createAndSwitchBranch = async () => {
    if (!plan || !branchName.trim()) return;
    const generation = drawerGenerationRef.current;
    const isCurrent = () => drawerGenerationRef.current === generation;
    setGitBusy('branch');
    setError('');
    try {
      const result = await api.createBranch(plan.workspaceId, { name: branchName.trim(), checkout: true });
      notifyReliabilityChanged();
      if (!isCurrent()) return;
      setGitStatus(result.status);
      setBranchName('');
      await onChanged();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      if (isCurrent()) setError(operationErrorMessage(err, 'Branch operation failed'));
    } finally {
      if (isCurrent()) setGitBusy('');
    }
  };

  return (
    <>
      <button className="drawer-scrim" style={drawerStyle} type="button" aria-label="Close preview" onClick={closeDrawer} />
      <aside className={compact ? 'plan-drawer compact' : 'plan-drawer'} style={drawerStyle} aria-label="Item preview">
        <button className="drawer-resize-handle" type="button" aria-label="Resize preview panel" onPointerDown={startResize}>
          <GripVertical size={16} />
        </button>
        <header className="plan-drawer-header">
          <div>
            <span className="drawer-kicker">{plan?.identifier ?? 'Loading'}</span>
            <h2>{plan?.title ?? 'Loading item'}</h2>
            <p>{plan ? `${plan.scope} / ${plan.branch}` : ''}</p>
          </div>
          <div className="drawer-actions">
            {onRunWithHarness && !work && <button type="button" className="secondary" onClick={onRunWithHarness}><Bot size={14} /> Run with the harness</button>}
            <button type="button" className="secondary drawer-open-details" onClick={openFullDetails}>Open details</button>
            <button type="button" className="icon-button" aria-label="Close preview" onClick={closeDrawer}><X size={16} /></button>
          </div>
        </header>
        {error && <p className="error drawer-error">{error}</p>}
        <div className={['plan-drawer-body', agentFocus || sideTab === 'agents' ? 'agents-open' : '', workItemCollapsed ? 'work-item-collapsed' : ''].filter(Boolean).join(' ')}>
          {(agentFocus || heldLiveSessionIds.length > 0) && (
          <section className="drawer-main drawer-agent-main" hidden={!agentFocus}>
            <AgentPane focus={agentFocus} heldSessionIds={heldLiveSessionIds} itemId={itemId} workspaceId={workspaceId ?? plan?.workspaceId} agents={agents} harness={paneHarness} onBack={() => onAgentFocus(undefined)} onFocus={onAgentFocus} onRelease={releaseHeldSession} onReload={onAgentsChanged} />
          </section>
          )}
          {agentFocus ? null : (
          <section className="drawer-main">
            <section className="drawer-section">
              <label className="drawer-file-picker">
                <span>Document</span>
                <FileMenu value={file?.id ?? ''} options={fileOptions} tree={files} onChange={selectDrawerFile} />
              </label>
              <div className="drawer-tabs">
                <div className="segmented-control segmented-control-compact" role="tablist" aria-label="Document view">
                  <button className={tab === 'preview' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'preview'} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
                  <button className={tab === 'raw' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'raw'} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
                  <button className={tab === 'diff' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'diff'} onClick={() => setTab('diff')}><RefreshCw size={15} /> Diff</button>
                </div>
                <span className={`autosave-state ${autoSaveState}`}>{autoSaveLabel(autoSaveState)}</span>
              </div>
              {dirtyFile && <div className="edit-state-banner">{autoSaveLabel(autoSaveState)}</div>}
              {tab === 'preview' && (file ? <ContentViewer file={file} content={editorContent} compact modeSwitch={false} /> : <div className="drawer-empty">No readable file selected.</div>)}
              {tab === 'raw' && (
                <textarea
                  ref={rawEditorRef}
                  className="drawer-raw drawer-raw-editor"
                  value={file ? editorContent : 'No readable file selected.'}
                  onChange={(event) => setEditorContent(event.target.value)}
                  disabled={!file || !file.editable}
                  spellCheck={false}
                />
              )}
              {tab === 'diff' && <pre className="drawer-raw">{diff || 'Loading diff...'}</pre>}
              <div className="drawer-file-note">{file?.path ?? 'No file selected'}</div>
            </section>
          </section>
          )}
          {workItemCollapsed ? (
          <aside className="drawer-work-item-rail" aria-label="Work item">
            <button type="button" className="icon-button" aria-label="Expand work item panel" title="Expand work item panel" onClick={() => collapseWorkItem(false)}><PanelRightOpen size={16} /></button>
            <span className="drawer-work-item-rail-divider" aria-hidden="true" />
            {([['info', Info, 'Info'], ['git', GitBranch, 'Git'], ['jira', Ticket, 'Jira'], ['agents', Bot, 'Agents']] as const).map(([tab, Icon, label]) => (
              <button key={tab} type="button" className={sideTab === tab ? 'icon-button active' : 'icon-button'} aria-label={`Open ${label}`} title={label} onClick={() => openSideTab(tab)}>
                <Icon size={16} />{tab === 'agents' && (agents?.live.length ?? 0) > 0 && <span className="side-tab-count" aria-hidden="true">{agents?.live.length}</span>}
              </button>
            ))}
          </aside>
          ) : (
          <aside className="drawer-meta drawer-work-item">
            <h3><Info size={15} /> Work Item<button type="button" className="icon-button drawer-work-item-collapse" aria-label="Collapse work item panel" title="Collapse work item panel" onClick={() => collapseWorkItem(true)}><PanelRightClose size={15} /></button></h3>
            <div className="side-panel-tabs" role="tablist" aria-label="Work item side panel">
              <button type="button" className={sideTab === 'info' ? 'active' : ''} onClick={() => setSideTab('info')}>
                <Info size={14} /> Info
              </button>
              <button type="button" className={sideTab === 'git' ? 'active' : ''} onClick={() => setSideTab('git')}>
                <GitBranch size={14} /> Git
              </button>
              <button type="button" className={sideTab === 'jira' ? 'active' : ''} onClick={() => setSideTab('jira')}>
                <Ticket size={14} /> Jira
              </button>
              <button type="button" className={sideTab === 'agents' ? 'active' : ''} onClick={() => setSideTab('agents')}>
                <Bot size={14} /> Agents{(agents?.live.length ?? 0) > 0 && <span className="side-tab-count" aria-label={`${agents?.live.length} live`}>{agents?.live.length}</span>}
              </button>
            </div>
            <div className="drawer-work-item-content">
              {sideTab === 'info' && (
                <>
                  <dl>
                    <dt>{labels.workspace}</dt><dd>{plan?.workspaceName ?? '-'}</dd>
                    <dt>{labels.source}</dt><dd>{plan ? plan.itemPath?.split('/').filter(Boolean)[0] || plan.scope || '-' : '-'}</dd>
                    <dt>{labels.identifier}</dt><dd>{plan?.identifier ?? '-'}</dd>
                    <dt>Branch</dt><dd>{plan?.branch ?? '-'}</dd>
                    <dt>Status</dt><dd>{plan?.status ? <DrawerStatusBadge status={plan.status} /> : '-'}</dd>
                    <dt>Metadata</dt><dd>{metadataSourceLabel(plan?.metadataSource)}</dd>
                    <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                    <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
                  </dl>
                  {!isDocumentationMetadataSource(plan?.metadataSource) && (
                    <div className="metadata-form drawer-metadata-form">
                      <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
                      <label>{labels.identifier}<input value={metadataDraft.identifier ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, identifier: event.target.value }))} /></label>
                      <label>Status<StatusMenu value={metadataDraft.status ?? 'draft'} onChange={(status) => setMetadataDraft((draft) => ({ ...draft, status }))} /></label>
                      <label>Owner<input value={metadataDraft.owner ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, owner: event.target.value }))} /></label>
                      <label>Tags<input value={(metadataDraft.tags ?? []).join(', ')} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label>
                    </div>
                  )}
                  <div className="workspace-actions">
                    <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || isDocumentationMetadataSource(plan?.metadataSource)} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
                  </div>
                  {(plan?.tags?.length ?? 0) > 0 && <div className="tags">{plan?.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
                </>
              )}
              {sideTab === 'git' && (
                gitStatus ? (
                  <section className="drawer-git-panel">
                  <div className="git-summary">
                    <span>{gitStatus.branch}</span>
                    <span>{gitStatus.ahead} ahead</span>
                    <span>{gitStatus.behind} behind</span>
                  </div>
                  <div className="workspace-actions">
                    <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('fetch')}>{gitBusy === 'fetch' ? 'Fetching...' : 'Fetch'}</button>
                    <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('pull')}>{gitBusy === 'pull' ? 'Pulling...' : 'Pull'}</button>
                    <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('push')}>{gitBusy === 'push' ? 'Pushing...' : 'Push'}</button>
                  </div>
                  <div className="git-changes">
                    {gitStatus.changes.length === 0 && <span>No local changes</span>}
                    {gitStatus.changes.map((change) => (
                      <label key={`${change.status}-${change.path}`}>
                        <input type="checkbox" checked={selectedGitPaths.includes(change.path)} onChange={() => toggleGitPath(change.path)} />
                        <span>{change.status}</span>
                        <strong>{change.path}</strong>
                      </label>
                    ))}
                  </div>
                  <textarea className="commit-message" value={gitMessage} onChange={(event) => setGitMessage(event.target.value)} placeholder="Commit message" />
                  <button className="primary" type="button" disabled={Boolean(gitBusy) || selectedGitPaths.length === 0 || !gitMessage.trim()} onClick={commitSelectedPaths}>
                    {gitBusy === 'commit' ? 'Committing...' : 'Commit Selected'}
                  </button>
                  <div className="branch-create-row">
                    <input value={branchName} onChange={(event) => setBranchName(event.target.value)} placeholder="new-branch-name" />
                    <button className="secondary" type="button" disabled={Boolean(gitBusy) || !branchName.trim()} onClick={createAndSwitchBranch}>
                      {gitBusy === 'branch' ? 'Creating...' : 'Create Branch'}
                    </button>
                  </div>
                  <details className="recent-activity-panel" open={gitActivityOpen} onToggle={(event) => {
                    const open = event.currentTarget.open;
                    setGitActivityOpen(open);
                    writePreference('workstream.drawer.gitActivityOpen', open ? '1' : '0');
                  }}>
                    <summary>
                      <span>Recent Activity</span>
                      <small>{gitActivity.length} events</small>
                    </summary>
                    <RecentGitActivity entries={gitActivity} loading={gitActivityLoading} emptyLabel="No activity found for this item." pathLabel={activityPath || 'workspace'} />
                  </details>
                </section>
                ) : (
                  <div className="metadata-callout">
                    <strong>Git status unavailable</strong>
                    <span>Open details to run Git actions.</span>
                  </div>
                )
              )}
              {sideTab === 'jira' && <JiraItemPanel itemId={itemId} />}
              {sideTab === 'agents' && <AgentsTab itemId={itemId} agents={agents} focus={agentFocus} onFocus={onAgentFocus} onError={setError} harnessRun={harnessRun} mode={agentsMode} onModeChange={setAgentsMode} onLaunched={(result) => { void Promise.resolve(onAgentsChanged()).then(() => onAgentFocus({ kind: 'session', id: result.session.id })); }} />}
            </div>
          </aside>
          )}
        </div>
      </aside>
    </>
  );
}

function firstFile(nodes: FileNode[]): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file') return node;
    const child = firstFile(node.children ?? []);
    if (child) return child;
  }
  return null;
}

function preferredPreviewFile(nodes: FileNode[]): FileNode | null {
  return findReadme(nodes, true) ?? findReadme(nodes, false) ?? firstFile(nodes);
}

function flattenFileOptions(nodes: FileNode[], parentPath = ''): DrawerFileOption[] {
  return nodes.flatMap((node) => {
    const path = parentPath ? `${parentPath}/${node.name}` : node.name;
    if (node.type === 'file') {
      return [{ id: node.id, path: node.path || path, label: node.path || path }];
    }
    return flattenFileOptions(node.children ?? [], path);
  });
}

function findReadme(nodes: FileNode[], rootOnly: boolean): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file' && node.name.toLowerCase() === 'readme.md') return node;
    if (!rootOnly && node.type === 'directory') {
      const child = findReadme(node.children ?? [], false);
      if (child) return child;
    }
  }
  return null;
}

function metadataSourceLabel(source?: string): string {
  return genericMetadataSourceLabel(source);
}

function DrawerStatusBadge({ status }: { status: ItemStatus }) {
  return <span className={`status-badge ${status}`}>{statusLabels[status]}</span>;
}

function autoSaveLabel(state: 'idle' | 'pending' | 'saving' | 'saved' | 'error'): string {
  switch (state) {
    case 'pending':
      return 'Autosave pending';
    case 'saving':
      return 'Saving...';
    case 'saved':
      return 'Saved';
    case 'error':
      return 'Autosave failed';
    case 'idle':
    default:
      return 'Autosave on';
  }
}

function clearTimeoutRef(ref: MutableRefObject<number | null>) {
  if (ref.current === null) return;
  window.clearTimeout(ref.current);
  ref.current = null;
}

function operationErrorMessage(caught: unknown, fallback: string) {
  const message = caught instanceof Error ? caught.message : fallback;
  const hint = caught instanceof ApiError ? caught.recoveryHint : '';
  return [message, hint].filter(Boolean).join(' ');
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function readStoredToggle(key: string): boolean {
  return readStringPreference(key) === '1';
}

function sameJSON(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/** What a Refresh on the Tasks board says the sources returned. */
export function sourcesChecked(sources: { found: number; added: number; lastError?: string }[]): string {
  const failed = sources.find((source) => source.lastError);
  if (failed) return `Jira: ${failed.lastError}`;
  const found = sources.reduce((sum, source) => sum + source.found, 0);
  const added = sources.reduce((sum, source) => sum + source.added, 0);
  return `Jira checked: ${found} found, ${added} new`;
}
