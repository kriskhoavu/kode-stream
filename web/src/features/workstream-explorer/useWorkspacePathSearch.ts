import { useEffect, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { WorkspacePathSearchResult } from '../../lib/types';

export function useWorkspacePathSearch({ workspaceId, includeIgnored, debounceMs = 250 }: { workspaceId?: string; includeIgnored: boolean; debounceMs?: number }) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<WorkspacePathSearchResult[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const requestID = useRef(0);

  useEffect(() => {
    const normalized = query.trim();
    const id = ++requestID.current;
    if (!normalized) {
      setResults([]);
      setTruncated(false);
      setLoading(false);
      setError('');
      return;
    }
    setLoading(true);
    const timer = window.setTimeout(() => {
      api.searchWorkspacePaths({ q: normalized, workspaceId, includeIgnored }).then((response) => {
        if (requestID.current !== id) return;
        setResults(response.results);
        setTruncated(response.truncated);
        setError('');
      }).catch((caught: unknown) => {
        if (requestID.current !== id) return;
        setResults([]);
        setError(caught instanceof Error ? caught.message : 'Path search failed');
      }).finally(() => {
        if (requestID.current === id) setLoading(false);
      });
    }, debounceMs);
    return () => window.clearTimeout(timer);
  }, [debounceMs, includeIgnored, query, workspaceId]);

  return { query, setQuery, results, truncated, loading, error };
}
