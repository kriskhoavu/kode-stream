import { Link2 } from 'lucide-react';

export function CloudAgentRegistration({ status, deepLink, busy, onConnect }: { status: string; deepLink: string; busy: boolean; onConnect: () => void }) {
  return <section className="cloud-agent-registration" aria-label="Cloud Agent connection">
    <div className="metadata-callout">
      <strong>Connect Cloud Agent</strong>
      <span>Use the local agent to choose a repository, validate the Git root, scan metadata, and publish the workspace to Cloud.</span>
    </div>
    <dl className="workspace-overview-grid">
      <div><dt>Status</dt><dd>{cloudAgentStatusLabel(status)}</dd></div>
      <div><dt>Install</dt><dd>macOS Homebrew first; Windows and Linux planned</dd></div>
      <div><dt>Network</dt><dd>Requires outbound HTTPS WebSocket access to Cloud</dd></div>
    </dl>
    <div className="repo-row-actions">
      <button className="primary" type="button" onClick={onConnect} disabled={busy}><Link2 size={16} /> {busy ? 'Connecting...' : status === 'offline' || status === 'stale' ? 'Reconnect agent' : 'Connect local workspace'}</button>
      {deepLink && <a className="secondary button-link" href={deepLink}>Open deep link again</a>}
    </div>
  </section>;
}

export function cloudAgentStatusLabel(status: string) {
  switch (status) {
    case 'connected': return 'Connected';
    case 'connecting': return 'Connecting';
    case 'stale': return 'Stale';
    case 'unsupported': return 'Agent not installed or blocked';
    default: return 'Offline';
  }
}
