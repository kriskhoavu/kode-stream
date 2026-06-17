import { useEffect, useMemo, useState } from 'react';
import { GitBranch, KanbanSquare, ListChecks, Moon, Sun, Boxes, FolderGit2 } from 'lucide-react';
import { api } from './lib/api';
import type { RepositoryConfig } from './lib/types';
import { KanbanPage } from './pages/KanbanPage';
import { PlanWorkspacePage } from './pages/PlanWorkspacePage';
import { RepositoriesPage } from './pages/RepositoriesPage';

type Route = { name: 'kanban' } | { name: 'repositories' } | { name: 'workspace'; planId: string };

function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/plans/')) {
    return { name: 'workspace', planId: decodeURIComponent(path.split('/')[2] ?? '') };
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
    const path = next.name === 'workspace' ? `/plans/${encodeURIComponent(next.planId)}` : next.name === 'repositories' ? '/repositories' : '/kanban';
    history.pushState(null, '', path);
    setRoute(next);
  };

  const refreshRepositories = () => api.repositories().then(setRepositories).catch(() => setRepositories([]));
  useEffect(() => {
    refreshRepositories();
  }, []);

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

  const selectWorkspace = (repo: RepositoryConfig) => {
    setActiveRepositoryId(repo.id);
    localStorage.setItem('activeRepositoryId', repo.id);
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
      <header className="topbar">
        <button className="brand" onClick={() => navigate({ name: 'kanban' })} aria-label="Plan Manager home">
          <Boxes size={20} />
          <span>Plan Manager</span>
        </button>
        <div className="workspace-title">
          <FolderGit2 size={16} />
          <span>{activeRepo?.name ?? 'No workspace selected'}</span>
        </div>
        <div className="topbar-actions">
          <span className="sync-dot">Last scan: {lastSync}</span>
          <button className="theme-toggle" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} aria-label="Toggle theme">
            {theme === 'light' ? <Moon size={17} /> : <Sun size={17} />}
            <span>{theme === 'light' ? 'Dark' : 'Light'}</span>
          </button>
        </div>
      </header>

      <aside className="left-nav">
        <NavButton active={route.name === 'kanban'} onClick={() => navigate({ name: 'kanban' })} icon={<KanbanSquare size={18} />} label="Kanban" />
        <NavButton active={false} onClick={() => navigate({ name: 'kanban' })} icon={<ListChecks size={18} />} label="Plans" />
        <NavButton active={false} onClick={() => navigate({ name: 'kanban' })} icon={<GitBranch size={18} />} label="Branches" />
        <NavButton active={route.name === 'repositories'} onClick={() => navigate({ name: 'repositories' })} icon={<FolderGit2 size={18} />} label="Repositories" />
        <div className="workspace-list">
          <span className="workspace-list-label">Workspaces</span>
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
        <div className="repo-status">
          <span className="repo-status-label">Read-only</span>
          <span>{activeRepo ? `${activeRepo.baselineBranch} · ${repositories.length} workspace${repositories.length === 1 ? '' : 's'}` : 'Use Repositories to add one'}</span>
        </div>
      </aside>

      <main className="main-content">
        {route.name === 'kanban' && <KanbanPage repository={activeRepo} onOpenPlan={(planId) => navigate({ name: 'workspace', planId })} onRepositoriesChanged={refreshRepositories} />}
        {route.name === 'workspace' && <PlanWorkspacePage planId={route.planId} onBack={() => navigate({ name: 'kanban' })} />}
        {route.name === 'repositories' && <RepositoriesPage repositories={repositories} onChanged={refreshRepositories} />}
      </main>

      <nav className="bottom-nav">
        <button className={route.name === 'kanban' ? 'active' : ''} onClick={() => navigate({ name: 'kanban' })}><KanbanSquare size={18} />Kanban</button>
        <button onClick={() => navigate({ name: 'kanban' })}><ListChecks size={18} />Plans</button>
        <button onClick={() => navigate({ name: 'kanban' })}><GitBranch size={18} />Branches</button>
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
