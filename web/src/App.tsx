import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { Bell, BookOpen, ChevronDown, KanbanSquare as WorkstreamIcon, Moon, Plus, Search, Sun, Boxes, FolderGit2, PanelLeftClose, PanelLeftOpen, Settings, Workflow } from 'lucide-react';
import type { WorkspaceConfig } from './lib/types';
import { useAppState } from './app/useAppState';
export type { Route } from './app/router';
export { routeFromLocation } from './app/router';
import { routeFromPath } from './app/router';
import { WorkstreamPage } from './pages/WorkstreamPage';
import { ItemWorkspacePage } from './pages/ItemWorkspacePage';
import { WorkspacesPage } from './pages/WorkspacesPage';
import { SettingsPage } from './pages/SettingsPage';
import { api, isExtensionSurface, localAPIOrigin } from './shared/api';
import { ActivityPanel } from './components/ReliabilityPanels';
import { HeaderBackdrop, paintsBand } from './components/HeaderBackdrop';
import { SearchDialog } from './components/SearchDialog';
import { useQuickSwitcher } from './features/search/hooks';
import { useAppSettings } from './features/settings/appSettings';
import { EmbeddedTerminalDock } from './features/ai-session/EmbeddedTerminalDock';
import { readStringPreference, writePreference } from './shared/preferences/store';

const KnowledgePage = lazy(() => import('./pages/KnowledgePage').then((module) => ({ default: module.KnowledgePage })));
const CanvasPage = lazy(() => import('./pages/CanvasPage').then((module) => ({ default: module.CanvasPage })));
const BranchReviewPage = lazy(() => import('./pages/BranchReviewPage').then((module) => ({ default: module.BranchReviewPage })));

