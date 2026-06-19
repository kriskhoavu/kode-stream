import { useEffect, useMemo, useRef, useState } from 'react';
import { Bell, ChevronDown, GitBranch, KanbanSquare, ListChecks, Moon, Plus, Sun, Boxes, FolderGit2 } from 'lucide-react';
import { api } from './lib/api';
import type { WorkspaceConfig } from './lib/types';
import { BranchesPage } from './pages/BranchesPage';
import { KanbanPage } from './pages/KanbanPage';
import { ItemsPage } from './pages/ItemsPage';
import { ItemWorkspacePage } from './pages/ItemWorkspacePage';
import { WorkspacesPage } from './pages/WorkspacesPage';
import { labels } from './lib/vocabulary';

export type Route = { name: 'kanban' } | { name: 'items' } | { name: 'branches' } | { name: 'workspaces' } | { name: 'workspace'; itemId: string };

const contentVersionStorageKey = 'itemManagerContentVersion';

export function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/items/')) {
    return { name: 'workspace', itemId: decodeURIComponent(path.split('/')[2] ?? '') };
  }
  if (path === '/items') {
    return { name: 'items' };
  }
  if (path.startsWith('/branches')) {
    return { name: 'branches' };
  }
  if (path.startsWith('/workspaces')) {
    return { name: 'workspaces' };
  }
  return { name: 'kanban' };
}

