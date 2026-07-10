import { Fragment, memo, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, Dispatch, DragEvent, MouseEvent, MutableRefObject, PointerEvent as ReactPointerEvent, SetStateAction } from 'react';
import { BookmarkPlus, ChevronDown, Code2, FileText, Filter, FolderGit2, GitBranch, GripVertical, Info, KanbanSquare as WorkstreamIcon, RefreshCw, RotateCw, Search, SlidersHorizontal, Ticket, Trash2, X } from 'lucide-react';
import { FileMenu } from '../components/FileMenu';
import { RecentGitActivity } from '../components/RecentGitActivity';
import { StatusMenu } from '../components/StatusMenu';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { JiraItemPanel } from '../features/jira/JiraItemPanel';
import { ApiError, api, statusLabels, statusOrder } from '../lib/api';
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
import { labels, metadataSourceLabel as genericMetadataSourceLabel } from '../lib/vocabulary';
import { emptyFilters, filterPlans, sourceFacetOptions, sourceLabel } from '../features/workstream/filtering';
import type { FacetOption, FilterKey, Filters } from '../features/workstream/filtering';
import { applyItemStatus, isDropStatus, isItemDraggable } from '../features/workstream/dragAndDrop';
import { BranchSnapshotPicker } from '../features/workstream/BranchSnapshotPicker';
import { inferCompatibilityFields, lastPathSegment, previewPathSegments } from '../features/workspaces/sourceSettings';
import { notifyReliabilityChanged } from '../features/reliability/hooks';

type DrawerTab = 'preview' | 'raw' | 'diff';
type DrawerSideTab = 'info' | 'git' | 'jira';
type DrawerFileOption = { id: string; path: string; label: string };
const DRAG_CLICK_SUPPRESSION_MS = 350;
const UNSORTED_SELECTION_ID = 'unsorted';

