import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { JiraIssueState } from '../../lib/types';

export function useJiraIssue(itemId: string) {
  const [result, setResult] = useState<JiraIssueState | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const requestRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const load = useCallback(async (refresh = false) => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const request = ++requestRef.current;
    refresh ? setRefreshing(true) : setLoading(true); setError('');
    try { const next = refresh ? await api.refreshJiraIssue(itemId, controller.signal) : await api.jiraIssue(itemId, controller.signal); if (request === requestRef.current) setResult(next); }
    catch (caught) { if (request === requestRef.current && !controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Jira details are unavailable.'); }
    finally { if (request === requestRef.current) { setLoading(false); setRefreshing(false); } }
  }, [itemId]);
  useEffect(() => { setResult(null); void load(); return () => abortRef.current?.abort(); }, [load]);
  return { result, loading, refreshing, error, refresh: () => load(true) };
}
