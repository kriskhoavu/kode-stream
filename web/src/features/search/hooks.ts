import { useCallback, useEffect, useState } from 'react';
import { api } from '../../lib/api';
import type { SearchResult } from '../../lib/types';

export function useQuickSwitcher() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((current) => !current);
      } else if (event.key === 'Escape') {
        setOpen(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  return { open, setOpen, close: () => setOpen(false) };
}

export function useGlobalSearch({ workspaceId, allWorkspaces, onNavigate }: {
  workspaceId?: string;
  allWorkspaces: boolean;
  onNavigate: (route: string, result: SearchResult) => void;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const text = query.trim();
    if (!text) {
      setResults([]);
      setLoading(false);
      setError('');
      setSelectedIndex(0);
      return;
    }
    setLoading(true);
    const timer = window.setTimeout(() => {
      api.search({ q: text, workspaceId: allWorkspaces ? undefined : workspaceId, limit: 30 })
        .then((next) => {
          setResults(next);
          setSelectedIndex(0);
          setError('');
        })
        .catch((caught: Error) => {
          setResults([]);
          setError(caught.message);
        })
        .finally(() => setLoading(false));
    }, 180);
    return () => window.clearTimeout(timer);
  }, [query, workspaceId, allWorkspaces]);

  const selectResult = useCallback((result: SearchResult) => {
    if (result.itemId) void api.recordRecentItem(result.itemId).catch(() => undefined);
    onNavigate(result.route, result);
  }, [onNavigate]);

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setSelectedIndex((index) => results.length ? (index + 1) % results.length : 0);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setSelectedIndex((index) => results.length ? (index - 1 + results.length) % results.length : 0);
    } else if (event.key === 'Enter' && results[selectedIndex]) {
      event.preventDefault();
      selectResult(results[selectedIndex]);
    }
  };

  return { query, setQuery, results, selectedIndex, setSelectedIndex, loading, error, onKeyDown, selectResult };
}