type SourceItemsEditorState = {
  workspace: WorkspaceConfig;
  directory: string;
  exists: boolean;
  mode?: string;
  card: SourceStructureCard;
  warnings: string[];
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  unsortedPreview: SourceStructurePreview[];
  preview: SourceStructurePreview[];
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

export function WorkstreamPage({ workspace, refreshKey, visibleStatuses = statusOrder, focusedItemId, onOpenPlan, onWorkspacesChanged, onOpenWorkspaces }: {
  workspace?: WorkspaceConfig;
  refreshKey: number;
  visibleStatuses?: ItemStatus[];
  focusedItemId?: string;
  onOpenPlan: (itemId: string) => void;
  onWorkspacesChanged: () => void | Promise<void>;
  onOpenWorkspaces?: () => void;
}) {
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [query, setQuery] = useState('');
  const [items, setPlans] = useState<ItemSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [scanState, setScanState] = useState('');
  const [openFacet, setOpenFacet] = useState<FilterKey | ''>('');
  const [drawerPlanId, setDrawerPlanId] = useState('');
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
  const [dragTargetStatus, setDragTargetStatus] = useState<ItemStatus | ''>('');
  const [workspaceGitStatus, setWorkspaceGitStatus] = useState<GitStatus | null>(null);
  const [workspaceBranchCurrent, setWorkspaceBranchCurrent] = useState('');
  const [workspaceBranchList, setWorkspaceBranchList] = useState<string[]>([]);
  const [selectedBranch, setSelectedBranch] = useState('');
  const [branchContext, setBranchContext] = useState<WorkstreamBranchLoadResult | null>(null);
  const [sourceItemsOpen, setSourceItemsOpen] = useState(false);
  const [sourceItemsLoading, setSourceItemsLoading] = useState(false);
  const [sourceItemsSaving, setSourceItemsSaving] = useState(false);
  const [sourceItemsDirectory, setSourceItemsDirectory] = useState('');
  const [sourceItemsError, setSourceItemsError] = useState('');
  const [sourceItemsEditor, setSourceItemsEditor] = useState<SourceItemsEditorState | null>(null);
  const suppressPreviewRef = useRef<{ itemId: string; until: number } | null>(null);
  const appliedFocusRef = useRef('');
  const text = query;

  const loadBranch = async (branch = selectedBranch, force = false) => {
    if (!workspace) return;
    setLoading(true);
    setError('');
    try {
      const result = await api.loadWorkstreamBranch(workspace.id, { branch: branch || undefined, force });
      setBranchContext(result);
      setSelectedBranch(result.branch);
      setPlans(result.items);
    } catch (err) {
      try {
        const fallbackItems = await api.items(new URLSearchParams({ workspaceId: workspace.id }));
        setBranchContext(null);
        setSelectedBranch(branch || workspace.baselineBranch);
        setPlans(fallbackItems);
        setError('');
      } catch {
        setError(err instanceof Error ? err.message : 'Failed to load branch snapshot');
        setPlans([]);
      }
    } finally {
      setLoading(false);
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
    void loadBranch(workspace.lastSelectedBranch || workspace.baselineBranch, false);
  }, [workspace?.id, refreshKey]);

  useEffect(() => {
    if (!focusedItemId || loading) return;
    const focusKey = `${workspace?.id ?? ''}:${focusedItemId}:${selectedBranch}`;
    if (appliedFocusRef.current === focusKey) return;
    const item = items.find((candidate) => candidate.id === focusedItemId);
    if (!item) return;
    appliedFocusRef.current = focusKey;
    setQuery(item.identifier || item.title);
    setDrawerPlanId(item.id);
  }, [focusedItemId, items, loading, selectedBranch, workspace?.id]);

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
      setNewPlanOpen(false);
      setJiraLookup(null);
      setNewPlanError('');
    };
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [newPlanOpen]);

  useEffect(() => {
    api.savedFilters()
      .then((saved) => setSavedFilters(saved.filter((filter) => !filter.workspaceId || filter.workspaceId === workspace?.id)))
      .catch(() => setSavedFilters([]));
  }, [workspace?.id]);

  useEffect(() => {
    if (!workspace) {
      setWorkspaceGitStatus(null);
      setWorkspaceBranchCurrent('');
      setWorkspaceBranchList([]);
      return;
    }
    let active = true;
    api.gitStatus(workspace.id)
      .then((status) => {
        if (active) setWorkspaceGitStatus(status);
      })
      .catch(() => {
        if (active) setWorkspaceGitStatus(null);
      });
    api.workspaceBranches(workspace.id)
      .then((response) => {
        if (!active) return;
        setWorkspaceBranchCurrent(response.current);
        setWorkspaceBranchList(response.branches);
      })
      .catch(() => {
        if (!active) return;
        setWorkspaceBranchCurrent('');
        setWorkspaceBranchList([]);
      });
    return () => { active = false; };
  }, [workspace]);

  const sourceOptions = useMemo(() => sourceFacetOptions(items, workspace), [items, workspace]);
  const filteredPlans = useMemo(() => filterPlans(items, filters, text, workspace), [items, filters, text, workspace]);
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
    { key: 'scopes', title: labels.scope, options: services.map((scope) => ({ value: scope, label: scope })) },
    { key: 'statuses', title: 'Status', options: statusOrder.map((item) => ({ value: item, label: statusLabels[item] })) },
    { key: 'authors', title: 'Authors', options: authors.map((author) => ({ value: author, label: author })) }
  ];
  const activeFilterCount = filters.sources.length
    + filters.scopes.length
    + filters.statuses.length
    + filters.authors.length
    + (text ? 1 : 0);
  const sourceMode = branchContext?.sourceMode ?? 'working_tree';
  const displayedStatuses = useMemo(() => {
    const visible = new Set(visibleStatuses);
    const statuses = statusOrder.filter((status) => visible.has(status));
    return statuses.length > 0 ? statuses : statusOrder;
  }, [visibleStatuses]);
  const boardStyle = useMemo(() => ({
    gridTemplateColumns: displayedStatuses
      .flatMap((status) => status === 'unsorted' ? ['minmax(260px, 1fr)', '44px'] : ['minmax(260px, 1fr)'])
      .join(' ')
  }) as CSSProperties, [displayedStatuses]);
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
    const firstWorkspaceUnsortedSource = workspace.sources.find((source) => unsortedSources.has(source));
    if (firstWorkspaceUnsortedSource) return firstWorkspaceUnsortedSource;
    return Array.from(unsortedSources)[0] ?? workspace.sources[0] ?? '';
  }, [items, workspace]);
  const activeItem = activeItemId ? items.find((item) => item.id === activeItemId) : undefined;

  const scan = async () => {
    if (!workspace) return;
    setScanState('Refreshing');
    try {
      const result = await api.loadWorkstreamBranch(workspace.id, { branch: selectedBranch || undefined, force: true });
      notifyReliabilityChanged();
      setScanState(`${result.itemCount} items indexed`);
      setBranchContext(result);
      setSelectedBranch(result.branch);
      setPlans(result.items);
      onWorkspacesChanged();
    } catch (err) {
      setScanState(err instanceof Error ? err.message : 'Refresh failed');
    }
  };

  const reloadPlans = async () => {
    if (!workspace) return;
    await loadBranch(selectedBranch, false);
  };

  const moveItem = async (itemId: string, status: ItemStatus) => {
    const item = items.find((candidate) => candidate.id === itemId);
    if (!item || !isItemDraggable(item) || !isDropStatus(status) || item.status === status || pendingItemIds.has(itemId)) return;

    const previousStatus = item.status;
    setError('');
    const materializeConfirmed = confirmSnapshotMaterialization(item, 'status');
    if (materializeConfirmed === null) return;
    setPlans((current) => applyItemStatus(current, itemId, status));
    setPendingItemIds((current) => new Set(current).add(itemId));
    try {
      const result = await api.updateStatus(itemId, { status, materializeConfirmed });
      setPlans((current) => current.map((candidate) => candidate.id === itemId ? { ...candidate, ...result.item } : candidate));
      notifyReliabilityChanged();
      await onWorkspacesChanged();
    } catch (err) {
      setPlans((current) => current.map((candidate) => (
        candidate.id === itemId && candidate.status === status ? { ...candidate, status: previousStatus } : candidate
      )));
      setError(err instanceof Error ? err.message : 'Status update failed');
    } finally {
      setPendingItemIds((current) => {
        const next = new Set(current);
        next.delete(itemId);
        return next;
      });
    }
  };

  const handleCardDragStart = (event: DragEvent<HTMLElement>, itemId: string) => {
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
    if (!activeItemId || !isDropStatus(status)) return;
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
    event.preventDefault();
    setDragTargetStatus('');
    void moveItem(itemId, status);
  };

  const fetchJiraIssue = async () => {
    if (!workspace || !newPlanDraft.jiraKey.trim()) return;
    setJiraLookupLoading(true);
    setNewPlanError('');
    try {
      const result = await api.workspaceJiraIssue(workspace.id, newPlanDraft.jiraKey.trim());
      setJiraLookup(result);
      if (result.state === 'available' && result.issue) {
        const issue = result.issue;
        const nextTags = jiraTags(issue).join(', ');
        setNewPlanDraft((draft) => ({
          ...draft,
          identifier: draft.identifier.trim() || issue.key,
          title: draft.title.trim() || issue.summary,
          owner: draft.owner.trim() || issue.assignee?.displayName || '',
          tags: draft.tags.trim() || nextTags
        }));
      }
    } catch (err) {
      setJiraLookup(null);
      setNewPlanError(err instanceof Error ? err.message : 'Jira lookup failed');
    } finally {
      setJiraLookupLoading(false);
    }
  };

  const createPlan = async () => {
    if (!workspace) return;
    setCreatingPlan(true);
    setNewPlanError('');
    try {
      const source = newPlanDraft.source || workspace.sources[0] || 'plans';
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
      setNewPlanOpen(false);
      setNewPlanDraft(emptyNewWorkItemDraft());
      setJiraLookup(null);
      await onWorkspacesChanged();
      await reloadPlans();
      onOpenPlan(result.item.id);
    } catch (err) {
      setNewPlanError(err instanceof Error ? err.message : 'Item creation failed');
    } finally {
      setCreatingPlan(false);
    }
  };

  const loadSourceItemsSettings = async (directory: string) => {
    if (!workspace) return;
    setSourceItemsLoading(true);
    setSourceItemsError('');
    setSourceItemsDirectory(directory);
    try {
      const result = await api.sourceStructure(workspace.id, directory);
      setSourceItemsEditor(sourceItemsEditorFromResult(workspace, directory, result));
    } catch (err) {
      setSourceItemsEditor(null);
      setSourceItemsError(err instanceof Error ? err.message : 'Source settings failed to load');
    } finally {
      setSourceItemsLoading(false);
    }
  };

  const openSourceItemsDialog = async () => {
    if (!workspace || workspace.sources.length === 0) {
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
          cards: [withInferredCompatibilityFields(sourceItemsEditor.card, sourceItemsEditor.directory)]
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
    const confirmed = window.confirm(`Reset Source Items for ${sourceItemsEditor.directory}? This removes workspace-settings.yaml and scans the source again.`);
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
    if (nextFilters.branches.length > 0) {
      void loadBranch(nextFilters.branches[0], false);
      nextFilters.branches = [];
    }
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
      <div className="page-title workstream-title">
        <div className="workstream-heading">
          <div>
            <h1><WorkstreamIcon size={22} /> Workstream</h1>
            <span><FolderGit2 size={15} /> {workspace?.name ?? 'No workspace selected'}</span>
          </div>
        </div>
        {workspace && (
          <div className="workspace-context" aria-label="Workspace context">
            <BranchSnapshotPicker
              selectedBranch={selectedBranch}
              currentCheckoutBranch={currentBranch}
              sourceMode={sourceMode}
              branches={branchOptions}
              ariaLabel="Select board branch"
              listboxLabel="Board branches"
              onSelect={(branch) => void loadBranch(branch, false)}
            />
            {workspace.sources.slice(0, 3).map((directory) => (
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
        <button className="secondary" onClick={scan}>
          <RotateCw size={16} /> Refresh
            </button>
            <button className="primary" onClick={() => setNewPlanOpen(true)} disabled={sourceMode === 'snapshot'}>
              + New Work Item
            </button>
        <span className="scan-state">{scanState}</span>
      </div>
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
      <SelectedFilters facets={facetConfig} filters={filters} onRemove={toggleFilter} />
      <div className="filter-summary">
        <span>{filteredPlans.length} of {items.length} items</span>
        {activeFilterCount > 0 && <span>{activeFilterCount} active filters</span>}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="workstream-board" style={boardStyle} aria-busy={loading}>
        {displayedStatuses.map((column) => (
          <Fragment key={column}>
            <WorkstreamColumn
              status={column}
              itemCount={grouped.get(column)?.length ?? 0}
              loading={loading}
              dragActive={Boolean(activeItem)}
              dragTargetStatus={dragTargetStatus}
              onDragOver={handleColumnDragOver}
              onDragLeave={handleColumnDragLeave}
              onDrop={handleColumnDrop}
              onCreate={column === 'unsorted' ? undefined : () => {
                setNewPlanDraft((draft) => ({ ...draft, status: column, source: workspace?.sources[0] ?? '' }));
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
                  onDragStart={(event) => handleCardDragStart(event, plan.id)}
                  onDragEnd={() => handleCardDragEnd(plan.id)}
                  onPreview={() => {
                    const suppressed = suppressPreviewRef.current;
                    if (suppressed?.itemId === plan.id && window.performance.now() <= suppressed.until) {
                      return;
                    }
                    suppressPreviewRef.current = null;
                    setDrawerPlanId(plan.id);
                  }}
                  onOpen={() => onOpenPlan(plan.id)}
                  onMove={(status) => moveItem(plan.id, status)}
                />
              ))}
            </WorkstreamColumn>
            {column === 'unsorted' && (
              <button className="workstream-separator" type="button" onClick={() => void openSourceItemsDialog()} title="Configure source items">
                <span className="separator-arrow">▶</span>
                <span className="separator-count">{grouped.get('unsorted')?.length ?? 0}</span>
                <span className="separator-label">Configure source items</span>
                <SlidersHorizontal size={15} />
              </button>
            )}
          </Fragment>
        ))}
      </div>
      {drawerPlanId && (
        <PlanPreviewDrawer
          itemId={drawerPlanId}
          refreshKey={refreshKey}
          onClose={() => setDrawerPlanId('')}
          onOpenFull={() => onOpenPlan(drawerPlanId)}
          onChanged={async () => {
            await onWorkspacesChanged();
            await reloadPlans();
          }}
        />
      )}
      {sourceItemsOpen && workspace && (
        <div className="modal-backdrop" role="presentation">
          <div className="modal-panel source-structure-modal" role="dialog" aria-modal="true" aria-label={labels.sourceStructure}>
            <header>
              <div>
                <h2>{labels.sourceStructure}</h2>
                <span>{workspace.name} / {sourceItemsDirectory || workspace.sources[0]}</span>
              </div>
              <button className="icon-button" type="button" onClick={() => setSourceItemsOpen(false)} disabled={sourceItemsSaving} aria-label="Close source items">
                <X size={16} />
              </button>
            </header>
            {workspace.sources.length > 1 && (
              <label className="repo-field">
                Source
                <select value={sourceItemsDirectory} onChange={(event) => void loadSourceItemsSettings(event.target.value)} disabled={sourceItemsLoading || sourceItemsSaving}>
                  {workspace.sources.map((directory) => <option key={directory} value={directory}>{directory}</option>)}
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
                    <span>Saving creates workspace-settings.yaml inside this source.</span>
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
        <div className="modal-backdrop" role="presentation">
          <section className={newPlanDraft.origin === 'jira' ? 'modal-panel new-work-item-modal jira-mode' : 'modal-panel new-work-item-modal'} role="dialog" aria-modal="true" aria-label="Create new work item">
            <header>
              <h2>New Work Item</h2>
              <button type="button" className="icon-button" onClick={() => {
                setNewPlanOpen(false);
                setJiraLookup(null);
                setNewPlanError('');
              }}><X size={16} /></button>
            </header>
            <div className={newPlanDraft.origin === 'jira' ? 'metadata-form jira-work-item-form' : 'metadata-form'}>
              <div className="segmented-control" role="group" aria-label="Item origin">
                <button
                  type="button"
                  className={newPlanDraft.origin === 'blank' ? 'active' : ''}
                  onClick={() => {
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
                  <label>Source<select value={newPlanDraft.source || workspace.sources[0] || ''} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, source: event.target.value }))}>
                    {workspace.sources.map((directory) => <option value={directory} key={directory}>{directory}</option>)}
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
                    <label>Source<select value={newPlanDraft.source || workspace.sources[0] || ''} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, source: event.target.value }))}>
                      {workspace.sources.map((directory) => <option value={directory} key={directory}>{directory}</option>)}
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
                            <dl className="jira-issue-preview-meta">
                              <div><dt>Type</dt><dd>{jiraLookup.issue.issueType || 'Issue'}</dd></div>
                              <div><dt>Status</dt><dd>{jiraLookup.issue.status || 'Unknown'}</dd></div>
                              <div><dt>Assignee</dt><dd>{jiraLookup.issue.assignee?.displayName || 'Unassigned'}</dd></div>
                              {jiraLookup.issue.priority && <div><dt>Priority</dt><dd>{jiraLookup.issue.priority}</dd></div>}
                            </dl>
                            {jiraLookup.issue.description && <p className="jira-issue-preview-description">{jiraLookup.issue.description}</p>}
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
              <button type="button" className="ghost" onClick={() => setNewPlanOpen(false)}>Cancel</button>
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
                  <option value="draft">Draft</option>
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
  const proposals = result.proposals ?? [];
  const selectedProposal = !result.exists && proposals.length > 0 ? proposals[0] : undefined;
  const unsortedPreview = [unsortedSourcePreview(directory)];
  const selectedProposalId = selectedProposal?.id ?? (!result.exists ? UNSORTED_SELECTION_ID : undefined);
  return {
    workspace,
    directory,
    exists: result.exists,
    mode: result.mode,
    card: normalizeSettingsCard(selectedProposal?.card ?? result.settings?.cards?.[0], directory),
    warnings: (result.warnings ?? []).map((warning) => warning.message),
    proposals,
    selectedProposalId,
    unsortedPreview,
    preview: selectedProposal?.preview ?? (!result.exists ? unsortedPreview : result.preview ?? [])
  };
}

function unsortedSourcePreview(directory: string): SourceStructurePreview {
  const sourceName = lastPathSegment(directory) || 'source';
  return {
    path: directory,
    source: sourceName,
    item: sourceName,
    scope: sourceName,
    identifier: sourceName,
    title: sourceName,
    status: 'unsorted',
    tags: [sourceName]
  };
}

function normalizeSettingsCard(card?: SourceStructureCard, directory = 'source'): SourceStructureCard {
  const legacyFields = card?.fields as SourceStructureCard['fields'] & { service?: string; ticket?: string } | undefined;
  return withInferredCompatibilityFields({
    pathPattern: genericTemplate(card?.pathPattern || '{folder}/feature/{item}'),
    fields: {
      source: genericTemplate(legacyFields?.source || legacyFields?.scope || legacyFields?.service || directory),
      item: genericTemplate(legacyFields?.item || legacyFields?.identifier || legacyFields?.ticket || '{item}'),
      scope: genericTemplate(legacyFields?.source || legacyFields?.scope || legacyFields?.service || directory),
      identifier: genericTemplate(legacyFields?.item || legacyFields?.identifier || legacyFields?.ticket || '{item}'),
      title: card?.fields?.title || 'readme_heading',
      status: card?.fields?.status || 'draft',
      owner: card?.fields?.owner || '',
      tags: Array.isArray(card?.fields?.tags) ? card.fields.tags : ['docs']
    }
  }, directory);
}

function genericTemplate(value: string): string {
  return value
    .replaceAll('{service}', '{folder}')
    .replaceAll('{scope}', '{folder}')
    .replaceAll('{ticket}', '{item}')
    .replaceAll('{identifier}', '{item}');
}

function withInferredCompatibilityFields(card: SourceStructureCard, directory: string): SourceStructureCard {
  return {
    ...card,
    fields: {
      ...card.fields,
      source: inferCompatibilityFields(card.pathPattern, directory).scope,
      item: inferCompatibilityFields(card.pathPattern, directory).identifier,
      ...inferCompatibilityFields(card.pathPattern, directory)
    }
  };
}

function applySourceItemsProposal(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>,
  proposal: SourceStructureProposal
) {
  setSourceItemsEditor((current) => {
    if (!current) return current;
    return {
      ...current,
      card: normalizeSettingsCard(proposal.card, current.directory),
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
      nextCard.fields.identifier = normalized;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, true);
      if (suggestedTemplate) {
        nextCard.fields.item = suggestedTemplate;
        nextCard.fields.identifier = suggestedTemplate;
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

function FacetMenu({ title, options, selected, open, onOpen, onClose, onToggle, onClear }: {
  title: string;
  options: FacetOption[];
  selected: string[];
  open: boolean;
  onOpen: () => void;
  onClose: () => void;
  onToggle: (value: string) => void;
  onClear: () => void;
}) {
  const [optionQuery, setOptionQuery] = useState('');
  const menuRef = useRef<HTMLElement | null>(null);
  const visibleOptions = options.filter((option) => option.value);
  const searchedOptions = visibleOptions.filter((option) => option.label.toLowerCase().includes(optionQuery.toLowerCase()));

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose();
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      onClose();
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [onClose, open]);

  if (visibleOptions.length === 0) return null;
  return (
    <section className="facet-menu" ref={menuRef}>
      <button type="button" className={selected.length > 0 ? 'facet-trigger active' : 'facet-trigger'} onClick={onOpen}>
        <span>{title}</span>
        <span className="facet-trigger-right">
          {selected.length > 0 && <strong>{selected.length}</strong>}
          <ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} />
        </span>
      </button>
      {open && (
        <div className="facet-popover">
          <div className="facet-popover-header">
            <strong>{title}</strong>
            <button type="button" onClick={onClear} disabled={selected.length === 0}>Clear</button>
          </div>
          {visibleOptions.length > 8 && (
            <label className="facet-search">
              <Search size={14} />
              <input value={optionQuery} onChange={(event) => setOptionQuery(event.target.value)} placeholder={`Find ${title.toLowerCase()}...`} />
            </label>
          )}
          <div className="facet-option-list">
            {searchedOptions.map((option) => (
              <label className="facet-option" key={option.value}>
                <input type="checkbox" checked={selected.includes(option.value)} onChange={() => onToggle(option.value)} />
                <span>{option.label}</span>
              </label>
            ))}
            {searchedOptions.length === 0 && <span className="facet-empty">No matches</span>}
          </div>
        </div>
      )}
    </section>
  );
}

function SelectedFilters({ facets, filters, onRemove }: { facets: { key: FilterKey; title: string; options: FacetOption[] }[]; filters: Filters; onRemove: (key: FilterKey, value: string) => void }) {
  const chips = facets.flatMap((facet) => filters[facet.key].map((value) => ({
    key: facet.key,
    value,
    title: facet.title,
    label: facet.options.find((option) => option.value === value)?.label ?? value
  })));
  if (chips.length === 0) return null;
  return (
    <div className="selected-filters">
      {chips.map((chip) => (
        <button type="button" key={`${chip.key}-${chip.value}`} onClick={() => onRemove(chip.key, chip.value)}>
          <span>{chip.title}: {chip.label}</span>
          <X size={13} />
        </button>
      ))}
    </div>
  );
}

function confirmSnapshotMaterialization(item: ItemSummary | ItemDetail | null, operation: 'file' | 'metadata' | 'status'): boolean | null {
  if (!item || item.sourceMode !== 'snapshot') return false;
  const copyTarget = item.metadataSource === 'docs'
    ? 'only this docs file'
    : `the whole plan at ${item.itemPath || item.identifier}`;
  const action = operation === 'status' ? 'move it' : operation === 'metadata' ? 'edit its metadata' : 'edit it';
  const message = `This item is loaded from branch ${item.branch}. To ${action}, Kode Stream will copy ${copyTarget} into the current checkout branch, then apply your change there.`;
  return window.confirm(message) ? true : null;
}

function WorkstreamColumn({ status, itemCount, loading, dragActive, dragTargetStatus, onDragOver, onDragLeave, onDrop, onCreate, children }: {
  status: ItemStatus;
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
  const droppable = isDropStatus(status);
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
        <h2>{statusLabels[status]}</h2>
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

const PlanCard = memo(function PlanCard({ item: plan, workspace, pending, active, onDragStart, onDragEnd, onPreview, onOpen, onMove }: {
  item: ItemSummary;
  workspace?: WorkspaceConfig;
  pending: boolean;
  active: boolean;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
  onPreview: () => void;
  onOpen: () => void;
  onMove: (status: ItemStatus) => void;
}) {
  const source = sourceLabel(plan, workspace);
  const docs = plan.metadataSource === 'docs';
  const showItem = plan.identifier.toLowerCase() !== plan.title.toLowerCase();
  const description = plan.description;
  const tags = docs ? plan.tags.filter((tag) => tag !== source && tag !== plan.scope && tag !== plan.identifier) : plan.tags;
  const draggable = isItemDraggable(plan) && !pending;
  const navigate = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    onOpen();
  };
  const classes = ['plan-card'];
  if (docs) classes.push('docs-plan');
  if (draggable) classes.push('draggable');
  if (active) classes.push('dragging');
  if (pending) classes.push('move-pending');
  return (
    <article
      className={classes.join(' ')}
      draggable={draggable}
      aria-busy={pending}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={onPreview}
      role="button"
      tabIndex={0}
      onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        onPreview();
      }
      }}
    >
      <div className="plan-card-title">
        <button type="button" className="plan-card-link plan-card-heading" onPointerDown={(event) => event.stopPropagation()} onClick={navigate}>{plan.title}</button>
        <span className="card-badges">
          {source && <span className={docs ? 'source-badge docs' : 'source-badge'}>{source}</span>}
        </span>
      </div>
      {showItem && <span className="plan-card-identifier">{plan.identifier}</span>}
      {description && <p>{description}</p>}
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {tags.length > 0 && <div className="tags">{tags.slice(0, 3).map((tag: string) => <span key={tag}>{tag}</span>)}</div>}
      {plan.status !== 'unsorted' && plan.metadataSource !== 'docs' && (
        <StatusMenu value={plan.status} onChange={onMove} ariaLabel="Move item status" />
      )}
    </article>
  );
});

function PlanPreviewDrawer({ itemId, refreshKey, onClose, onOpenFull, onChanged }: { itemId: string; refreshKey: number; onClose: () => void; onOpenFull: () => void; onChanged: () => void | Promise<void> }) {
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
  const [sideTab, setSideTab] = useState<DrawerSideTab>('info');
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
  const [gitActivity, setGitActivity] = useState<GitActivityEntry[]>([]);
  const [gitActivityLoading, setGitActivityLoading] = useState(false);
  const [gitMessage, setGitMessage] = useState('');
  const [selectedGitPaths, setSelectedGitPaths] = useState<string[]>([]);
  const [branchName, setBranchName] = useState('');
  const [gitBusy, setGitBusy] = useState('');
  const [gitActivityOpen, setGitActivityOpen] = useState(() => readStoredToggle('workstream.drawer.gitActivityOpen'));
  const [error, setError] = useState('');
  const [width, setWidth] = useState(1120);
  const [topOffset, setTopOffset] = useState(() => Math.max(0, 68 - window.scrollY));
  const autoSaveTimerRef = useRef<number | null>(null);
  const autoSaveSettledTimerRef = useRef<number | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const drawerStyle = {
    '--drawer-width': `${width}px`,
    '--drawer-top': `${topOffset}px`,
  } as CSSProperties & Record<'--drawer-width' | '--drawer-top', string>;
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
      if (!dirtyFile) {
        onClose();
        return;
      }
      void saveDrawerFileNow().then((saved) => {
        if (saved) onClose();
      });
    };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [dirtyFile, editorContent, file, onClose, savedContent]);

  useEffect(() => {
    const updateTopOffset = () => setTopOffset(Math.max(0, 68 - window.scrollY));
    window.addEventListener('scroll', updateTopOffset, { passive: true });
    return () => window.removeEventListener('scroll', updateTopOffset);
  }, []);

  useEffect(() => {
    let active = true;
    setPlan(null);
    setFile(null);
    setFiles([]);
    setDiff('');
    setGitStatus(null);
    setError('');
    api.item(itemId).then((payload) => {
      if (!active) return;
      setPlan(payload);
      api.gitStatus(payload.workspaceId).then((status) => {
        if (active) setGitStatus(status);
      }).catch(() => {
        if (active) setGitStatus(null);
      });
    }).catch((err: Error) => {
      if (active) setError(err.message);
    });
    api.files(itemId).then(async (tree) => {
      if (!active) return;
      setFiles(tree);
      const previewFile = preferredPreviewFile(tree);
      if (previewFile) {
        try {
          const content = await api.file(itemId, previewFile.id);
          if (active) {
            setFile(content);
            setEditorContent(content.content);
            setSavedContent(content.content);
            setAutoSaveState('idle');
          }
        } catch (err) {
          if (active) setError(err instanceof Error ? err.message : 'File failed to load');
        }
      }
    }).catch((err: Error) => {
      if (active) setError(err.message);
    });
    api.diff(itemId).then((payload) => {
      if (active) setDiff(payload.diff || 'No local changes.');
    }).catch(() => {
      if (active) setDiff('No diff available.');
    });
    return () => {
      active = false;
      clearDrawerAutoSaveTimers();
    };
  }, [itemId, refreshKey]);

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
    clearTimeoutRef(autoSaveTimerRef);
    clearTimeoutRef(autoSaveSettledTimerRef);
    setSavingFile(true);
    setAutoSaveState('saving');
    setError('');
    try {
      const materializeConfirmed = confirmSnapshotMaterialization(plan, 'file');
      if (materializeConfirmed === null) {
        setAutoSaveState('idle');
        return false;
      }
      const updated = await api.saveFile(itemId, targetFile.id, { content, expectedHash: targetFile.hash, materializeConfirmed });
      notifyReliabilityChanged();
      setFile(updated);
      setSavedContent(content);
      setAutoSaveState('saved');
      autoSaveSettledTimerRef.current = window.setTimeout(() => setAutoSaveState('idle'), 1400);
      clearTimeoutRef(autoSaveRefreshTimerRef);
      autoSaveRefreshTimerRef.current = window.setTimeout(() => {
        api.diff(itemId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
        if (plan) void loadGitStatus(plan.workspaceId);
      }, 600);
      return true;
    } catch (err) {
      setError(operationErrorMessage(err, 'File save failed'));
      setAutoSaveState('error');
      return false;
    } finally {
      setSavingFile(false);
    }
  };

  const loadDrawerFile = async (fileId: string) => {
    try {
      const content = await api.file(itemId, fileId);
      setFile(content);
      setEditorContent(content.content);
      setSavedContent(content.content);
      setAutoSaveState('idle');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
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

  const loadGitStatus = async (workspaceId: string) => {
    try {
      setGitStatus(await api.gitStatus(workspaceId));
    } catch {
      setGitStatus(null);
    }
  };

  const loadGitActivity = async (workspaceId: string, path: string) => {
    setGitActivityLoading(true);
    try {
      setGitActivity(await api.gitActivity(workspaceId, { path: path || undefined, limit: 8 }));
    } catch {
      setGitActivity([]);
    } finally {
      setGitActivityLoading(false);
    }
  };

  useEffect(() => {
    if (!plan) {
      setGitActivity([]);
      setGitActivityLoading(false);
      return;
    }
    void loadGitActivity(plan.workspaceId, activityPath);
  }, [plan?.workspaceId, activityPath]);

  const saveMetadata = async () => {
    if (!plan) return;
    setSavingMetadata(true);
    setError('');
    try {
      const materializeConfirmed = confirmSnapshotMaterialization(plan, 'metadata');
      if (materializeConfirmed === null) return;
      const result = await api.saveMetadata(itemId, { ...metadataDraft, materializeConfirmed });
      notifyReliabilityChanged();
      setPlan(result.item);
      await loadGitStatus(plan.workspaceId);
      await onChanged();
    } catch (err) {
      setError(operationErrorMessage(err, 'Metadata save failed'));
    } finally {
      setSavingMetadata(false);
    }
  };

  const runGitOperation = async (operation: 'fetch' | 'pull' | 'push') => {
    if (!plan) return;
    setGitBusy(operation);
    setError('');
    try {
      const confirm = operation === 'pull' && Boolean(gitStatus?.dirty);
      const result = operation === 'fetch'
        ? await api.gitFetch(plan.workspaceId)
        : operation === 'pull'
          ? await api.gitPull(plan.workspaceId, { confirm })
          : await api.gitPush(plan.workspaceId);
      notifyReliabilityChanged();
      setGitStatus(result.status);
      if (operation === 'pull') {
        await onChanged();
      }
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(operationErrorMessage(err, `${operation} failed`));
    } finally {
      setGitBusy('');
    }
  };

  const toggleGitPath = (path: string) => {
    setSelectedGitPaths((current) => current.includes(path) ? current.filter((item) => item !== path) : [...current, path]);
  };

  const commitSelectedPaths = async () => {
    if (!plan) return;
    setGitBusy('commit');
    setError('');
    try {
      const result = await api.gitCommit(plan.workspaceId, { message: gitMessage, paths: selectedGitPaths });
      notifyReliabilityChanged();
      setGitStatus(result.status);
      await loadGitActivity(plan.workspaceId, activityPath);
      setGitMessage('');
      setSelectedGitPaths([]);
      await onChanged();
      api.diff(itemId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(operationErrorMessage(err, 'Commit failed'));
    } finally {
      setGitBusy('');
    }
  };

  const createAndSwitchBranch = async () => {
    if (!plan || !branchName.trim()) return;
    setGitBusy('branch');
    setError('');
    try {
      const result = await api.createBranch(plan.workspaceId, { name: branchName.trim(), checkout: true });
      notifyReliabilityChanged();
      setGitStatus(result.status);
      setBranchName('');
      await onChanged();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(operationErrorMessage(err, 'Branch operation failed'));
    } finally {
      setGitBusy('');
    }
  };

  const startResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startingWidth = width;

    const onPointerMove = (moveEvent: PointerEvent) => {
      const delta = startX - moveEvent.clientX;
      setWidth(Math.min(1120, Math.max(460, startingWidth + delta)));
    };

    const onPointerUp = () => {
      document.body.classList.remove('is-resizing-drawer');
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
    };

    document.body.classList.add('is-resizing-drawer');
    window.addEventListener('pointermove', onPointerMove);
    window.addEventListener('pointerup', onPointerUp, { once: true });
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
            <button type="button" className="secondary" onClick={openFullDetails}>Open details</button>
            <button type="button" className="icon-button" aria-label="Close preview" onClick={closeDrawer}><X size={16} /></button>
          </div>
        </header>
        {error && <p className="error drawer-error">{error}</p>}
        <div className="plan-drawer-body">
          <section className="drawer-main">
            {plan?.description && (
              <section className="drawer-section">
                <h3>Description</h3>
                <p>{plan.description}</p>
              </section>
            )}
            <section className="drawer-section">
              <label className="drawer-file-picker">
                <span>Document</span>
                <FileMenu value={file?.id ?? ''} options={fileOptions} onChange={selectDrawerFile} />
              </label>
              <div className="drawer-tabs">
                <div>
                  <button className={tab === 'preview' ? 'active' : ''} type="button" onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
                  <button className={tab === 'raw' ? 'active' : ''} type="button" onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
                  <button className={tab === 'diff' ? 'active' : ''} type="button" onClick={() => setTab('diff')}><RefreshCw size={15} /> Diff</button>
                </div>
                <span className={`autosave-state ${autoSaveState}`}>{autoSaveLabel(autoSaveState)}</span>
              </div>
              {dirtyFile && <div className="edit-state-banner">{autoSaveLabel(autoSaveState)}</div>}
              {tab === 'preview' && (file ? <ContentViewer file={file} content={editorContent} compact /> : <div className="drawer-empty">No readable file selected.</div>)}
              {tab === 'raw' && (
                <textarea
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
          <aside className="drawer-meta drawer-work-item">
            <h3><Info size={15} /> Work Item</h3>
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
                  {plan?.metadataSource !== 'docs' && (
                    <div className="metadata-form drawer-metadata-form">
                      <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
                      <label>{labels.identifier}<input value={metadataDraft.identifier ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, identifier: event.target.value }))} /></label>
                      <label>Status<StatusMenu value={metadataDraft.status ?? 'draft'} onChange={(status) => setMetadataDraft((draft) => ({ ...draft, status }))} /></label>
                      <label>Owner<input value={metadataDraft.owner ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, owner: event.target.value }))} /></label>
                      <label>Tags<input value={(metadataDraft.tags ?? []).join(', ')} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label>
                    </div>
                  )}
                  <div className="workspace-actions">
                    <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || plan?.metadataSource === 'docs'} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
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
                    localStorage.setItem('workstream.drawer.gitActivityOpen', open ? '1' : '0');
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
            </div>
          </aside>
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
  return localStorage.getItem(key) === '1';
}
