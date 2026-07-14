import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { Bell, BookOpen, ChevronDown, KanbanSquare as WorkstreamIcon, Moon, Plus, Search, Sun, Boxes, FolderGit2, Settings } from 'lucide-react';
import type { WorkspaceConfig } from './lib/types';
import { useAppState } from './app/useAppState';
export type { Route } from './app/router';
export { routeFromLocation } from './app/router';
import { WorkstreamPage } from './pages/WorkstreamPage';
import { ItemWorkspacePage } from './pages/ItemWorkspacePage';
import { WorkspacesPage } from './pages/WorkspacesPage';
import { SettingsPage } from './pages/SettingsPage';
import { api } from './lib/api';
import { ActivityPanel } from './components/ReliabilityPanels';
import { SearchDialog } from './components/SearchDialog';
import { useQuickSwitcher } from './features/search/hooks';
import { useAppSettings } from './features/settings/appSettings';
import { EmbeddedTerminalDock } from './features/ai-session/EmbeddedTerminalDock';

const KnowledgePage = lazy(() => import('./pages/KnowledgePage').then((module) => ({ default: module.KnowledgePage })));

export function App() {
  const {
    route,
    theme,
    setTheme,
    workspaces,
    activeRepo,
    runtimeContext,
    contentRefreshKey,
    navigate,
    selectWorkspace: selectWorkspaceState,
    refreshAppData,
    refreshAppStateOnly,
    lastSync
  } = useAppState();
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const [activityOpen, setActivityOpen] = useState(false);
  const [appSettings, setAppSettings] = useAppSettings();
  const quickSwitcher = useQuickSwitcher();
  const workspaceMenuRef = useRef<HTMLDivElement | null>(null);
  const profileMenuRef = useRef<HTMLDivElement | null>(null);
  const cloudUserLabel = runtimeContext.user?.name || runtimeContext.user?.email || runtimeContext.user?.id || 'Cloud user';
  const modeLabel = runtimeContext.mode === 'cloud' ? `Cloud · ${runtimeContext.role ?? 'viewer'} · ${runtimeContext.agent.status}` : 'Local';

  useEffect(() => {
    if (!workspaceMenuOpen && !profileMenuOpen) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      const target = event.target as Node;
      if (workspaceMenuRef.current && !workspaceMenuRef.current.contains(target)) {
        setWorkspaceMenuOpen(false);
      }
      if (profileMenuRef.current && !profileMenuRef.current.contains(target)) {
        setProfileMenuOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setWorkspaceMenuOpen(false);
        setProfileMenuOpen(false);
      }
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick);
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [workspaceMenuOpen, profileMenuOpen]);

  const selectWorkspace = (repo: WorkspaceConfig) => {
    selectWorkspaceState(repo);
    setWorkspaceMenuOpen(false);
  };

  return (
    <div className="app-shell">
      <aside className="left-nav">
        <button className="brand" onClick={() => navigate({ name: 'workstream' })} aria-label="Kode Stream home">
          <Boxes size={20} />
          <span>Kode Stream</span>
        </button>
        <div className="nav-section">
          <span className="nav-section-label">Workspace</span>
          <NavButton active={route.name === 'workstream'} onClick={() => navigate({ name: 'workstream' })} icon={<WorkstreamIcon size={18} />} label="Workstream" />
          <NavButton active={route.name === 'knowledge'} onClick={() => navigate({ name: 'knowledge' })} icon={<BookOpen size={18} />} label="Knowledge" />
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
            <WorkstreamIcon size={16} />
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
                      <small>{repo.location === 'cloud_agent' ? 'Cloud Agent' : 'Local'} · {repo.baselineBranch} · {repo.sources.join(', ') || 'plans'}</small>
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
          <span className="runtime-mode-label">{modeLabel}</span>
          <button className="icon-button topbar-icon" type="button" aria-label="Recent activity" aria-expanded={activityOpen} onClick={() => setActivityOpen((open) => !open)}>
            <Bell size={17} />
          </button>
          <button className="icon-button topbar-icon" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} aria-label="Toggle theme">
            {theme === 'light' ? <Moon size={17} /> : <Sun size={17} />}
          </button>
          <div className="profile-menu-wrapper" ref={profileMenuRef}>
            <button
              className="user-avatar profile-trigger"
              type="button"
              aria-label="Current user"
              aria-haspopup="menu"
              aria-expanded={profileMenuOpen}
              onClick={() => {
                setWorkspaceMenuOpen(false);
                setProfileMenuOpen((open) => !open);
              }}
            >
              K
            </button>
            {profileMenuOpen && (
              <div className="profile-menu" role="menu" aria-label="User menu">
                <div className="profile-menu-header">
                  <strong>{runtimeContext.mode === 'cloud' ? cloudUserLabel[0]?.toUpperCase() ?? 'C' : 'K'}</strong>
                  <span>{runtimeContext.mode === 'cloud' ? cloudUserLabel : 'Signed in locally'}</span>
                </div>
                <button
                  className={route.name === 'settings' ? 'profile-menu-item active' : 'profile-menu-item'}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    setProfileMenuOpen(false);
                    navigate({ name: 'settings' });
                  }}
                >
                  <Settings size={15} />
                  <span>Settings</span>
                </button>
                {runtimeContext.mode === 'cloud' && <button
                  className="profile-menu-item"
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    setProfileMenuOpen(false);
                    void api.logout().then(() => refreshAppData());
                  }}
                >
                  <Settings size={15} />
                  <span>Logout</span>
                </button>}
              </div>
            )}
          </div>
        </div>
      </header>

      {activityOpen && <ActivityPanel workspaceId={activeRepo?.id} onClose={() => setActivityOpen(false)} />}
      {quickSwitcher.open && <SearchDialog workspaceId={activeRepo?.id} onClose={quickSwitcher.close} onNavigate={(path) => {
        history.pushState(null, '', path);
        window.dispatchEvent(new PopStateEvent('popstate'));
      }} />}

      <main className="main-content">
        {route.name === 'workstream' && (
          <WorkstreamPage
            workspace={activeRepo}
            refreshKey={contentRefreshKey}
            visibleStatuses={appSettings.visibleWorkstreamStatuses}
            focusedItemId={route.focusedItemId}
            onOpenPlan={(itemId) => navigate({ name: 'item', itemId })}
            onWorkspacesChanged={() => refreshAppData()}
            onOpenWorkspaces={() => navigate({ name: 'workspaces' })}
          />
        )}
        {route.name === 'item' && <ItemWorkspacePage itemId={route.itemId} refreshKey={contentRefreshKey} workspaces={workspaces} onBack={() => navigate({ name: 'workstream' })} onOpenItem={(nextItemId) => navigate({ name: 'item', itemId: nextItemId })} onContentChanged={() => refreshAppStateOnly()} />}
        {route.name === 'workspaces' && <WorkspacesPage workspaces={workspaces} runtimeContext={runtimeContext} onChanged={() => refreshAppData()} />}
        {route.name === 'settings' && <SettingsPage settings={appSettings} onChange={setAppSettings} />}
        {route.name === 'knowledge' && <Suspense fallback={<section className="empty-state">Loading Knowledge...</section>}><KnowledgePage workspaces={workspaces} location={route.location} onLocationChange={(location) => navigate({ name: 'knowledge', location })} /></Suspense>}
      </main>

      <nav className="bottom-nav">
        <button className={route.name === 'workstream' ? 'active' : ''} onClick={() => navigate({ name: 'workstream' })}><WorkstreamIcon size={18} />Workstream</button>
        <button className={route.name === 'knowledge' ? 'active' : ''} onClick={() => navigate({ name: 'knowledge' })}><BookOpen size={18} />Knowledge</button>
        <button className={route.name === 'workspaces' ? 'active' : ''} onClick={() => navigate({ name: 'workspaces' })}><FolderGit2 size={18} />Workspaces</button>
        <button className={route.name === 'settings' ? 'active' : ''} onClick={() => navigate({ name: 'settings' })}><Settings size={18} />Settings</button>
      </nav>
		<EmbeddedTerminalDock workspaces={workspaces} />
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
