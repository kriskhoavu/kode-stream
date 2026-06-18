import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, MouseEvent, MutableRefObject, PointerEvent as ReactPointerEvent } from 'react';
import { ChevronDown, Code2, FileText, Filter, FolderGit2, GitBranch, GripVertical, Info, KanbanSquare, Pencil, RefreshCw, RotateCw, Search, X } from 'lucide-react';
import { marked } from 'marked';
import { api, editableStatusOrder, statusLabels, statusOrder } from '../lib/api';
import type { FileContent, FileNode, GitStatus, PlanDetail, PlanMetadataUpdateInput, PlanStatus, PlanSummary, RepositoryConfig } from '../lib/types';

type FilterKey = 'sources' | 'statuses' | 'branches' | 'authors';

type Filters = Record<FilterKey, string[]>;

type FacetOption = { value: string; label: string };

type DrawerTab = 'preview' | 'raw' | 'diff';
type DrawerSideTab = 'info' | 'git';
type DrawerFileOption = { id: string; path: string; label: string };

const emptyFilters: Filters = {
  sources: [],
  statuses: [],
  branches: [],
  authors: []
};

export function KanbanPage({ repository, refreshKey, onOpenPlan, onRepositoriesChanged }: {
  repository?: RepositoryConfig;
  refreshKey: number;
  onOpenPlan: (planId: string) => void;
  onRepositoriesChanged: () => void | Promise<void>;
}) {
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [query, setQuery] = useState('');
  const [plans, setPlans] = useState<PlanSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [scanState, setScanState] = useState('');
  const [openFacet, setOpenFacet] = useState<FilterKey | ''>('');
  const [drawerPlanId, setDrawerPlanId] = useState('');
  const [newPlanOpen, setNewPlanOpen] = useState(false);
  const [newPlanDraft, setNewPlanDraft] = useState({ planDirectory: '', service: '', ticket: '', title: '', status: 'draft' as PlanStatus });
  const [newPlanError, setNewPlanError] = useState('');
  const [creatingPlan, setCreatingPlan] = useState(false);
  const text = query;

  useEffect(() => {
    if (!repository) {
      setPlans([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    api.plans(new URLSearchParams({ repositoryId: repository.id }))
      .then(setPlans)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [repository, refreshKey]);

  const sourceOptions = useMemo(() => sourceFacetOptions(plans, repository), [plans, repository]);
  const filteredPlans = useMemo(() => filterPlans(plans, filters, text, repository), [plans, filters, text, repository]);
  const branches = useMemo(() => unique(plans.map((plan) => plan.branch)), [plans]);
  const authors = useMemo(() => unique(plans.map((plan) => plan.author || plan.owner || 'Unknown')), [plans]);
  const facetConfig: { key: FilterKey; title: string; options: FacetOption[] }[] = [
    { key: 'sources', title: 'Source', options: sourceOptions },
    { key: 'statuses', title: 'Status', options: statusOrder.map((item) => ({ value: item, label: statusLabels[item] })) },
    { key: 'authors', title: 'Authors', options: authors.map((author) => ({ value: author, label: author })) },
    { key: 'branches', title: 'Branches', options: branches.map((branch) => ({ value: branch, label: branch })) }
  ];
  const activeFilterCount = Object.values(filters).reduce((sum, values) => sum + values.length, 0) + (text ? 1 : 0);
  const grouped = useMemo(() => {
    const map = new Map<PlanStatus, PlanSummary[]>();
    statusOrder.forEach((item) => map.set(item, []));
    filteredPlans.forEach((plan) => map.get(plan.status)?.push(plan));
    return map;
  }, [filteredPlans]);

  const scan = async () => {
    if (!repository) return;
    setScanState('Scanning');
    try {
      const result = await api.scan(repository.id);
      setScanState(`${result.planCount} plans indexed`);
      onRepositoriesChanged();
      setPlans(await api.plans(new URLSearchParams({ repositoryId: repository.id })));
    } catch (err) {
      setScanState(err instanceof Error ? err.message : 'Scan failed');
    }
  };

  const reloadPlans = async () => {
    if (!repository) return;
    setPlans(await api.plans(new URLSearchParams({ repositoryId: repository.id })));
  };

  const movePlan = async (planId: string, status: PlanStatus) => {
    try {
      await api.updateStatus(planId, { status });
      await onRepositoriesChanged();
      await reloadPlans();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Status update failed');
    }
  };

  const createPlan = async () => {
    if (!repository) return;
    setCreatingPlan(true);
    setNewPlanError('');
    try {
      const planDirectory = newPlanDraft.planDirectory || repository.planDirectories[0] || 'plans';
      const result = await api.createPlan({
        repositoryId: repository.id,
        planDirectory,
        service: newPlanDraft.service.trim(),
        ticket: newPlanDraft.ticket.trim(),
        title: newPlanDraft.title.trim(),
        status: newPlanDraft.status
      });
      setNewPlanOpen(false);
      setNewPlanDraft({ planDirectory: '', service: '', ticket: '', title: '', status: 'draft' });
      await onRepositoriesChanged();
      await reloadPlans();
      onOpenPlan(result.plan.id);
    } catch (err) {
      setNewPlanError(err instanceof Error ? err.message : 'Plan creation failed');
    } finally {
      setCreatingPlan(false);
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

  const clearFilters = () => {
    setFilters(emptyFilters);
    setQuery('');
  };

  if (!repository && !loading) {
    return (
      <section className="empty-state">
        <h1>Kanban</h1>
        <p>Register a local Git repository to create a workspace.</p>
      </section>
    );
  }

  return (
    <section className="kanban-page">
      <div className="page-title kanban-title">
        <div className="kanban-heading">
          <div>
            <h1><KanbanSquare size={22} /> Kanban board</h1>
            <span><FolderGit2 size={15} /> {repository?.name ?? 'No workspace selected'}</span>
          </div>
        </div>
        {repository && repository.planDirectories.length > 0 && (
          <div className="workspace-context" aria-label="Plan sources">
            {repository.planDirectories.slice(0, 3).map((directory) => (
              <span key={directory}>{directory}</span>
            ))}
          </div>
        )}
        </div>
      <div className="board-toolbar">
        <label className="filter-input plan-search">
          <Search size={15} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search plans..." />
        </label>
        <button className="secondary" onClick={scan}>
          <RotateCw size={16} /> Scan
        </button>
        <button className="primary" onClick={() => setNewPlanOpen(true)}>
          + New Plan
        </button>
        <button className="secondary" onClick={clearFilters} disabled={activeFilterCount === 0}>
          <X size={16} /> Clear
        </button>
        <span className="readonly-badge"><Pencil size={15} /> Authoring</span>
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
            onToggle={(value) => toggleFilter(facet.key, value)}
            onClear={() => setFilters((current) => ({ ...current, [facet.key]: [] }))}
          />
        ))}
      </div>
      <SelectedFilters facets={facetConfig} filters={filters} onRemove={toggleFilter} />
      <div className="filter-summary">
        <span>{filteredPlans.length} of {plans.length} items</span>
        {activeFilterCount > 0 && <span>{activeFilterCount} active filters</span>}
      </div>
      {error && <p className="error">{error}</p>}
      <div className="kanban-board" aria-busy={loading}>
        {statusOrder.map((column) => (
          <div className={`kanban-column ${column}`} key={column}>
            <header>
              <h2>{statusLabels[column]}</h2>
              <span>{grouped.get(column)?.length ?? 0}</span>
              <Filter size={14} />
            </header>
            <div className="card-stack">
              {loading && Array.from({ length: 3 }).map((_, index) => <div className="plan-card skeleton" key={index} />)}
              {!loading && grouped.get(column)?.map((plan) => (
                <PlanCard
                  key={plan.id}
                  plan={plan}
                  repository={repository}
                  onPreview={() => setDrawerPlanId(plan.id)}
                  onOpen={() => onOpenPlan(plan.id)}
                  onMove={(status) => movePlan(plan.id, status)}
                />
              ))}
              {!loading && (grouped.get(column)?.length ?? 0) === 0 && <div className="column-empty">No plans</div>}
            </div>
            {column !== 'unsorted' && (
              <button className="new-plan-column-button" type="button" onClick={() => {
                setNewPlanDraft((draft) => ({ ...draft, status: column, planDirectory: repository?.planDirectories[0] ?? '' }));
                setNewPlanOpen(true);
              }}>+ New plan</button>
            )}
          </div>
        ))}
      </div>
      {drawerPlanId && (
        <PlanPreviewDrawer
          planId={drawerPlanId}
          refreshKey={refreshKey}
          onClose={() => setDrawerPlanId('')}
          onOpenFull={() => onOpenPlan(drawerPlanId)}
          onChanged={async () => {
            await onRepositoriesChanged();
            await reloadPlans();
          }}
        />
      )}
      {newPlanOpen && repository && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-label="Create new plan">
            <header>
              <h2>New plan</h2>
              <button type="button" className="icon-button" onClick={() => setNewPlanOpen(false)}><X size={16} /></button>
            </header>
            <div className="metadata-form">
              <label>Source<select value={newPlanDraft.planDirectory || repository.planDirectories[0] || ''} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, planDirectory: event.target.value }))}>
                {repository.planDirectories.map((directory) => <option value={directory} key={directory}>{directory}</option>)}
              </select></label>
              <label>Service<input value={newPlanDraft.service} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, service: event.target.value }))} placeholder="platform" /></label>
              <label>Ticket<input value={newPlanDraft.ticket} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, ticket: event.target.value }))} placeholder="PM-003" /></label>
              <label>Title<input value={newPlanDraft.title} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, title: event.target.value }))} placeholder="Plan title" /></label>
              <label>Status<select value={newPlanDraft.status} onChange={(event) => setNewPlanDraft((draft) => ({ ...draft, status: event.target.value as PlanStatus }))}>
                {editableStatusOrder.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
              </select></label>
            </div>
            {newPlanError && <p className="error">{newPlanError}</p>}
            <footer className="modal-actions">
              <button type="button" className="ghost" onClick={() => setNewPlanOpen(false)}>Cancel</button>
              <button type="button" className="primary" disabled={creatingPlan || !newPlanDraft.service || !newPlanDraft.ticket} onClick={createPlan}>{creatingPlan ? 'Creating...' : 'Create Plan'}</button>
            </footer>
          </section>
        </div>
      )}
    </section>
  );
}

