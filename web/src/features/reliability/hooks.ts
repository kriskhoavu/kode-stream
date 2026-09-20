import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { AuditEvent, WorkspaceHealth } from '../../lib/types';

const reliabilityChangedEvent = 'kode-stream:reliability-changed';

export function notifyReliabilityChanged() {
  window.dispatchEvent(new Event(reliabilityChangedEvent));
}

export function useWorkspaceHealth(workspaceId?: string) {
  const [health, setHealth] = useState<WorkspaceHealth | null>(null);
  const [loading, setLoading] = useState(Boolean(workspaceId));
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | undefined>(undefined);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    if (!workspaceId) {
      setHealth(null);
      setLoading(false);
      setError('');
      return;
    }
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError('');
    try {
      const nextHealth = await api.workspaceHealth(workspaceId, controller.signal);
      if (!controller.signal.aborted) setHealth(nextHealth);
    } catch (caught) {
      if (!controller.signal.aborted) {
        setHealth(null);
        setError(caught instanceof Error ? caught.message : 'Workspace health is unavailable.');
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
      if (abortRef.current === controller) abortRef.current = undefined;
    }
  }, [workspaceId]);

  useReliabilityRefresh(refresh);
  useEffect(() => () => abortRef.current?.abort(), []);
  return { health, loading, error, refresh };
}

export function useAuditEvents(workspaceId?: string, limit = 20) {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | undefined>(undefined);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError('');
    try {
      const nextEvents = await api.auditEvents({ workspaceId, limit, signal: controller.signal });
      if (!controller.signal.aborted) setEvents(nextEvents);
    } catch (caught) {
      if (!controller.signal.aborted) {
        setEvents([]);
        setError(caught instanceof Error ? caught.message : 'Recent activity is unavailable.');
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
      if (abortRef.current === controller) abortRef.current = undefined;
    }
  }, [workspaceId, limit]);

  useReliabilityRefresh(refresh);
  useEffect(() => () => abortRef.current?.abort(), []);
  return { events, loading, error, refresh };
}

function useReliabilityRefresh(refresh: () => Promise<void>) {
  useEffect(() => {
    void refresh();
    const onChanged = () => void refresh();
    window.addEventListener(reliabilityChangedEvent, onChanged);
    return () => window.removeEventListener(reliabilityChangedEvent, onChanged);
  }, [refresh]);
}
