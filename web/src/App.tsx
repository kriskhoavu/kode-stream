import { useEffect, useRef, useState } from 'react';
import { Bell, ChevronDown, GitBranch, KanbanSquare, ListChecks, Moon, Plus, Search, Sun, Boxes, FolderGit2 } from 'lucide-react';
import type { WorkspaceConfig } from './lib/types';
import { useAppState } from './app/useAppState';
export type { Route } from './app/router';
export { routeFromLocation } from './app/router';
import { BranchesPage } from './pages/BranchesPage';
import { KanbanPage } from './pages/KanbanPage';
import { ItemsPage } from './pages/ItemsPage';
import { ItemWorkspacePage } from './pages/ItemWorkspacePage';
import { WorkspacesPage } from './pages/WorkspacesPage';
import { ActivityPanel } from './components/ReliabilityPanels';
import { SearchDialog } from './components/SearchDialog';
import { useQuickSwitcher } from './features/search/hooks';
import { labels } from './lib/vocabulary';

export function App() {
  const {
    route,
    theme,
    setTheme,
    workspaces,
    activeRepo,
    contentRefreshKey,
    showStaleNotice,
    setShowStaleNotice,
    navigate,
    selectWorkspace: selectWorkspaceState,
    refreshAppData,
    refreshAppStateOnly,
    lastSync
  } = useAppState();
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [activityOpen, setActivityOpen] = useState(false);
  const quickSwitcher = useQuickSwitcher();
  const workspaceMenuRef = useRef<HTMLDivElement | null>(null);

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
    selectWorkspaceState(repo);
    setWorkspaceMenuOpen(false);
  };

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
          <button className="search-trigger" type="button" onClick={() => quickSwitcher.setOpen(true)} aria-label="Search">
            <Search size={16} /><span>Search</span>
          </button>
          <button className="icon-button topbar-icon" type="button" aria-label="Recent activity" aria-expanded={activityOpen} onClick={() => setActivityOpen((open) => !open)}>
            <Bell size={17} />
          </button>
          <button className="icon-button topbar-icon" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} aria-label="Toggle theme">
            {theme === 'light' ? <Moon size={17} /> : <Sun size={17} />}
          </button>
          <span className="user-avatar" aria-label="Current user">K</span>
        </div>
      </header>

      {activityOpen && <ActivityPanel workspaceId={activeRepo?.id} onClose={() => setActivityOpen(false)} />}
      {quickSwitcher.open && <SearchDialog workspaceId={activeRepo?.id} onClose={quickSwitcher.close} onNavigate={(path) => {
        history.pushState(null, '', path);
        window.dispatchEvent(new PopStateEvent('popstate'));
      }} />}

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