function FacetMenu({ title, options, selected, open, onOpen, onToggle, onClear }: {
  title: string;
  options: FacetOption[];
  selected: string[];
  open: boolean;
  onOpen: () => void;
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
        onOpen();
      }
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => document.removeEventListener('pointerdown', closeOnOutsideClick);
  }, [onOpen, open]);

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

function PlanCard({ plan, repository, onPreview, onOpen, onMove }: { plan: PlanSummary; repository?: RepositoryConfig; onPreview: () => void; onOpen: () => void; onMove: (status: PlanStatus) => void }) {
  const source = sourceLabel(plan, repository);
  const docs = plan.metadataSource === 'docs';
  const navigate = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    onOpen();
  };
  return (
    <article className={docs ? 'plan-card docs-plan' : 'plan-card'} onClick={onPreview} role="button" tabIndex={0} onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        onPreview();
      }
    }}>
      <div className="plan-card-title">
        <button type="button" className="plan-card-link plan-card-heading" onClick={navigate}>{plan.title}</button>
        <span className="card-badges">
          {plan.service && <span className="service-badge">{plan.service}</span>}
          {plan.service && source && <span className="badge-separator">|</span>}
          {source && <span className={docs ? 'source-badge docs' : 'source-badge'}>{source}</span>}
        </span>
      </div>
      <button type="button" className="plan-card-link plan-card-ticket" onClick={navigate}>{plan.ticket}</button>
      <p>{plan.description || plan.ticket}</p>
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {plan.tags.length > 0 && <div className="tags">{plan.tags.slice(0, 3).map((tag) => <span key={tag}>{tag}</span>)}</div>}
      {plan.status !== 'unsorted' && plan.metadataSource !== 'docs' && (
        <select className="status-move-select" value={plan.status} onClick={(event) => event.stopPropagation()} onChange={(event) => onMove(event.target.value as PlanStatus)} aria-label="Move plan status">
          {editableStatusOrder.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
        </select>
      )}
    </article>
  );
}

