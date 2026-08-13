import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../shared/api';
import type { AppState, RuntimeContext, WorkspaceConfig } from '../lib/types';
import { pathForRoute, routeDescriptors, routeFromLocation } from './router';
import type { Route } from './router';
import { readStringPreference, removePreference, writePreference } from '../shared/preferences/store';

const localRuntimeContext: RuntimeContext = {
  mode: 'local',
  role: 'admin',
  capabilities: {
    read: true,
    write: true,
    workspace_registration: true,
    git: true,
    system: true,
    terminal: true,
    ai: true,
    runtime: true,
    verification: true
  },
  agent: { available: true, status: 'local' }
};

function normalizeRuntimeContext(state?: AppState): RuntimeContext {
  return {
    mode: state?.mode === 'cloud' ? 'cloud' : 'local',
    user: state?.user,
    role: state?.role ?? (state?.mode === 'cloud' ? undefined : 'admin'),
    capabilities: { ...localRuntimeContext.capabilities, ...(state?.capabilities ?? {}) },
    agent: state?.agent ?? localRuntimeContext.agent
  };
}

const unavailableRuntimeContext: RuntimeContext = {
  mode: 'cloud', role: 'viewer', capabilities: {
    read: false, write: false, workspace_registration: false, git: false, system: false,
    terminal: false, ai: false, runtime: false, verification: false
  }, agent: { available: false, status: 'offline' }
};

export type AppDataStatus = 'loading' | 'ready' | 'unavailable';

export function useAppState() {
  const [route, setRoute] = useState<Route>(routeFromLocation);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => readStringPreference('theme') === 'dark' ? 'dark' : 'light');
  const [workspaces, setWorkspaces] = useState<WorkspaceConfig[]>([]);
  const [runtimeContext, setRuntimeContext] = useState<RuntimeContext>(unavailableRuntimeContext);
  const [dataStatus, setDataStatus] = useState<AppDataStatus>('loading');
  const [activeWorkspaceId, setActiveWorkspaceId] = useState(() => readStringPreference('activeWorkspaceId'));
  const [contentRefreshKey, setContentRefreshKey] = useState(0);
  const generation = useRef(0);
  const controller = useRef<AbortController | undefined>(undefined);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    writePreference('theme', theme);
  }, [theme]);

  useEffect(() => {
    const onPop = () => setRoute(routeFromLocation());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  useEffect(() => {
    const descriptor = routeDescriptors[route.name];
    document.title = descriptor.title;
    const main = document.getElementById(descriptor.mainId);
    main?.focus();
  }, [route]);

  const navigate = useCallback((next: Route) => {
    history.pushState(null, '', pathForRoute(next));
    setRoute(next);
  }, []);

  const refreshAppData = useCallback(async (advanceContent = true) => {
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    const requestGeneration = ++generation.current;
    setDataStatus('loading');
    try {
      const [state, nextWorkspaces] = await Promise.all([api.state(active.signal), api.workspaces(active.signal)]);
      if (active.signal.aborted || requestGeneration !== generation.current) return false;
      setRuntimeContext(normalizeRuntimeContext(state));
      setWorkspaces(nextWorkspaces);
      setDataStatus('ready');
      if (advanceContent) setContentRefreshKey((key) => key + 1);
      return true;
    } catch (error) {
      if (active.signal.aborted || requestGeneration !== generation.current) return false;
      // A failure must never masquerade as a trusted Local administrator session or an empty registry.
      setRuntimeContext((current) => current.mode === 'local' ? current : unavailableRuntimeContext);
      setDataStatus('unavailable');
      return false;
    }
  }, []);
  const refreshAppStateOnly = useCallback(async () => refreshAppData(false), [refreshAppData]);

  useEffect(() => {
    void refreshAppData();
    return () => controller.current?.abort();
  }, [refreshAppData]);

  useEffect(() => {
    const refreshCheckoutContext = () => void refreshAppData();
    const refreshVisibleCheckoutContext = () => {
      if (document.visibilityState === 'visible') refreshCheckoutContext();
    };
    window.addEventListener('focus', refreshCheckoutContext);
    document.addEventListener('visibilitychange', refreshVisibleCheckoutContext);
    return () => {
      window.removeEventListener('focus', refreshCheckoutContext);
      document.removeEventListener('visibilitychange', refreshVisibleCheckoutContext);
    };
  }, [refreshAppData]);

  useEffect(() => {
    if (workspaces.length === 0) {
      setActiveWorkspaceId('');
      removePreference('activeWorkspaceId');
      return;
    }
    if (!workspaces.some((repo) => repo.id === activeWorkspaceId)) {
      const nextId = workspaces[0].id;
      setActiveWorkspaceId(nextId);
      writePreference('activeWorkspaceId', nextId);
    }
  }, [activeWorkspaceId, workspaces]);

  const selectWorkspace = (repo: WorkspaceConfig) => {
    setActiveWorkspaceId(repo.id);
    writePreference('activeWorkspaceId', repo.id);
    if (route.name === 'knowledge') {
      navigate({ name: 'knowledge', location: { workspaceId: repo.id, view: 'browse' } });
      return;
    }
		if (route.name === 'canvas') {
			navigate({ name: 'canvas', location: { workspaceId: repo.id } });
			return;
		}
    if (route.name === 'review') {
      navigate({ name: 'review', location: { workspaceId: repo.id } });
      return;
    }
    navigate({ name: 'workstream' });
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
    runtimeContext,
    dataStatus,
    contentRefreshKey,
    navigate,
    selectWorkspace,
    refreshAppData,
    refreshAppStateOnly,
    lastSync
  };
}
