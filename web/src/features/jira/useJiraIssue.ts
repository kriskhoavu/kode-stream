import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { JiraIssueState } from '../../lib/types';

/** Where a Jira ticket is looked up: through a Kode Stream item, or by key in a workspace. */
export type JiraIssueSource = { itemId: string } | { workspaceId: string; issueKey: string };

function load(source: JiraIssueSource, refresh: boolean, signal: AbortSignal): Promise<JiraIssueState> {
  if ('itemId' in source) return refresh ? api.refreshJiraIssue(source.itemId, signal) : api.jiraIssue(source.itemId, signal);
  return refresh ? api.refreshWorkspaceJiraIssue(source.workspaceId, source.issueKey, signal) : api.workspaceJiraIssue(source.workspaceId, source.issueKey, signal);
}

export function useJiraIssue(target: string | JiraIssueSource) {
  const source: JiraIssueSource = typeof target === 'string' ? { itemId: target } : target;
  const key = 'itemId' in source ? `item:${source.itemId}` : `workspace:${source.workspaceId}:${source.issueKey}`;
  const [result, setResult] = useState<JiraIssueState | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const requestRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const sourceRef = useRef(source);
  sourceRef.current = source;
  const run = useCallback(async (refresh = false) => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const request = ++requestRef.current;
    refresh ? setRefreshing(true) : setLoading(true); setError('');
    try { const next = await load(sourceRef.current, refresh, controller.signal); if (request === requestRef.current) setResult(next); }
    catch (caught) { if (request === requestRef.current && !controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'Jira details are unavailable.'); }
    finally { if (request === requestRef.current) { setLoading(false); setRefreshing(false); } }
    // `key` names the source; the ref holds its current value.
  }, [key]);
  useEffect(() => { setResult(null); void run(); return () => abortRef.current?.abort(); }, [run]);
  return { result, loading, refreshing, error, refresh: () => run(true) };
}
