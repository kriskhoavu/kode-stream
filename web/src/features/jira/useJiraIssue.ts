import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { JiraIssueState } from '../../lib/types';

export function useJiraIssue(itemId: string) {
  const [result, setResult] = useState<JiraIssueState | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const requestRef = useRef(0);
  const load = useCallback(async (refresh = false) => {
    const request = ++requestRef.current;
    refresh ? setRefreshing(true) : setLoading(true); setError('');
    try { const next = refresh ? await api.refreshJiraIssue(itemId) : await api.jiraIssue(itemId); if (request === requestRef.current) setResult(next); }
    catch (caught) { if (request === requestRef.current) setError(caught instanceof Error ? caught.message : 'Jira details are unavailable.'); }
    finally { if (request === requestRef.current) { setLoading(false); setRefreshing(false); } }
  }, [itemId]);
  useEffect(() => { setResult(null); void load(); }, [load]);
  return { result, loading, refreshing, error, refresh: () => load(true) };
}
