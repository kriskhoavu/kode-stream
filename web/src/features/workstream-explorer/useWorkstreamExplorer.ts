import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ExplorerLocation } from './types';
import { api } from '../../shared/api';
import { readPreference, readStringPreference, writePreference } from '../../shared/preferences/store';
import type { ExplorerTreeMode, WorkspaceConfig } from '../../lib/types';
import { sourcePaths } from '../../lib/sources';
import { buildItemDecorations, directoryCacheKey, explorerNodeId, flattenVisibleTree } from './tree';
import type { DirectoryCacheEntry, ExplorerSelection } from './types';
import { ancestorDirectoryPaths, buildWorkspaceGitStateMap } from './productivity';
import type { WorkspacePathGitState } from '../../lib/types';

const expandedStorageKey = 'workstreamExplorer.expandedNodeIds';
const ignoredStorageKey = 'workstreamExplorer.showIgnored';
const modeStorageKey = 'workstreamExplorer.treeMode';

export function useWorkstreamExplorer(workspaces: WorkspaceConfig[], location?: ExplorerLocation, onLocationChange?: (location?: ExplorerLocation) => void) {
  const workspaceKey = workspaces
    .map((workspace) => [workspace.id, workspace.name, workspace.baselineBranch, sourcePaths(workspace).join('|')].join(':'))
    .join('\u0000');
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(() => readExpanded());
  const [showIgnored, setShowIgnoredState] = useState(() => readStringPreference(ignoredStorageKey) === 'true');
	const [mode, setModeState] = useState<ExplorerTreeMode>(() => location?.mode ?? readMode());
  const [cache, setCache] = useState<Map<string, DirectoryCacheEntry>>(new Map());
  const [decorations, setDecorations] = useState(() => new Map());
  const [filter, setFilter] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [gitStateByPath, setGitStateByPath] = useState<Map<string, WorkspacePathGitState>>(new Map());
  const lastAutoExpandedLocation = useRef('');
  const cacheRef = useRef(cache);
  const requestID = useRef(0);
  const inFlightDirectories = useRef(new Map<string, number>());
  const directoryAbortControllers = useRef(new Map<string, AbortController>());
  const decorationsAbortRef = useRef<AbortController | undefined>(undefined);
  const gitStatesAbortRef = useRef<AbortController | undefined>(undefined);

  const abortDirectoryRequests = useCallback((matches: (key: string) => boolean = () => true) => {
    for (const [key, controller] of directoryAbortControllers.current) {
      if (!matches(key)) continue;
      controller.abort();
      directoryAbortControllers.current.delete(key);
      inFlightDirectories.current.delete(key);
    }
  }, []);

  useEffect(() => {
    cacheRef.current = cache;
  }, [cache]);

  useEffect(() => {
    const activeWorkspaceIDs = new Set(workspaces.map((workspace) => workspace.id));
    const belongsToActiveWorkspace = (key: string) => activeWorkspaceIDs.has(key.slice(0, key.indexOf(':')));
    abortDirectoryRequests((key) => !belongsToActiveWorkspace(key));
    setCache((current) => {
      const next = new Map([...current].filter(([key]) => belongsToActiveWorkspace(key)));
      return next.size === current.size ? current : next;
    });
  }, [abortDirectoryRequests, workspaceKey]);

  const loadDecorations = useCallback(async () => {
    decorationsAbortRef.current?.abort();
    const controller = new AbortController();
    decorationsAbortRef.current = controller;
    try {
      const items = await api.items(new URLSearchParams(), controller.signal);
      if (!controller.signal.aborted) setDecorations(buildItemDecorations(items));
    } catch {
      if (!controller.signal.aborted) setDecorations(new Map());
    } finally {
      if (decorationsAbortRef.current === controller) decorationsAbortRef.current = undefined;
    }
  }, []);

  useEffect(() => {
    void loadDecorations();
    return () => decorationsAbortRef.current?.abort();
  }, [loadDecorations]);

  const loadGitStates = useCallback(async () => {
    gitStatesAbortRef.current?.abort();
    const controller = new AbortController();
    gitStatesAbortRef.current = controller;
    const maps = await Promise.all(workspaces.map(async (workspace) => {
      try {
        return buildWorkspaceGitStateMap(workspace.id, await api.workspacePathGitStates(workspace.id, controller.signal));
      } catch {
        return new Map<string, WorkspacePathGitState>();
      }
    }));
    if (!controller.signal.aborted) setGitStateByPath(new Map(maps.flatMap((map) => [...map.entries()])));
    if (gitStatesAbortRef.current === controller) gitStatesAbortRef.current = undefined;
  }, [workspaceKey]);

  useEffect(() => {
    void loadGitStates();
    return () => gitStatesAbortRef.current?.abort();
  }, [loadGitStates]);

  const selection = useMemo<ExplorerSelection | undefined>(() => {
    if (!location?.workspaceId) return undefined;
    const path = location.path ?? '';
    const parentPath = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '';
    const cachedEntry = cache.get(directoryCacheKey(location.workspaceId, parentPath, showIgnored, mode))?.entries.find((entry) => entry.path === path);
    return {
      nodeId: explorerNodeId(location.workspaceId, path),
      kind: path ? (cachedEntry?.type ?? 'file') : 'workspace',
      workspaceId: location.workspaceId,
      path
    };
  }, [cache, location, mode, showIgnored]);

  const loadDirectory = useCallback(async (workspaceId: string, path: string, force = false) => {
		if (mode === 'sources' && path === '') return;
		const key = directoryCacheKey(workspaceId, path, showIgnored, mode);
    if (!force && (inFlightDirectories.current.has(key) || ['loading', 'loaded'].includes(cacheRef.current.get(key)?.state ?? ''))) return;
    if (force) directoryAbortControllers.current.get(key)?.abort();
    const id = ++requestID.current;
    const controller = new AbortController();
    inFlightDirectories.current.set(key, id);
    directoryAbortControllers.current.set(key, controller);
    setCache((current) => new Map(current).set(key, { state: 'loading', entries: [], hiddenCount: 0 }));
    try {
      const listing = force
        ? await api.workspaceTree(workspaceId, path, showIgnored, true, controller.signal)
        : await api.workspaceTree(workspaceId, path, showIgnored, false, controller.signal);
      if (controller.signal.aborted || inFlightDirectories.current.get(key) !== id) return;
      setCache((current) => new Map(current).set(key, { state: 'loaded', entries: listing.entries, hiddenCount: listing.hiddenCount }));
    } catch (error) {
      if (controller.signal.aborted || inFlightDirectories.current.get(key) !== id) return;
      setCache((current) => new Map(current).set(key, { state: 'error', entries: [], hiddenCount: 0, error: error instanceof Error ? error.message : 'Directory failed to load' }));
    } finally {
      if (inFlightDirectories.current.get(key) === id) inFlightDirectories.current.delete(key);
      if (directoryAbortControllers.current.get(key) === controller) directoryAbortControllers.current.delete(key);
    }
	}, [mode, showIgnored]);

	const setMode = useCallback((value: ExplorerTreeMode) => {
		setModeState(value);
		writePreference(modeStorageKey, value);
		onLocationChange?.({ ...location, mode: value });
	}, [location, onLocationChange]);

	useEffect(() => {
		if (location?.mode && location.mode !== mode) setModeState(location.mode);
	}, [location?.mode, mode]);

  const toggleExpanded = useCallback((workspaceId: string, path: string) => {
    const id = explorerNodeId(workspaceId, path);
    setExpandedNodeIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      writePreference(expandedStorageKey, [...next]);
      return next;
    });
    if (!expandedNodeIds.has(id)) void loadDirectory(workspaceId, path);
  }, [expandedNodeIds, loadDirectory]);

  const setShowIgnored = useCallback((value: boolean) => {
    setShowIgnoredState(value);
    writePreference(ignoredStorageKey, String(value));
    abortDirectoryRequests();
    setCache(new Map());
  }, [abortDirectoryRequests]);

  const refresh = useCallback(() => {
    abortDirectoryRequests();
    setCache(new Map());
    for (const id of expandedNodeIds) {
      const separator = id.indexOf(':');
      void loadDirectory(id.slice(0, separator), id.slice(separator + 1), true);
    }
    void loadGitStates();
  }, [abortDirectoryRequests, expandedNodeIds, loadDirectory, loadGitStates]);

  const refreshWorkspaceBranch = useCallback(async (workspaceId: string) => {
    const prefix = `${workspaceId}:`;
    abortDirectoryRequests((key) => key.startsWith(prefix));
    setCache((current) => new Map([...current].filter(([key]) => !key.startsWith(prefix))));
    setGitStateByPath((current) => new Map([...current].filter(([key]) => !key.startsWith(prefix))));
    const paths = [...expandedNodeIds]
      .filter((id) => id.startsWith(prefix))
      .map((id) => id.slice(prefix.length));
    await Promise.all(paths.map((path) => loadDirectory(workspaceId, path, true)));
    await Promise.all([loadGitStates(), loadDecorations()]);
  }, [abortDirectoryRequests, expandedNodeIds, loadDecorations, loadDirectory, loadGitStates]);

  const select = useCallback((workspaceId: string, path: string) => onLocationChange?.({ workspaceId, path: path || undefined }), [onLocationChange]);

  const invalidateDirectories = useCallback(async (workspaceId: string, paths: string[]) => {
    for (const path of paths) {
      for (const treeMode of ['sources', 'all'] as const) {
        inFlightDirectories.current.delete(directoryCacheKey(workspaceId, path, false, treeMode));
        inFlightDirectories.current.delete(directoryCacheKey(workspaceId, path, true, treeMode));
        directoryAbortControllers.current.get(directoryCacheKey(workspaceId, path, false, treeMode))?.abort();
        directoryAbortControllers.current.delete(directoryCacheKey(workspaceId, path, false, treeMode));
        directoryAbortControllers.current.get(directoryCacheKey(workspaceId, path, true, treeMode))?.abort();
        directoryAbortControllers.current.delete(directoryCacheKey(workspaceId, path, true, treeMode));
      }
    }
    setCache((current) => {
      const next = new Map(current);
      for (const path of paths) {
			for (const treeMode of ['sources', 'all'] as const) {
				next.delete(directoryCacheKey(workspaceId, path, false, treeMode));
				next.delete(directoryCacheKey(workspaceId, path, true, treeMode));
			}
      }
      return next;
    });
    await Promise.all(paths.filter((path) => expandedNodeIds.has(explorerNodeId(workspaceId, path))).map((path) => loadDirectory(workspaceId, path, true)));
    await loadGitStates();
  }, [expandedNodeIds, loadDirectory, loadGitStates]);

  useEffect(() => () => abortDirectoryRequests(), [abortDirectoryRequests]);

  const expandPath = useCallback(async (workspaceId: string, path: string, type: 'file' | 'directory' = 'file') => {
    const ancestors = ancestorDirectoryPaths(path, type);
    setExpandedNodeIds((current) => {
      const ids = ancestors.map((ancestor) => explorerNodeId(workspaceId, ancestor));
      if (ids.every((id) => current.has(id))) return current;
      const next = new Set([...current, ...ids]);
      writePreference(expandedStorageKey, [...next]);
      return next;
    });
    await Promise.all(ancestors.map((ancestor) => loadDirectory(workspaceId, ancestor)));
  }, [loadDirectory]);

  const expandToPath = useCallback(async (workspaceId: string, path: string, type: 'file' | 'directory' = 'file') => {
    await expandPath(workspaceId, path, type);
    select(workspaceId, path);
  }, [expandPath, select]);

  const collapseAll = useCallback(() => {
    setExpandedNodeIds(new Set());
    writePreference(expandedStorageKey, []);
  }, []);

  const collapsePath = useCallback((workspaceId: string, path = '') => {
    const normalizedPath = explorerNodeId(workspaceId, path);
    const descendantPrefix = normalizedPath.endsWith(':') ? normalizedPath : `${normalizedPath}/`;
    setExpandedNodeIds((current) => {
      const next = new Set([...current].filter((id) => id !== normalizedPath && !id.startsWith(descendantPrefix)));
      writePreference(expandedStorageKey, [...next]);
      return next;
    });
  }, []);

  useEffect(() => {
    for (const id of expandedNodeIds) {
      const separator = id.indexOf(':');
      if (separator > 0) void loadDirectory(id.slice(0, separator), id.slice(separator + 1));
    }
  }, [expandedNodeIds, loadDirectory, showIgnored]);

  useEffect(() => {
    if (!location?.workspaceId) {
      lastAutoExpandedLocation.current = '';
      return;
    }
    const locationKey = JSON.stringify([location.workspaceId, location.path ?? '', mode, showIgnored]);
    if (lastAutoExpandedLocation.current === locationKey) return;
    lastAutoExpandedLocation.current = locationKey;
    const segments = (location.path ?? '').split('/').filter(Boolean);
    const directoryPaths = ['', ...segments.slice(0, -1).map((_, index) => segments.slice(0, index + 1).join('/'))];
    setExpandedNodeIds((current) => {
      const next = new Set(current);
      directoryPaths.forEach((path) => next.add(explorerNodeId(location.workspaceId!, path)));
      if (next.size === current.size) return current;
      writePreference(expandedStorageKey, [...next]);
      return next;
    });
    directoryPaths.forEach((path) => void loadDirectory(location.workspaceId!, path));
  }, [loadDirectory, location?.path, location?.workspaceId, mode, showIgnored]);

	const rows = useMemo(() => flattenVisibleTree({ workspaces, expandedNodeIds, cache, includeIgnored: showIgnored, decorations, filter, mode }), [cache, decorations, expandedNodeIds, filter, mode, showIgnored, workspaceKey]);

	return { rows, cache, decorations, gitStateByPath, expandedNodeIds, showIgnored, mode, filter, activeIndex, selection, setFilter, setActiveIndex, setShowIgnored, setMode, toggleExpanded, loadDirectory, refresh, refreshWorkspaceBranch, collapseAll, collapsePath, select, invalidateDirectories, expandPath, expandToPath };
}

function readMode(): ExplorerTreeMode {
	return readStringPreference(modeStorageKey) === 'all' ? 'all' : 'sources';
}

function readExpanded(): Set<string> {
  return new Set(readPreference<string[]>(expandedStorageKey, (value) => Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : undefined, []));
}
