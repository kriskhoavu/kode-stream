import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, MouseEvent, PointerEvent as ReactPointerEvent } from 'react';
import { ChevronDown, Code2, FileText, Filter, FolderGit2, GripVertical, Info, KanbanSquare, LockKeyhole, RefreshCw, RotateCw, Search, X } from 'lucide-react';
import { marked } from 'marked';
import { api, statusLabels, statusOrder } from '../lib/api';
import type { FileContent, FileNode, PlanDetail, PlanStatus, PlanSummary, RepositoryConfig } from '../lib/types';

type FilterKey = 'sources' | 'statuses' | 'branches' | 'authors';

type Filters = Record<FilterKey, string[]>;

type FacetOption = { value: string; label: string };

type DrawerTab = 'preview' | 'raw' | 'diff';

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
        <span className="readonly-badge"><LockKeyhole size={15} /> Read-only</span>
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
            <button className="new-plan-column-button" type="button" onClick={() => {
              setNewPlanDraft((draft) => ({ ...draft, status: column, planDirectory: repository?.planDirectories[0] ?? '' }));
              setNewPlanOpen(true);
            }}>+ New plan</button>
          </div>
        ))}
      </div>
      {drawerPlanId && (
        <PlanPreviewDrawer
          planId={drawerPlanId}
          refreshKey={refreshKey}
          onClose={() => setDrawerPlanId('')}
          onOpenFull={() => onOpenPlan(drawerPlanId)}
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
                {statusOrder.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
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
  const docs = source === 'docs';
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
        {source && <span className={docs ? 'source-badge docs' : 'source-badge'}>{source}</span>}
      </div>
      <span>{plan.service} / {plan.branch}</span>
      <button type="button" className="plan-card-link plan-card-ticket" onClick={navigate}>{plan.ticket}</button>
      <p>{plan.description || plan.ticket}</p>
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {plan.tags.length > 0 && <div className="tags">{plan.tags.slice(0, 3).map((tag) => <span key={tag}>{tag}</span>)}</div>}
      <select className="status-move-select" value={plan.status} onClick={(event) => event.stopPropagation()} onChange={(event) => onMove(event.target.value as PlanStatus)} aria-label="Move plan status">
        {statusOrder.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
      </select>
    </article>
  );
}

function PlanPreviewDrawer({ planId, refreshKey, onClose, onOpenFull }: { planId: string; refreshKey: number; onClose: () => void; onOpenFull: () => void }) {
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<DrawerTab>('preview');
  const [error, setError] = useState('');
  const [width, setWidth] = useState(560);
  const drawerStyle = { '--drawer-width': `${width}px` } as CSSProperties & Record<'--drawer-width', string>;

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [onClose]);

  useEffect(() => {
    let active = true;
    setPlan(null);
    setFile(null);
    setFiles([]);
    setDiff('');
    setError('');
    api.plan(planId).then((payload) => {
      if (active) setPlan(payload);
    }).catch((err: Error) => {
      if (active) setError(err.message);
    });
    api.files(planId).then(async (tree) => {
      if (!active) return;
      setFiles(tree);
      const first = firstFile(tree);
      if (first) {
        try {
          const content = await api.file(planId, first.id);
          if (active) setFile(content);
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
    };
  }, [planId, refreshKey]);

  const preview = useMemo(() => ({ __html: marked.parse(file?.content ?? '') as string }), [file]);

  const startResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startingWidth = width;

    const onPointerMove = (moveEvent: PointerEvent) => {
      const delta = startX - moveEvent.clientX;
      setWidth(Math.min(920, Math.max(420, startingWidth + delta)));
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
      <button className="drawer-scrim" type="button" aria-label="Close preview" onClick={onClose} />
      <aside className="plan-drawer" style={drawerStyle} aria-label="Plan preview">
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
            <button type="button" className="secondary" onClick={onOpenFull}>Open details</button>
            <button type="button" className="icon-button" aria-label="Close preview" onClick={onClose}><X size={16} /></button>
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
              <div className="drawer-tabs">
                <button className={tab === 'preview' ? 'active' : ''} type="button" onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
                <button className={tab === 'raw' ? 'active' : ''} type="button" onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
                <button className={tab === 'diff' ? 'active' : ''} type="button" onClick={() => setTab('diff')}><RefreshCw size={15} /> Diff</button>
              </div>
              {tab === 'preview' && (file ? <article className="drawer-markdown" dangerouslySetInnerHTML={preview} /> : <div className="drawer-empty">No readable file selected.</div>)}
              {tab === 'raw' && <pre className="drawer-raw">{file?.content ?? 'No readable file selected.'}</pre>}
              {tab === 'diff' && <pre className="drawer-raw">{diff || 'Loading diff...'}</pre>}
            </section>
          </section>
          <aside className="drawer-meta">
            <h3><Info size={15} /> Details</h3>
            <dl>
              <dt>Repository</dt><dd>{plan?.repositoryName ?? '-'}</dd>
              <dt>Status</dt><dd>{plan?.status ?? '-'}</dd>
              <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
              <dt>Source</dt><dd>{metadataSourceLabel(plan?.metadataSource)}</dd>
              <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
            </dl>
            {(plan?.tags?.length ?? 0) > 0 && <div className="tags">{plan?.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
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

function metadataSourceLabel(source?: string): string {
  return source === 'docs' ? 'Docs' : 'Plan';
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}
