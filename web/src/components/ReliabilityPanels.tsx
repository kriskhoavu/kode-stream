import { Activity, AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, Circle, CircleX, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { useAuditEvents, useWorkspaceHealth } from '../features/reliability/hooks';

export function WorkspaceHealthPanel({ workspaceId, embedded = false }: { workspaceId: string; embedded?: boolean }) {
  const { health, loading, error, refresh } = useWorkspaceHealth(workspaceId);
  const [collapsed, setCollapsed] = useState(true);
  const healthStatus = loading ? undefined : error ? 'failed' : health?.summary;

  if (embedded) {
    return <div className="workspace-health-embedded" aria-label="Workspace health">
      <div className="workspace-health-embedded-summary">
        <span className={`health-summary-icon ${healthStatus ?? 'idle'}`}><HealthIcon status={healthStatus} /></span>
        <span>{loading ? 'Checking workspace...' : error ? error : health?.summary === 'ok' ? 'All workspace checks passed.' : health ? 'Workspace checks need attention.' : 'Workspace health is unavailable.'}</span>
        <button className="secondary" type="button" onClick={() => void refresh()}><RefreshCw size={14} /> Refresh health</button>
      </div>
      {!loading && !error && health && <div className="health-checks">
        {health.checks.map((check) => <div className={`health-check ${check.status}`} key={check.name} title={check.recoveryHint || check.message}>
          <HealthIcon status={check.status} />
          <span>{check.message}</span>
        </div>)}
      </div>}
    </div>;
  }

  return (
    <section className="health-panel" aria-label="Workspace health">
      <header>
        <button
          className="health-toggle"
          type="button"
          onClick={() => setCollapsed((value) => !value)}
          aria-expanded={!collapsed}
          aria-label={collapsed ? 'Expand workspace health' : 'Collapse workspace health'}
        >
          {collapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          <span className={`health-summary-icon ${healthStatus ?? 'idle'}`}>
            <HealthIcon status={healthStatus} />
          </span>
          <span>Health</span>
        </button>
        <button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh workspace health" title="Refresh health">
          <RefreshCw size={14} />
        </button>
      </header>
      {!collapsed && loading && <span className="reliability-muted">Checking workspace...</span>}
      {!collapsed && !loading && error && <span className="error">{error}</span>}
      {!collapsed && !loading && health && (
        <div className="health-checks">
          {health.checks.map((check) => (
            <div className={`health-check ${check.status}`} key={check.name} title={check.recoveryHint || check.message}>
              <HealthIcon status={check.status} />
              <span>{check.message}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

export function ActivityPanel({ workspaceId, onClose }: { workspaceId?: string; onClose: () => void }) {
  const { events, loading, error, refresh } = useAuditEvents(workspaceId, 12);
  return (
    <section className="activity-panel" aria-label="Recent activity">
      <header>
        <span><Activity size={15} /> Recent activity</span>
        <div>
          <button className="icon-button" type="button" onClick={() => void refresh()} aria-label="Refresh recent activity" title="Refresh activity"><RefreshCw size={14} /></button>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close recent activity">×</button>
        </div>
      </header>
      {loading && <span className="reliability-muted">Loading activity...</span>}
      {!loading && error && <span className="error">{error}</span>}
      {!loading && !error && events.length === 0 && <span className="reliability-muted">No recent activity.</span>}
      {!loading && events.length > 0 && (
        <div className="activity-list">
          {events.map((event) => (
            <div className={`activity-row ${event.status}`} key={event.id}>
              <HealthIcon status={event.status === 'success' ? 'ok' : event.status === 'blocked' ? 'warning' : 'failed'} />
              <span><strong>{operationLabel(event.operation)}</strong><small>{event.error || event.message}</small></span>
              <time dateTime={event.time}>{new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(new Date(event.time))}</time>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function HealthIcon({ status }: { status?: string }) {
  if (!status) return <Circle size={14} />;
  if (status === 'failed') return <CircleX size={14} />;
  if (status === 'warning') return <AlertTriangle size={14} />;
  return <CheckCircle2 size={14} />;
}

function operationLabel(operation: string) {
  return operation.replaceAll('_', ' ').replace(/^./, (value) => value.toUpperCase());
}
