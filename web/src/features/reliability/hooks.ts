import { useCallback, useEffect, useState } from 'react';
import { api } from '../../lib/api';
import type { AuditEvent, WorkspaceHealth } from '../../lib/types';

const reliabilityChangedEvent = 'kode-stream:reliability-changed';

export function notifyReliabilityChanged() {
  window.dispatchEvent(new Event(reliabilityChangedEvent));
}

export function useWorkspaceHealth(workspaceId?: string) {
  const [health, setHealth] = useState<WorkspaceHealth | null>(null);
  const [loading, setLoading] = useState(Boolean(workspaceId));
  const [error, setError] = useState('');

  const refresh = useCallback(async () => {
    if (!workspaceId) {
      setHealth(null);
      setLoading(false);
      setError('');
      return;
    }
    setLoading(true);
    setError('');
    try {
      setHealth(await api.workspaceHealth(workspaceId));
    } catch (caught) {
      setHealth(null);
      setError(caught instanceof Error ? caught.message : 'Workspace health is unavailable.');
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useReliabilityRefresh(refresh);
  return { health, loading, error, refresh };
}

export function useAuditEvents(workspaceId?: string, limit = 20) {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setEvents(await api.auditEvents({ workspaceId, limit }));
    } catch (caught) {
      setEvents([]);
      setError(caught instanceof Error ? caught.message : 'Recent activity is unavailable.');
    } finally {
      setLoading(false);
    }
  }, [workspaceId, limit]);

  useReliabilityRefresh(refresh);
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