function PlanPreviewDrawer({ planId, refreshKey, onClose, onOpenFull, onChanged }: { planId: string; refreshKey: number; onClose: () => void; onOpenFull: () => void; onChanged: () => void | Promise<void> }) {
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [editorContent, setEditorContent] = useState('');
  const [savedContent, setSavedContent] = useState('');
  const [savingFile, setSavingFile] = useState(false);
  const [autoSaveState, setAutoSaveState] = useState<'idle' | 'pending' | 'saving' | 'saved' | 'error'>('idle');
  const [metadataDraft, setMetadataDraft] = useState<PlanMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<DrawerTab>('preview');
  const [sideTab, setSideTab] = useState<DrawerSideTab>('info');
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
  const [gitMessage, setGitMessage] = useState('');
  const [selectedGitPaths, setSelectedGitPaths] = useState<string[]>([]);
  const [branchName, setBranchName] = useState('');
  const [gitBusy, setGitBusy] = useState('');
  const [error, setError] = useState('');
  const [width, setWidth] = useState(1120);
  const autoSaveTimerRef = useRef<number | null>(null);
  const autoSaveSettledTimerRef = useRef<number | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const drawerStyle = { '--drawer-width': `${width}px` } as CSSProperties & Record<'--drawer-width', string>;
  const compact = width < 700;
  const dirtyFile = file !== null && editorContent !== savedContent;
  const fileOptions = useMemo(() => flattenFileOptions(files), [files]);
  const dirtyMetadata = Boolean(plan) && (
    (metadataDraft.title ?? '') !== (plan?.title ?? '') ||
    (metadataDraft.service ?? '') !== (plan?.service ?? '') ||
    (metadataDraft.ticket ?? '') !== (plan?.ticket ?? '') ||
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
    let active = true;
    setPlan(null);
    setFile(null);
    setFiles([]);
    setDiff('');
    setGitStatus(null);
    setError('');
    api.plan(planId).then((payload) => {
      if (!active) return;
      setPlan(payload);
      api.gitStatus(payload.repositoryId).then((status) => {
        if (active) setGitStatus(status);
      }).catch(() => {
        if (active) setGitStatus(null);
      });
    }).catch((err: Error) => {
      if (active) setError(err.message);
    });
    api.files(planId).then(async (tree) => {
      if (!active) return;
      setFiles(tree);
      const previewFile = preferredPreviewFile(tree);
      if (previewFile) {
        try {
          const content = await api.file(planId, previewFile.id);
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
    api.diff(planId).then((payload) => {
      if (active) setDiff(payload.diff || 'No local changes.');
    }).catch(() => {
      if (active) setDiff('No diff available.');
    });
    return () => {
      active = false;
      clearDrawerAutoSaveTimers();
    };
  }, [planId, refreshKey]);

  useEffect(() => {
    if (!plan) return;
    setMetadataDraft({
      title: plan.title,
      service: plan.service,
      ticket: plan.ticket,
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

  const preview = useMemo(() => ({ __html: marked.parse(editorContent || file?.content || '') as string }), [editorContent, file]);

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
      const updated = await api.saveFile(planId, targetFile.id, { content, expectedHash: targetFile.hash });
      setFile(updated);
      setSavedContent(content);
      setAutoSaveState('saved');
      autoSaveSettledTimerRef.current = window.setTimeout(() => setAutoSaveState('idle'), 1400);
      clearTimeoutRef(autoSaveRefreshTimerRef);
      autoSaveRefreshTimerRef.current = window.setTimeout(() => {
        api.diff(planId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
        if (plan) void loadGitStatus(plan.repositoryId);
      }, 600);
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File save failed');
      setAutoSaveState('error');
      return false;
    } finally {
      setSavingFile(false);
    }
  };

  const loadDrawerFile = async (fileId: string) => {
    try {
      const content = await api.file(planId, fileId);
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

  const loadGitStatus = async (repositoryId: string) => {
    try {
      setGitStatus(await api.gitStatus(repositoryId));
    } catch {
      setGitStatus(null);
    }
  };

  const saveMetadata = async () => {
    if (!plan) return;
    setSavingMetadata(true);
    setError('');
    try {
      const result = await api.saveMetadata(planId, metadataDraft);
      setPlan(result.plan);
      await loadGitStatus(plan.repositoryId);
      await onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Metadata save failed');
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
        ? await api.gitFetch(plan.repositoryId)
        : operation === 'pull'
          ? await api.gitPull(plan.repositoryId, { confirm })
          : await api.gitPush(plan.repositoryId);
      setGitStatus(result.status);
      if (operation === 'pull') {
        await onChanged();
      }
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : `${operation} failed`);
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
      const result = await api.gitCommit(plan.repositoryId, { message: gitMessage, paths: selectedGitPaths });
      setGitStatus(result.status);
      setGitMessage('');
      setSelectedGitPaths([]);
      await onChanged();
      api.diff(planId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Commit failed');
    } finally {
      setGitBusy('');
    }
  };

  const createAndSwitchBranch = async () => {
    if (!plan || !branchName.trim()) return;
    setGitBusy('branch');
    setError('');
    try {
      const result = await api.createBranch(plan.repositoryId, { name: branchName.trim(), checkout: true });
      setGitStatus(result.status);
      setBranchName('');
      await onChanged();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Branch operation failed');
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
      <button className="drawer-scrim" type="button" aria-label="Close preview" onClick={closeDrawer} />
      <aside className={compact ? 'plan-drawer compact' : 'plan-drawer'} style={drawerStyle} aria-label="Plan preview">
        <button className="drawer-resize-handle" type="button" aria-label="Resize preview panel" onPointerDown={startResize}>
          <GripVertical size={16} />
        </button>
        <header className="plan-drawer-header">
          <div>
            <span className="drawer-kicker">{plan?.ticket ?? 'Loading'}</span>
            <h2>{plan?.title ?? 'Loading plan'}</h2>
            <p>{plan ? `${plan.service} / ${plan.branch}` : ''}</p>
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
                <select value={file?.id ?? ''} onChange={(event) => void selectDrawerFile(event.target.value)} disabled={fileOptions.length === 0}>
                  {fileOptions.length === 0 && <option value="">No files available</option>}
                  {fileOptions.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
                </select>
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
              {tab === 'preview' && (file ? <article className="drawer-markdown" dangerouslySetInnerHTML={preview} /> : <div className="drawer-empty">No readable file selected.</div>)}
              {tab === 'raw' && (
                <textarea
                  className="drawer-raw drawer-raw-editor"
                  value={file ? editorContent : 'No readable file selected.'}
                  onChange={(event) => setEditorContent(event.target.value)}
                  disabled={!file}
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
            </div>
            <div className="drawer-work-item-content">
              {sideTab === 'info' && (
                <>
                  <dl>
                    <dt>Repository</dt><dd>{plan?.repositoryName ?? '-'}</dd>
                    <dt>Service</dt><dd>{plan?.service ?? '-'}</dd>
                    <dt>Branch</dt><dd>{plan?.branch ?? '-'}</dd>
                    <dt>Status</dt><dd>{plan?.status ? <DrawerStatusBadge status={plan.status} /> : '-'}</dd>
                    <dt>Source</dt><dd>{metadataSourceLabel(plan?.metadataSource)}</dd>
                    <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                    <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
                  </dl>
                  {plan?.metadataSource !== 'docs' && (
                    <div className="metadata-form drawer-metadata-form">
                      <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
                      <label>Service<input value={metadataDraft.service ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, service: event.target.value }))} /></label>
                      <label>Ticket<input value={metadataDraft.ticket ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, ticket: event.target.value }))} /></label>
                      <label>Status<select value={metadataDraft.status ?? 'draft'} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, status: event.target.value as PlanStatus }))}>
                        {editableStatusOrder.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
                      </select></label>
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
                </section>
                ) : (
                  <div className="metadata-callout">
                    <strong>Git status unavailable</strong>
                    <span>Open details to run Git actions.</span>
                  </div>
                )
              )}
            </div>
          </aside>
        </div>
      </aside>
    </>
  );
}

export function filterPlans(plans: PlanSummary[], filters: Filters, text: string, repository?: RepositoryConfig): PlanSummary[] {
  const query = text.trim().toLowerCase();
  return plans.filter((plan) => {
    if (filters.sources.length > 0 && !filters.sources.includes(sourceRoot(plan, repository))) return false;
    if (filters.statuses.length > 0 && !filters.statuses.includes(plan.status)) return false;
    if (filters.branches.length > 0 && !filters.branches.includes(plan.branch)) return false;
    const author = plan.author || plan.owner || 'Unknown';
    if (filters.authors.length > 0 && !filters.authors.includes(author)) return false;
    if (query && !planSearchText(plan).includes(query)) return false;
    return true;
  });
}

function sourceFacetOptions(plans: PlanSummary[], repository?: RepositoryConfig): FacetOption[] {
  const roots = new Set(plans.map((plan) => sourceRoot(plan, repository)).filter(Boolean));
  return Array.from(roots)
    .sort((a, b) => a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' }))
    .map((root) => ({ value: root, label: root }));
}

function sourceLabel(plan: PlanSummary, repository?: RepositoryConfig): string {
  return sourceRoot(plan, repository);
}

function sourceRoot(plan: PlanSummary, repository?: RepositoryConfig): string {
  const root = plan.planRoot || '';
  const directories = repository?.planDirectories ?? [];
  const matched = directories
    .filter((directory) => root === directory || root.startsWith(`${directory}/`))
    .sort((a, b) => b.length - a.length)[0];
  if (matched) return matched;
  return root.split('/').filter(Boolean)[0] ?? '';
}

function planSearchText(plan: PlanSummary): string {
  return [
    plan.title,
    plan.ticket,
    plan.service,
    plan.branch,
    plan.repositoryName,
    plan.author,
    plan.owner,
    plan.description,
    plan.metadataSource,
    ...plan.tags
  ].filter(Boolean).join(' ').toLowerCase();
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
  return source === 'docs' ? 'Docs' : 'Plan';
}

function DrawerStatusBadge({ status }: { status: PlanStatus }) {
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

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}
