import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { useDismiss } from './components/useDismiss';
import { LeftNav, WorkspaceMenu } from './app/AppNavigation';
import { Bell, BookOpen, ChevronDown, KanbanSquare as WorkstreamIcon, Moon, Search, Sun, FolderGit2, Settings, Bot } from 'lucide-react';
import type { WorkspaceConfig } from './lib/types';
import { useAppState } from './app/useAppState';
import { routeFromPath } from './app/router';
import { api, isExtensionSurface, localAPIOrigin } from './shared/api';
import { ActivityPanel } from './components/ReliabilityPanels';
import { HeaderBackdrop, bandClasses } from './components/HeaderBackdrop';
import { SearchDialog } from './components/SearchDialog';
import { useQuickSwitcher } from './features/search/hooks';
import { useAppSettings } from './features/settings/appSettings';
import { EmbeddedTerminalDock } from './features/ai-session/EmbeddedTerminalDock';
import { readStringPreference, writePreference } from './shared/preferences/store';
import './features/harness/harness.css';

const WorkstreamPage = lazy(() => import('./pages/WorkstreamPage').then((module) => ({ default: module.WorkstreamPage })));
const ItemWorkspacePage = lazy(() => import('./pages/ItemWorkspacePage').then((module) => ({ default: module.ItemWorkspacePage })));
const WorkspacesPage = lazy(() => import('./pages/WorkspacesPage').then((module) => ({ default: module.WorkspacesPage })));
const SettingsPage = lazy(() => import('./pages/SettingsPage').then((module) => ({ default: module.SettingsPage })));
const KnowledgePage = lazy(() => import('./pages/KnowledgePage').then((module) => ({ default: module.KnowledgePage })));
const CanvasPage = lazy(() => import('./pages/CanvasPage').then((module) => ({ default: module.CanvasPage })));
const BranchReviewPage = lazy(() => import('./pages/BranchReviewPage').then((module) => ({ default: module.BranchReviewPage })));
const HarnessRunRedirect = lazy(() => import('./features/workstream/HarnessRunRedirect').then((module) => ({ default: module.HarnessRunRedirect })));

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
    replace,
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

  useDismiss(workspaceMenuRef, workspaceMenuOpen, () => setWorkspaceMenuOpen(false));
  useDismiss(profileMenuRef, profileMenuOpen, () => setProfileMenuOpen(false));

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

  const band = bandClasses(route.name, appSettings.headerBackdrop);
  const shellClasses = ['app-shell', leftNavCollapsed ? 'left-nav-collapsed' : '', band].filter(Boolean).join(' ');

  return (
    <div className={shellClasses}>
      {band && <HeaderBackdrop variant={appSettings.headerBackdrop} />}
      <LeftNav route={route} navigate={navigate} workspaces={workspaces} activeWorkspaceId={activeRepo?.id} lastSync={lastSync} collapsed={leftNavCollapsed} onToggle={toggleLeftNav} showAgentSpace={!extensionSurface} onSelectWorkspace={selectWorkspace} />

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
          {workspaceMenuOpen && <WorkspaceMenu workspaces={workspaces} activeWorkspaceId={activeRepo?.id} onSelect={selectWorkspace} onManage={() => {
            setWorkspaceMenuOpen(false);
            navigate({ name: 'workspaces' });
          }} />}
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
        {dataStatus === 'unavailable' && <section className="empty-state" role="alert"><p>Application data is unavailable. Your existing workspace view is retained.</p><button type="button" className="primary" onClick={() => void refreshAppData()}>Retry</button></section>}
        {route.name === 'workstream' && <Suspense fallback={<section className="empty-state">Loading Workstream...</section>}>
          <WorkstreamPage
            workspace={activeRepo}
            refreshKey={contentRefreshKey}
            visibleStatuses={appSettings.visibleWorkstreamStatuses}
            focusedItemId={route.focusedItemId}
            focusedSessionId={route.focusedSessionId}
            onOpenPlan={(itemId, view) => navigate({ name: 'item', itemId, view })}
            onOpenHarnessRun={(runId) => navigate({ name: 'harness-run', runId })}
            onWorkspacesChanged={async () => { await refreshAppData(); }}
            onOpenWorkspaces={() => navigate({ name: 'workspaces' })}
            onOpenReview={() => navigate({ name: 'review', location: { workspaceId: activeRepo?.id } })}
          />
        </Suspense>}
        {route.name === 'item' && <Suspense fallback={<section className="empty-state">Loading item...</section>}><ItemWorkspacePage key={route.itemId} itemId={route.itemId} view={route.view} onViewChange={(view) => navigate({ name: 'item', itemId: route.itemId, view })} refreshKey={contentRefreshKey} workspaces={workspaces} allowEmbeddedAISessions={!extensionSurface} onBack={() => navigate({ name: 'workstream' })} onContentChanged={async () => { await refreshAppStateOnly(); }} /></Suspense>}
        {route.name === 'workspaces' && <Suspense fallback={<section className="empty-state">Loading workspaces...</section>}><WorkspacesPage workspaces={workspaces} runtimeContext={runtimeContext} showHarness={!extensionSurface} onChanged={async () => { await refreshAppData(); }} /></Suspense>}
        {route.name === 'settings' && <Suspense fallback={<section className="empty-state">Loading settings...</section>}><SettingsPage settings={appSettings} onChange={setAppSettings} showHarness={!extensionSurface} /></Suspense>}
        {route.name === 'knowledge' && <Suspense fallback={<section className="empty-state">Loading Knowledge...</section>}><KnowledgePage workspaces={workspaces} activeWorkspace={activeRepo} location={route.location} onLocationChange={(location) => navigate({ name: 'knowledge', location })} /></Suspense>}
		{route.name === 'canvas' && !extensionSurface && <Suspense fallback={<section className="empty-state">Loading Agent Space...</section>}><CanvasPage workspace={workspaces.find((candidate) => candidate.id === route.location?.workspaceId) ?? activeRepo} location={route.location} onLocationChange={(location) => navigate({ name: 'canvas', location })} onOpenItem={(itemId) => navigate({ name: 'item', itemId })} onOpenOnCard={(itemId, sessionId) => navigate({ name: 'workstream', focusedItemId: itemId, focusedSessionId: sessionId })} onOpenHarnessRun={(runId) => navigate({ name: 'harness-run', runId })} onOpenWorkspaces={() => navigate({ name: 'workspaces' })} /></Suspense>}
        {route.name === 'review' && <Suspense fallback={<section className="empty-state">Loading Branch Review...</section>}><BranchReviewPage
          workspace={workspaces.find((candidate) => candidate.id === route.location?.workspaceId) ?? activeRepo}
          location={route.location}
          onLocationChange={(location) => navigate({ name: 'review', location })}
          onExit={() => navigate({ name: 'workstream' })}
          onImported={(itemId) => { void refreshAppData(); navigate({ name: 'item', itemId }); }}
          onCheckoutSwitched={async () => { await refreshAppData(); navigate({ name: 'workstream' }); }}
        /></Suspense>}
        {route.name === 'harness-run' && <Suspense fallback={<section className="empty-state">Opening this run…</section>}><HarnessRunRedirect runId={route.runId} onResolved={(itemId) => replace({ name: 'item', itemId, view: 'run' })} onMissing={() => replace({ name: 'workstream' })} onBack={() => navigate({ name: 'workstream' })} /></Suspense>}
        {route.name === 'not-found' && <section className="empty-state" role="alert"><h1>Page not found</h1><p>This link is not a Kode Stream route.</p><button type="button" className="primary" onClick={() => navigate({ name: 'workstream' })}>Go to Workstream</button></section>}
      </main>

      <nav className="bottom-nav">
        <button className={route.name === 'workstream' ? 'active' : ''} onClick={() => navigate({ name: 'workstream' })}><WorkstreamIcon size={18} />Workstream</button>
        <button className={route.name === 'knowledge' ? 'active' : ''} onClick={() => navigate({ name: 'knowledge' })}><BookOpen size={18} />Knowledge</button>
		{!extensionSurface && <button className={route.name === 'canvas' ? 'active' : ''} onClick={() => navigate({ name: 'canvas', location: { workspaceId: activeRepo?.id } })}><Bot size={18} />Agent Space</button>}
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
