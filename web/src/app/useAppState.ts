import { useEffect, useMemo, useState } from 'react';
import { api } from '../lib/api';
import type { WorkspaceConfig } from '../lib/types';
import { pathForRoute, routeFromLocation } from './router';
import type { Route } from './router';

export function useAppState() {
  const [route, setRoute] = useState<Route>(routeFromLocation);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => (localStorage.getItem('theme') as 'light' | 'dark') || 'light');
  const [workspaces, setWorkspaces] = useState<WorkspaceConfig[]>([]);
  const [activeWorkspaceId, setActiveWorkspaceId] = useState(() => localStorage.getItem('activeWorkspaceId') ?? '');
  const [contentRefreshKey, setContentRefreshKey] = useState(0);

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
    history.pushState(null, '', pathForRoute(next));
    setRoute(next);
  };

  const refreshWorkspaces = () => api.workspaces().then(setWorkspaces).catch(() => setWorkspaces([]));
  const refreshAppData = async () => {
    await refreshWorkspaces();
    setContentRefreshKey((key) => key + 1);
  };
  const refreshAppStateOnly = async () => {
    await refreshWorkspaces();
  };

  useEffect(() => {
    void refreshAppData();
  }, []);

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

  const selectWorkspace = (repo: WorkspaceConfig) => {
    setActiveWorkspaceId(repo.id);
    localStorage.setItem('activeWorkspaceId', repo.id);
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

  return {
    route,
    theme,
    setTheme,
    workspaces,
    activeRepo,
    contentRefreshKey,
    navigate,
    selectWorkspace,
    refreshAppData,
    refreshAppStateOnly,
    lastSync
  };
}