export function App() {
  const extensionSurface = isExtensionSurface();
  const {
    route,
    theme,
    setTheme,
    workspaces,
    activeRepo,
    runtimeContext,
    dataStatus,
    contentRefreshKey,
    navigate,
    selectWorkspace: selectWorkspaceState,
    refreshAppData,
    refreshAppStateOnly,
    lastSync
  } = useAppState();
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [leftNavCollapsed, setLeftNavCollapsed] = useState(() => readStringPreference('leftNavCollapsed') === 'true');
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const [activityOpen, setActivityOpen] = useState(false);
  const [localAPIStatus, setLocalAPIStatus] = useState<'ready' | 'checking' | 'unavailable'>(extensionSurface ? 'checking' : 'ready');
  const [appSettings, setAppSettings] = useAppSettings();
  const quickSwitcher = useQuickSwitcher();
  const workspaceMenuRef = useRef<HTMLDivElement | null>(null);
  const profileMenuRef = useRef<HTMLDivElement | null>(null);
  const cloudUserLabel = runtimeContext.user?.name || runtimeContext.user?.email || runtimeContext.user?.id || 'Cloud user';
  const modeLabel = dataStatus === 'unavailable' ? 'Runtime unavailable' : runtimeContext.mode === 'cloud' ? `Cloud · ${runtimeContext.role ?? 'viewer'} · ${runtimeContext.agent.status}` : 'Local';

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

  const toggleLeftNav = () => setLeftNavCollapsed((collapsed) => {
    const next = !collapsed;
    writePreference('leftNavCollapsed', String(next));
    return next;
  });

  const checkLocalAPI = useCallback(async () => {
    if (!extensionSurface) return;
    setLocalAPIStatus('checking');
    if (await api.localServerReachable()) {
      setLocalAPIStatus('ready');
      return;
    }
    setLocalAPIStatus('unavailable');
  }, [extensionSurface]);

  useEffect(() => {
    if (extensionSurface) void checkLocalAPI();
  }, [extensionSurface, checkLocalAPI]);

  if (extensionSurface && localAPIStatus !== 'ready') {
    return <LocalServerUnavailable status={localAPIStatus} apiOrigin={localAPIOrigin()} onRetry={() => void checkLocalAPI()} />;
  }

  /*
   * One condition drives both the class and the render. The band class also
   * strips the topbar's fill, blur and border, so a rendered-but-empty band
   * would leave the topbar with no chrome and nothing behind it.
   */
  const showBand = paintsBand(route.name, appSettings.headerBackdrop);
  const shellClasses = ['app-shell', leftNavCollapsed ? 'left-nav-collapsed' : '', showBand ? `header-band header-band-${route.name}` : ''].filter(Boolean).join(' ');

  return (
    <div className={shellClasses}>
      {showBand && <HeaderBackdrop variant={appSettings.headerBackdrop} />}
      <aside className="left-nav">
        <div className="left-nav-brand-row">
          <button className="brand" onClick={() => navigate({ name: 'workstream' })} aria-label="Kode Stream home">
            <Boxes size={20} />
            <span>Kode Stream</span>
          </button>
          <button className="left-nav-toggle" type="button" onClick={toggleLeftNav} aria-label={leftNavCollapsed ? 'Expand navigation' : 'Collapse navigation'} title={leftNavCollapsed ? 'Expand navigation' : 'Collapse navigation'}>{leftNavCollapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />}</button>
        </div>
        <div className="nav-section">
          <span className="nav-section-label">Workspace</span>
          <NavButton active={route.name === 'workstream'} onClick={() => navigate({ name: 'workstream' })} icon={<WorkstreamIcon size={18} />} label="Workstream" />
					{!extensionSurface && <NavButton active={route.name === 'canvas'} onClick={() => navigate({ name: 'canvas', location: { workspaceId: activeRepo?.id } })} icon={<Workflow size={18} />} label="Workbench" />}
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
          <span>Add Workspace</span>
        </button>
        <div className="repo-status">
          <span className="repo-status-label">Last scan</span>
          <span>{lastSync}</span>
        </div>
      </aside>

      <header className="topbar">
        <div className="workspace-switcher" ref={workspaceMenuRef}>
          <button className="workspace-title" type="button" onClick={() => {
            setProfileMenuOpen(false);
            setActivityOpen(false);
            quickSwitcher.close();
            setWorkspaceMenuOpen((open) => !open);
          }} aria-haspopup="menu" aria-expanded={workspaceMenuOpen}>
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
          <button className="search-trigger" type="button" onClick={() => {
            setWorkspaceMenuOpen(false);
            setProfileMenuOpen(false);
            setActivityOpen(false);
            quickSwitcher.setOpen(true);
          }} aria-label="Search">
            <Search size={16} /><span>Search</span>
          </button>
          <span className="runtime-mode-label">{modeLabel}</span>
          <button className="icon-button topbar-icon" type="button" aria-label="Recent activity" aria-expanded={activityOpen} onClick={() => {
            setWorkspaceMenuOpen(false);
            setProfileMenuOpen(false);
            quickSwitcher.close();
            setActivityOpen((open) => !open);
          }}>
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
                setActivityOpen(false);
                quickSwitcher.close();
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
      {quickSwitcher.open && <SearchDialog workspaceId={activeRepo?.id} onClose={quickSwitcher.close} onNavigate={(path) => navigate(routeFromNavigationPath(path))} />}

      <main id="app-main" className="main-content" tabIndex={-1}>
        <div className="sr-only" aria-live="polite">{dataStatus === 'unavailable' ? 'Application data is unavailable. Retry to continue.' : `Navigated to ${route.name}`}</div>
        {dataStatus === 'unavailable' && <section className="empty-state" role="alert"><p>Application data is unavailable. Your existing workspace view is retained.</p><button type="button" className="primary" onClick={() => void refreshAppData()}>Retry</button></section>}
        {route.name === 'workstream' && (
          <WorkstreamPage
            workspace={activeRepo}
            refreshKey={contentRefreshKey}
            visibleStatuses={appSettings.visibleWorkstreamStatuses}
            focusedItemId={route.focusedItemId}
            onOpenPlan={(itemId) => navigate({ name: 'item', itemId })}
            onWorkspacesChanged={async () => { await refreshAppData(); }}
            onOpenWorkspaces={() => navigate({ name: 'workspaces' })}
            onOpenReview={() => navigate({ name: 'review', location: { workspaceId: activeRepo?.id } })}
          />
        )}
        {route.name === 'item' && <ItemWorkspacePage key={route.itemId} itemId={route.itemId} refreshKey={contentRefreshKey} workspaces={workspaces} allowEmbeddedAISessions={!extensionSurface} onBack={() => navigate({ name: 'workstream' })} onOpenItem={(nextItemId) => navigate({ name: 'item', itemId: nextItemId })} onContentChanged={async () => { await refreshAppStateOnly(); }} />}
        {route.name === 'workspaces' && <WorkspacesPage workspaces={workspaces} runtimeContext={runtimeContext} onChanged={async () => { await refreshAppData(); }} />}
        {route.name === 'settings' && <SettingsPage settings={appSettings} onChange={setAppSettings} />}
        {route.name === 'knowledge' && <Suspense fallback={<section className="empty-state">Loading Knowledge...</section>}><KnowledgePage workspaces={workspaces} activeWorkspace={activeRepo} location={route.location} onLocationChange={(location) => navigate({ name: 'knowledge', location })} /></Suspense>}
		{route.name === 'canvas' && !extensionSurface && <Suspense fallback={<section className="empty-state">Loading Canvas...</section>}><CanvasPage workspace={activeRepo} location={route.location} onLocationChange={(location) => navigate({ name: 'canvas', location })} onOpenItem={(itemId) => navigate({ name: 'item', itemId })} onOpenWorkspaces={() => navigate({ name: 'workspaces' })} /></Suspense>}
        {route.name === 'review' && <Suspense fallback={<section className="empty-state">Loading Branch Review...</section>}><BranchReviewPage
          workspace={workspaces.find((candidate) => candidate.id === route.location?.workspaceId) ?? activeRepo}
          location={route.location}
          onLocationChange={(location) => navigate({ name: 'review', location })}
          onExit={() => navigate({ name: 'workstream' })}
          onImported={(itemId) => { void refreshAppData(); navigate({ name: 'item', itemId }); }}
          onCheckoutSwitched={async () => { await refreshAppData(); navigate({ name: 'workstream' }); }}
        /></Suspense>}
        {route.name === 'not-found' && <section className="empty-state" role="alert"><h1>Page not found</h1><p>This link is not a Kode Stream route.</p><button type="button" className="primary" onClick={() => navigate({ name: 'workstream' })}>Go to Workstream</button></section>}
      </main>

      <nav className="bottom-nav">
        <button className={route.name === 'workstream' ? 'active' : ''} onClick={() => navigate({ name: 'workstream' })}><WorkstreamIcon size={18} />Workstream</button>
        <button className={route.name === 'knowledge' ? 'active' : ''} onClick={() => navigate({ name: 'knowledge' })}><BookOpen size={18} />Knowledge</button>
		{!extensionSurface && <button className={route.name === 'canvas' ? 'active' : ''} onClick={() => navigate({ name: 'canvas', location: { workspaceId: activeRepo?.id } })}><Workflow size={18} />Workbench</button>}
        <button className={route.name === 'workspaces' ? 'active' : ''} onClick={() => navigate({ name: 'workspaces' })}><FolderGit2 size={18} />Workspaces</button>
        <button className={route.name === 'settings' ? 'active' : ''} onClick={() => navigate({ name: 'settings' })}><Settings size={18} />Settings</button>
      </nav>
		{!extensionSurface && <EmbeddedTerminalDock workspaces={workspaces} />}
    </div>
  );
}

function routeFromNavigationPath(path: string) {
  const url = new URL(path, window.location.origin);
  return routeFromPath(url.pathname, url.search);
}

export function LocalServerUnavailable({ status, apiOrigin, onRetry }: { status: 'checking' | 'unavailable'; apiOrigin: string; onRetry: () => void }) {
  return (
    <main className="main-content">
      <section className="empty-state" role={status === 'checking' ? 'status' : 'alert'} aria-live="polite">
        <h1>Kode Stream local server unavailable</h1>
        <p>{status === 'checking' ? `Checking ${apiOrigin}...` : `Start kode-stream serve -port 4317, then retry ${apiOrigin}.`}</p>
        <button className="primary" type="button" onClick={onRetry}>Retry</button>
      </section>
    </main>
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
