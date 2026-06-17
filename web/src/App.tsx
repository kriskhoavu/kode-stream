import { useEffect, useMemo, useRef, useState } from 'react';
import { Bell, ChevronDown, GitBranch, KanbanSquare, ListChecks, Moon, Plus, Sun, Boxes, FolderGit2 } from 'lucide-react';
import { api } from './lib/api';
import type { RepositoryConfig } from './lib/types';
import { BranchesPage } from './pages/BranchesPage';
import { KanbanPage } from './pages/KanbanPage';
import { PlansPage } from './pages/PlansPage';
import { PlanWorkspacePage } from './pages/PlanWorkspacePage';
import { RepositoriesPage } from './pages/RepositoriesPage';

type Route = { name: 'kanban' } | { name: 'plans' } | { name: 'branches' } | { name: 'repositories' } | { name: 'workspace'; planId: string };

const contentVersionStorageKey = 'planManagerContentVersion';

function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/plans/')) {
    return { name: 'workspace', planId: decodeURIComponent(path.split('/')[2] ?? '') };
  }
  if (path === '/plans') {
    return { name: 'plans' };
  }
  if (path.startsWith('/branches')) {
    return { name: 'branches' };
  }
  if (path.startsWith('/repositories')) {
    return { name: 'repositories' };
  }
  return { name: 'kanban' };
}

export function App() {
  const [route, setRoute] = useState<Route>(routeFromLocation);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => (localStorage.getItem('theme') as 'light' | 'dark') || 'light');
  const [repositories, setRepositories] = useState<RepositoryConfig[]>([]);
  const [activeRepositoryId, setActiveRepositoryId] = useState(() => localStorage.getItem('activeRepositoryId') ?? '');
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
    const path = next.name === 'workspace' ? `/plans/${encodeURIComponent(next.planId)}` : next.name === 'repositories' ? '/repositories' : next.name === 'plans' ? '/plans' : next.name === 'branches' ? '/branches' : '/kanban';
    history.pushState(null, '', path);
    setRoute(next);
  };

  const refreshRepositories = () => api.repositories().then(setRepositories).catch(() => setRepositories([]));
  const markStateCurrent = async (broadcast = false) => {
    const state = await api.state();
    setStateVersion(state.version);
    setShowStaleNotice(false);
    if (broadcast) {
      localStorage.setItem(contentVersionStorageKey, `${state.version}:${Date.now()}`);
    }
  };
  const refreshAppData = async (broadcast = false) => {
    await refreshRepositories();
    setContentRefreshKey((key) => key + 1);
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
    if (repositories.length === 0) {
      setActiveRepositoryId('');
      localStorage.removeItem('activeRepositoryId');
      return;
    }
    if (!repositories.some((repo) => repo.id === activeRepositoryId)) {
      const nextId = repositories[0].id;
      setActiveRepositoryId(nextId);
      localStorage.setItem('activeRepositoryId', nextId);
    }
  }, [activeRepositoryId, repositories]);

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

  const selectWorkspace = (repo: RepositoryConfig) => {
    setActiveRepositoryId(repo.id);
    localStorage.setItem('activeRepositoryId', repo.id);
    setWorkspaceMenuOpen(false);
    navigate({ name: 'kanban' });
  };

  const activeRepo = repositories.find((repo) => repo.id === activeRepositoryId) ?? repositories[0];
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
          <NavButton active={route.name === 'plans'} onClick={() => navigate({ name: 'plans' })} icon={<ListChecks size={18} />} label="Plans" />
          <NavButton active={route.name === 'branches'} onClick={() => navigate({ name: 'branches' })} icon={<GitBranch size={18} />} label="Branches" />
          <NavButton active={route.name === 'repositories'} onClick={() => navigate({ name: 'repositories' })} icon={<FolderGit2 size={18} />} label="Repositories" />
        </div>
        <div className="workspace-list">
          <span className="workspace-list-label">Repositories</span>
          {repositories.map((repo) => (
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
          {repositories.length === 0 && <span className="workspace-empty">No repositories registered</span>}
        </div>
        <button className="add-repository-button" type="button" onClick={() => navigate({ name: 'repositories' })}>
          <Plus size={16} />
          Add Repository
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
                <strong>Repositories</strong>
                <span>{repositories.length} workspace{repositories.length === 1 ? '' : 's'}</span>
              </div>
              <div className="workspace-menu-list">
                {repositories.map((repo) => (
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
                      <small>{repo.baselineBranch} · {repo.planDirectories.join(', ') || 'plans'}</small>
                    </span>
                  </button>
                ))}
                {repositories.length === 0 && <span className="workspace-menu-empty">No repositories registered</span>}
              </div>
              <button className="workspace-menu-add" type="button" onClick={() => {
                setWorkspaceMenuOpen(false);
                navigate({ name: 'repositories' });
              }}>
                <Plus size={15} />
                Add or manage repositories
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
        {route.name === 'kanban' && <KanbanPage repository={activeRepo} refreshKey={contentRefreshKey} onOpenPlan={(planId) => navigate({ name: 'workspace', planId })} onRepositoriesChanged={() => refreshAppData(true)} />}
        {route.name === 'plans' && <PlansPage repository={activeRepo} refreshKey={contentRefreshKey} onOpenPlan={(planId) => navigate({ name: 'workspace', planId })} />}
        {route.name === 'branches' && <BranchesPage repository={activeRepo} refreshKey={contentRefreshKey} onOpenBranch={(branch) => navigate({ name: 'kanban' })} />}
        {route.name === 'workspace' && <PlanWorkspacePage planId={route.planId} refreshKey={contentRefreshKey} onBack={() => navigate({ name: 'kanban' })} onContentChanged={() => refreshAppData(true)} />}
        {route.name === 'repositories' && <RepositoriesPage repositories={repositories} onChanged={() => refreshAppData(true)} />}
      </main>

      {showStaleNotice && (
        <div className="stale-notice" role="status" aria-live="polite">
          <strong>Content may have changed</strong>
          <span>Refresh the current view to load the latest plans and repositories.</span>
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
        <button className={route.name === 'plans' ? 'active' : ''} onClick={() => navigate({ name: 'plans' })}><ListChecks size={18} />Plans</button>
        <button className={route.name === 'branches' ? 'active' : ''} onClick={() => navigate({ name: 'branches' })}><GitBranch size={18} />Branches</button>
        <button className={route.name === 'repositories' ? 'active' : ''} onClick={() => navigate({ name: 'repositories' })}><FolderGit2 size={18} />Repos</button>
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
