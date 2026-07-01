import { ExternalLink, FileDown, RefreshCw } from 'lucide-react';
import { api } from '../../lib/api';
import { useJiraIssue } from './useJiraIssue';

export function JiraItemPanel({ itemId }: { itemId: string }) {
  const jira = useJiraIssue(itemId);
  if (jira.loading) return <div className="jira-panel-state" role="status">Loading Jira ticket...</div>;
  if (jira.error) return <div className="jira-panel-state error" role="alert">{jira.error}</div>;
  if (!jira.result) return null;
  if (jira.result.state !== 'available' || !jira.result.issue) return <div className="jira-panel-state"><strong>{stateTitle(jira.result.state)}</strong><span>{jira.result.message}</span>{jira.result.recoveryHint && <small>{jira.result.recoveryHint}</small>}<button className="secondary" type="button" disabled={jira.refreshing} onClick={() => void jira.refresh()}><RefreshCw size={14} /> Refresh</button></div>;
  const issue = jira.result.issue;
  return <div className="jira-item-panel">
    <div className="jira-panel-heading"><span className="status-badge">{issue.status}</span><button className="icon-button" type="button" aria-label="Refresh Jira ticket" disabled={jira.refreshing} onClick={() => void jira.refresh()}><RefreshCw size={14} /></button></div>
    <a href={issue.browserUrl} target="_blank" rel="noreferrer"><strong>{issue.key}</strong> <ExternalLink size={13} /></a>
    <h3>{issue.summary}</h3>
    <dl><dt>Type</dt><dd>{issue.issueType || '—'}</dd><dt>Priority</dt><dd>{issue.priority || '—'}</dd><dt>Assignee</dt><dd>{issue.assignee?.displayName || 'Unassigned'}</dd><dt>Reporter</dt><dd>{issue.reporter?.displayName || '—'}</dd><dt>Updated</dt><dd>{formatDate(issue.updatedAt)}</dd></dl>
    {issue.labels.length > 0 && <div className="jira-labels">{issue.labels.map((label) => <span key={label}>{label}</span>)}</div>}
    <section><h4>Description</h4><p className="jira-description">{issue.description || 'No description.'}</p></section>
    <section><h4>Attachments</h4>{issue.attachments.length === 0 ? <p>No attachments.</p> : <ul className="jira-attachments">{issue.attachments.map((attachment) => <li key={attachment.id}><span><strong>{attachment.filename}</strong><small>{attachment.mediaType || 'file'} · {formatBytes(attachment.sizeBytes)}</small></span><a className="icon-button" href={api.jiraAttachmentURL(itemId, attachment.id)} target="_blank" rel="noreferrer" aria-label={`Open ${attachment.filename}`}><FileDown size={15} /></a></li>)}</ul>}</section>
  </div>;
}

function stateTitle(state: string) { return ({ not_configured:'Jira not configured', invalid_identifier:'Not a Jira ticket', project_mismatch:'Different Jira project', not_found:'No Jira ticket', authentication_failed:'Jira authentication failed', forbidden:'Jira access forbidden', unavailable:'Jira unavailable' } as Record<string,string>)[state] ?? 'Jira unavailable'; }
function formatDate(value?: string) { if (!value) return '—'; const date=new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString(); }
function formatBytes(value: number) { if (value < 1024) return `${value} B`; if (value < 1024*1024) return `${(value/1024).toFixed(1)} KB`; return `${(value/(1024*1024)).toFixed(1)} MB`; }
