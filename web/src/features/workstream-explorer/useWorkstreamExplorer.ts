import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ExplorerLocation } from '../../app/router';
import { api } from '../../lib/api';
import type { ExplorerTreeMode, WorkspaceConfig } from '../../lib/types';
import { buildItemDecorations, directoryCacheKey, explorerNodeId, flattenVisibleTree } from './tree';
import type { DirectoryCacheEntry, ExplorerSelection } from './types';
import { ancestorDirectoryPaths, buildWorkspaceGitStateMap } from './productivity';
import type { WorkspacePathGitState } from '../../lib/types';

const expandedStorageKey = 'workstreamExplorer.expandedNodeIds';
const ignoredStorageKey = 'workstreamExplorer.showIgnored';
const modeStorageKey = 'workstreamExplorer.treeMode';

export function useWorkstreamExplorer(workspaces: WorkspaceConfig[], location?: ExplorerLocation, onLocationChange?: (location?: ExplorerLocation) => void) {
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(() => readExpanded());
  const [showIgnored, setShowIgnoredState] = useState(() => localStorage.getItem(ignoredStorageKey) === 'true');
	const [mode, setModeState] = useState<ExplorerTreeMode>(() => location?.mode ?? readMode());
  const [cache, setCache] = useState<Map<string, DirectoryCacheEntry>>(new Map());
  const [decorations, setDecorations] = useState(() => new Map());
  const [filter, setFilter] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [gitStateByPath, setGitStateByPath] = useState<Map<string, WorkspacePathGitState>>(new Map());
  const lastAutoExpandedLocation = useRef('');

  const loadDecorations = useCallback(async () => {
    try {
      setDecorations(buildItemDecorations(await api.items(new URLSearchParams())));
    } catch {
      setDecorations(new Map());
    }
  }, []);

  useEffect(() => { void loadDecorations(); }, [loadDecorations, workspaces]);

  const loadGitStates = useCallback(async () => {
    const maps = await Promise.all(workspaces.map(async (workspace) => {
      try {
        return buildWorkspaceGitStateMap(workspace.id, await api.workspacePathGitStates(workspace.id));
      } catch {
        return new Map<string, WorkspacePathGitState>();
      }
    }));
    setGitStateByPath(new Map(maps.flatMap((map) => [...map.entries()])));
  }, [workspaces]);

  useEffect(() => { void loadGitStates(); }, [loadGitStates]);

  const selection = useMemo<ExplorerSelection | undefined>(() => {
    if (!location?.workspaceId) return undefined;
    const path = location.path ?? '';
    const cachedEntry = [...cache.values()].flatMap((entry) => entry.entries).find((entry) => entry.path === path);
    return {
      nodeId: explorerNodeId(location.workspaceId, path),
      kind: path ? (cachedEntry?.type ?? 'file') : 'workspace',
      workspaceId: location.workspaceId,
      path
    };
  }, [cache, location]);

  const loadDirectory = useCallback(async (workspaceId: string, path: string, force = false) => {
		if (mode === 'sources' && path === '') return;
		const key = directoryCacheKey(workspaceId, path, showIgnored, mode);
    if (!force && ['loading', 'loaded'].includes(cache.get(key)?.state ?? '')) return;
    setCache((current) => new Map(current).set(key, { state: 'loading', entries: [], hiddenCount: 0 }));
    try {
      const listing = await api.workspaceTree(workspaceId, path, showIgnored);
      setCache((current) => new Map(current).set(key, { state: 'loaded', entries: listing.entries, hiddenCount: listing.hiddenCount }));
    } catch (error) {
      setCache((current) => new Map(current).set(key, { state: 'error', entries: [], hiddenCount: 0, error: error instanceof Error ? error.message : 'Directory failed to load' }));
    }
	}, [cache, mode, showIgnored]);

	const setMode = useCallback((value: ExplorerTreeMode) => {
		setModeState(value);
		localStorage.setItem(modeStorageKey, value);
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
      localStorage.setItem(expandedStorageKey, JSON.stringify([...next]));
      return next;
    });
    if (!expandedNodeIds.has(id)) void loadDirectory(workspaceId, path);
  }, [expandedNodeIds, loadDirectory]);

  const setShowIgnored = useCallback((value: boolean) => {
    setShowIgnoredState(value);
    localStorage.setItem(ignoredStorageKey, String(value));
    setCache(new Map());
  }, []);

  const refresh = useCallback(() => {
    setCache(new Map());
    for (const id of expandedNodeIds) {
      const separator = id.indexOf(':');
      void loadDirectory(id.slice(0, separator), id.slice(separator + 1), true);
    }
    void loadGitStates();
  }, [expandedNodeIds, loadDirectory, loadGitStates]);

  const refreshWorkspaceBranch = useCallback(async (workspaceId: string) => {
    const prefix = `${workspaceId}:`;
    setCache((current) => new Map([...current].filter(([key]) => !key.startsWith(prefix))));
    setGitStateByPath((current) => new Map([...current].filter(([key]) => !key.startsWith(prefix))));
    const paths = [...expandedNodeIds]
      .filter((id) => id.startsWith(prefix))
      .map((id) => id.slice(prefix.length));
    await Promise.all(paths.map((path) => loadDirectory(workspaceId, path, true)));
    await Promise.all([loadGitStates(), loadDecorations()]);
  }, [expandedNodeIds, loadDecorations, loadDirectory, loadGitStates]);

  const select = useCallback((workspaceId: string, path: string) => onLocationChange?.({ workspaceId, path: path || undefined }), [onLocationChange]);

  const invalidateDirectories = useCallback(async (workspaceId: string, paths: string[]) => {
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

  const expandToPath = useCallback(async (workspaceId: string, path: string, type: 'file' | 'directory' = 'file') => {
    const ancestors = ancestorDirectoryPaths(path, type);
    setExpandedNodeIds((current) => {
      const next = new Set(current);
      ancestors.forEach((ancestor) => next.add(explorerNodeId(workspaceId, ancestor)));
      localStorage.setItem(expandedStorageKey, JSON.stringify([...next]));
      return next;
    });
    await Promise.all(ancestors.map((ancestor) => loadDirectory(workspaceId, ancestor)));
    select(workspaceId, path);
  }, [loadDirectory, select]);

  const collapseAll = useCallback(() => {
    setExpandedNodeIds(new Set());
    localStorage.setItem(expandedStorageKey, '[]');
  }, []);

  useEffect(() => {
    for (const id of expandedNodeIds) {
      const separator = id.indexOf(':');
      if (separator > 0) void loadDirectory(id.slice(0, separator), id.slice(separator + 1));
    }
  }, [expandedNodeIds, loadDirectory, showIgnored, workspaces]);

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
      localStorage.setItem(expandedStorageKey, JSON.stringify([...next]));
      return next;
    });
    directoryPaths.forEach((path) => void loadDirectory(location.workspaceId!, path));
  }, [loadDirectory, location?.path, location?.workspaceId, mode, showIgnored]);

	const rows = useMemo(() => flattenVisibleTree({ workspaces, expandedNodeIds, cache, includeIgnored: showIgnored, decorations, filter, mode }), [cache, decorations, expandedNodeIds, filter, mode, showIgnored, workspaces]);

	return { rows, cache, decorations, gitStateByPath, expandedNodeIds, showIgnored, mode, filter, activeIndex, selection, setFilter, setActiveIndex, setShowIgnored, setMode, toggleExpanded, loadDirectory, refresh, refreshWorkspaceBranch, collapseAll, select, invalidateDirectories, expandToPath };
}

function readMode(): ExplorerTreeMode {
	return localStorage.getItem(modeStorageKey) === 'all' ? 'all' : 'sources';
}

function readExpanded(): Set<string> {
  try {
    const value = JSON.parse(localStorage.getItem(expandedStorageKey) ?? '[]');
    return new Set(Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []);
  } catch {
    return new Set();
  }
}
