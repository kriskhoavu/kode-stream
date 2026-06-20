import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ExplorerLocation } from '../../app/router';
import { api } from '../../lib/api';
import type { WorkspaceConfig } from '../../lib/types';
import { buildItemDecorations, directoryCacheKey, explorerNodeId, flattenVisibleTree } from './tree';
import type { DirectoryCacheEntry, ExplorerSelection } from './types';

const expandedStorageKey = 'workspaceExplorer.expandedNodeIds';
const ignoredStorageKey = 'workspaceExplorer.showIgnored';

export function useWorkspaceExplorer(workspaces: WorkspaceConfig[], location?: ExplorerLocation, onLocationChange?: (location?: ExplorerLocation) => void) {
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(() => readExpanded());
  const [showIgnored, setShowIgnoredState] = useState(() => localStorage.getItem(ignoredStorageKey) === 'true');
  const [cache, setCache] = useState<Map<string, DirectoryCacheEntry>>(new Map());
  const [decorations, setDecorations] = useState(() => new Map());
  const [filter, setFilter] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    api.items(new URLSearchParams()).then((items) => setDecorations(buildItemDecorations(items))).catch(() => setDecorations(new Map()));
  }, [workspaces]);

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
    const key = directoryCacheKey(workspaceId, path, showIgnored);
    if (!force && ['loading', 'loaded'].includes(cache.get(key)?.state ?? '')) return;
    setCache((current) => new Map(current).set(key, { state: 'loading', entries: [], hiddenCount: 0 }));
    try {
      const listing = await api.workspaceTree(workspaceId, path, showIgnored);
      setCache((current) => new Map(current).set(key, { state: 'loaded', entries: listing.entries, hiddenCount: listing.hiddenCount }));
    } catch (error) {
      setCache((current) => new Map(current).set(key, { state: 'error', entries: [], hiddenCount: 0, error: error instanceof Error ? error.message : 'Directory failed to load' }));
    }
  }, [cache, showIgnored]);

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
  }, [expandedNodeIds, loadDirectory]);

  const collapseAll = useCallback(() => {
    setExpandedNodeIds(new Set());
    localStorage.setItem(expandedStorageKey, '[]');
  }, []);

  useEffect(() => {
    for (const workspace of workspaces) {
      if (expandedNodeIds.has(explorerNodeId(workspace.id, ''))) void loadDirectory(workspace.id, '');
    }
  }, [expandedNodeIds, loadDirectory, workspaces]);

  const select = useCallback((workspaceId: string, path: string) => onLocationChange?.({ workspaceId, path: path || undefined }), [onLocationChange]);
  const rows = useMemo(() => flattenVisibleTree({ workspaces, expandedNodeIds, cache, includeIgnored: showIgnored, decorations, filter }), [cache, decorations, expandedNodeIds, filter, showIgnored, workspaces]);

  return { rows, cache, decorations, expandedNodeIds, showIgnored, filter, activeIndex, selection, setFilter, setActiveIndex, setShowIgnored, toggleExpanded, loadDirectory, refresh, collapseAll, select };
}

function readExpanded(): Set<string> {
  try {
    const value = JSON.parse(localStorage.getItem(expandedStorageKey) ?? '[]');
    return new Set(Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []);
  } catch {
    return new Set();
  }
}