export function App() {
  const [route, setRoute] = useState<Route>(routeFromLocation);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => (localStorage.getItem('theme') as 'light' | 'dark') || 'light');
  const [workspaces, setWorkspaces] = useState<WorkspaceConfig[]>([]);
  const [activeWorkspaceId, setActiveWorkspaceId] = useState(() => localStorage.getItem('activeWorkspaceId') ?? '');
  const [contentRefreshKey, setContentRefreshKey] = useState(0);
  const [stateVersion, setStateVersion] = useState('');
  const [showStaleNotice, setShowStaleNotice] = useState(false);
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const workspaceMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('theme', theme);
  }, [theme]);

  useEffect(() => {
    const onPop = () => setRoute(routeFromLocation());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const navigate = (next: Route) => {
    const path = next.name === 'workspace' ? `/items/${encodeURIComponent(next.itemId)}` : next.name === 'workspaces' ? '/workspaces' : next.name === 'items' ? '/items' : next.name === 'branches' ? '/branches' : '/kanban';
    history.pushState(null, '', path);
    setRoute(next);
  };

  const refreshWorkspaces = () => api.workspaces().then(setWorkspaces).catch(() => setWorkspaces([]));
  const markStateCurrent = async (broadcast = false) => {
    const state = await api.state();
    setStateVersion(state.version);
    setShowStaleNotice(false);
    if (broadcast) {
      localStorage.setItem(contentVersionStorageKey, `${state.version}:${Date.now()}`);
    }
  };
  const refreshAppData = async (broadcast = false) => {
    await refreshWorkspaces();
    setContentRefreshKey((key) => key + 1);
    await markStateCurrent(broadcast);
  };
  const refreshAppStateOnly = async (broadcast = false) => {
    await refreshWorkspaces();
    await markStateCurrent(broadcast);
  };

  useEffect(() => {
    void refreshAppData();
  }, []);

  useEffect(() => {
    const checkState = async () => {
      if (document.hidden) return;
      try {
        const state = await api.state();
        if (!stateVersion) {
          setStateVersion(state.version);
        } else if (state.version !== stateVersion) {
          setShowStaleNotice(true);
        }
      } catch {
        // The regular page APIs already surface request errors where needed.
      }
    };
    const interval = window.setInterval(checkState, 30000);
    const onVisibilityChange = () => {
      if (!document.hidden) void checkState();
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      window.clearInterval(interval);
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, [stateVersion]);

  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key !== contentVersionStorageKey || !event.newValue) return;
      const version = event.newValue.split(':')[0];
      if (stateVersion && version !== stateVersion) {
        setShowStaleNotice(true);
      }
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, [stateVersion]);

  useEffect(() => {
    if (workspaces.length === 0) {
      setActiveWorkspaceId('');
      localStorage.removeItem('activeWorkspaceId');
      return;
    }
    if (!workspaces.some((repo) => repo.id === activeWorkspaceId)) {
      const nextId = workspaces[0].id;
      setActiveWorkspaceId(nextId);
      localStorage.setItem('activeWorkspaceId', nextId);
    }
  }, [activeWorkspaceId, workspaces]);

  useEffect(() => {
    if (!workspaceMenuOpen) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (workspaceMenuRef.current && !workspaceMenuRef.current.contains(event.target as Node)) {
        setWorkspaceMenuOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setWorkspaceMenuOpen(false);
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick);
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [workspaceMenuOpen]);

  const selectWorkspace = (repo: WorkspaceConfig) => {
    setActiveWorkspaceId(repo.id);
    localStorage.setItem('activeWorkspaceId', repo.id);
    setWorkspaceMenuOpen(false);
    navigate({ name: 'kanban' });
  };

  const activeRepo = workspaces.find((repo) => repo.id === activeWorkspaceId) ?? workspaces[0];
  const lastSync = useMemo(() => {
    if (!activeRepo?.lastScannedAt) return 'Not scanned';
    return new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' }).format(
      Math.round((new Date(activeRepo.lastScannedAt).getTime() - Date.now()) / 60000),
      'minute'
    );
  }, [activeRepo]);

  return (
    <div className="app-shell">
      <aside className="left-nav">
        <button className="brand" onClick={() => navigate({ name: 'kanban' })} aria-label="Plan Manager home">
          <Boxes size={20} />
          <span>Plan Manager</span>
        </button>
        <div className="nav-section">
          <span className="nav-section-label">Workspace</span>
          <NavButton active={route.name === 'kanban'} onClick={() => navigate({ name: 'kanban' })} icon={<KanbanSquare size={18} />} label="Kanban" />
          <NavButton active={route.name === 'items'} onClick={() => navigate({ name: 'items' })} icon={<ListChecks size={18} />} label={labels.items} />
          <NavButton active={route.name === 'branches'} onClick={() => navigate({ name: 'branches' })} icon={<GitBranch size={18} />} label="Branches" />
          <NavButton active={route.name === 'workspaces'} onClick={() => navigate({ name: 'workspaces' })} icon={<FolderGit2 size={18} />} label={labels.workspaces} />
        </div>
        <div className="workspace-list">
          <span className="workspace-list-label">Workspaces</span>
          {workspaces.map((repo) => (
            <button
              className={repo.id === activeRepo?.id ? 'workspace-button active' : 'workspace-button'}
              key={repo.id}
              onClick={() => selectWorkspace(repo)}
              title={repo.path}
            >
              <FolderGit2 size={16} />
              <span>{repo.name}</span>
            </button>
          ))}
          {workspaces.length === 0 && <span className="workspace-empty">No workspaces registered</span>}
        </div>
        <button className="add-repository-button" type="button" onClick={() => navigate({ name: 'workspaces' })}>
          <Plus size={16} />
          Add Workspace
        </button>
        <div className="repo-status">
          <span className="repo-status-label">Last scan</span>
          <span>{lastSync}</span>
        </div>
      </aside>

      <header className="topbar">
        <div className="workspace-switcher" ref={workspaceMenuRef}>
          <button className="workspace-title" type="button" onClick={() => setWorkspaceMenuOpen((open) => !open)} aria-haspopup="menu" aria-expanded={workspaceMenuOpen}>
            <KanbanSquare size={16} />
            <span>{activeRepo?.name ?? 'No workspace selected'}</span>
            <ChevronDown className={workspaceMenuOpen ? 'workspace-title-chevron open' : 'workspace-title-chevron'} size={15} />
          </button>
          {workspaceMenuOpen && (
            <div className="workspace-menu" role="menu">
              <div className="workspace-menu-header">
                <strong>Workspaces</strong>
                <span>{workspaces.length} workspace{workspaces.length === 1 ? '' : 's'}</span>
              </div>
              <div className="workspace-menu-list">
                {workspaces.map((repo) => (
                  <button
                    className={repo.id === activeRepo?.id ? 'workspace-menu-item active' : 'workspace-menu-item'}
                    key={repo.id}
                    type="button"
                    onClick={() => selectWorkspace(repo)}
                    role="menuitem"
                    title={repo.path}
                  >
                    <FolderGit2 size={16} />
                    <span>
                      <strong>{repo.name}</strong>
                      <small>{repo.baselineBranch} · {repo.sources.join(', ') || 'plans'}</small>
                    </span>
                  </button>
                ))}
                {workspaces.length === 0 && <span className="workspace-menu-empty">No workspaces registered</span>}
              </div>
              <button className="workspace-menu-add" type="button" onClick={() => {
                setWorkspaceMenuOpen(false);
                navigate({ name: 'workspaces' });
              }}>
                <Plus size={15} />
                Add or manage workspaces
              </button>
            </div>
          )}
        </div>
        <div className="topbar-actions">
          <button className="icon-button topbar-icon" type="button" aria-label="Notifications">
            <Bell size={17} />
          </button>
          <button className="icon-button topbar-icon" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} aria-label="Toggle theme">
            {theme === 'light' ? <Moon size={17} /> : <Sun size={17} />}
          </button>
          <span className="user-avatar" aria-label="Current user">K</span>
        </div>
      </header>

      <main className="main-content">
        {route.name === 'kanban' && (
          <KanbanPage
            workspace={activeRepo}
            refreshKey={contentRefreshKey}
            onOpenPlan={(itemId) => navigate({ name: 'workspace', itemId })}
            onWorkspacesChanged={() => refreshAppData(true)}
            onOpenWorkspaces={() => navigate({ name: 'workspaces' })}
          />
        )}
        {route.name === 'items' && <ItemsPage workspace={activeRepo} refreshKey={contentRefreshKey} onOpenPlan={(itemId) => navigate({ name: 'workspace', itemId })} />}
        {route.name === 'branches' && <BranchesPage workspace={activeRepo} refreshKey={contentRefreshKey} onOpenBranch={(branch) => navigate({ name: 'kanban' })} />}
        {route.name === 'workspace' && <ItemWorkspacePage itemId={route.itemId} refreshKey={contentRefreshKey} onBack={() => navigate({ name: 'kanban' })} onContentChanged={() => refreshAppStateOnly(true)} />}
        {route.name === 'workspaces' && <WorkspacesPage workspaces={workspaces} onChanged={() => refreshAppData(true)} />}
      </main>

      {showStaleNotice && (
        <div className="stale-notice" role="status" aria-live="polite">
          <strong>Content may have changed</strong>
          <span>Refresh the current view to load the latest items and workspaces.</span>
          <div>
            <button className="primary" type="button" onClick={() => void refreshAppData()}>
              Refresh
            </button>
            <button className="ghost" type="button" onClick={() => setShowStaleNotice(false)}>
              Dismiss
            </button>
          </div>
        </div>
      )}

      <nav className="bottom-nav">
        <button className={route.name === 'kanban' ? 'active' : ''} onClick={() => navigate({ name: 'kanban' })}><KanbanSquare size={18} />Kanban</button>
        <button className={route.name === 'items' ? 'active' : ''} onClick={() => navigate({ name: 'items' })}><ListChecks size={18} />Items</button>
        <button className={route.name === 'branches' ? 'active' : ''} onClick={() => navigate({ name: 'branches' })}><GitBranch size={18} />Branches</button>
        <button className={route.name === 'workspaces' ? 'active' : ''} onClick={() => navigate({ name: 'workspaces' })}><FolderGit2 size={18} />Workspaces</button>
      </nav>
    </div>
  );
}

function NavButton({ active, icon, label, onClick }: { active: boolean; icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button className={active ? 'nav-button active' : 'nav-button'} onClick={onClick}>
      {icon}
      <span>{label}</span>
    </button>
  );
}
