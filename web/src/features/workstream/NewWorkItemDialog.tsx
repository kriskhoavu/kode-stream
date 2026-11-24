import { useRef, useState } from 'react';
import { Search, X } from 'lucide-react';
import { ModalLayer } from '../../components/ModalLayer';
import { useModalDialog } from '../../components/overlay';
import { StatusMenu } from '../../components/StatusMenu';
import { useBodyScrollLock } from '../../components/useBodyScrollLock';
import { MarkdownPreview } from '../content-viewer/renderers/MarkdownPreview';
import { notifyReliabilityChanged } from '../reliability/hooks';
import { sourcesShownIn } from '../../lib/sources';
import { api } from '../../shared/api';
import type { ItemSummary, JiraIssueState, WorkspaceConfig } from '../../lib/types';
import { emptyNewWorkItemDraft, jiraReadme, jiraTags, parseTags, type NewWorkItemDraft } from './newWorkItem';
import { useWorkspaceGuard } from './useWorkspaceGuard';

export function NewWorkItemDialog({ workspace, seed, onClose, onCreated }: {
  workspace: WorkspaceConfig;
  seed?: Partial<NewWorkItemDraft>;
  onClose: () => void;
  onCreated: (item: ItemSummary) => void;
}) {
  const [draft, setDraft] = useState<NewWorkItemDraft>(() => ({ ...emptyNewWorkItemDraft(), ...seed }));
  const [error, setError] = useState('');
  const [creating, setCreating] = useState(false);
  const [jiraLookup, setJiraLookup] = useState<JiraIssueState | null>(null);
  const [jiraLookupLoading, setJiraLookupLoading] = useState(false);
  const jiraLookupAbortRef = useRef<AbortController | null>(null);
  const track = useWorkspaceGuard(workspace.id);
  const close = () => {
    jiraLookupAbortRef.current?.abort();
    jiraLookupAbortRef.current = null;
    onClose();
  };
  const dialog = useModalDialog(close, !creating);
  useBodyScrollLock();

  const fetchJiraIssue = async () => {
    if (!draft.jiraKey.trim()) return;
		jiraLookupAbortRef.current?.abort();
		const controller = new AbortController();
		jiraLookupAbortRef.current = controller;
		const active = track(workspace.id);
    setJiraLookupLoading(true);
    setError('');
    try {
			const result = await api.workspaceJiraIssue(workspace.id, draft.jiraKey.trim(), controller.signal);
			if (!active()) return;
      setJiraLookup(result);
      if (result.state === 'available' && result.issue) {
        const issue = result.issue;
        const nextTags = jiraTags(issue).join(', ');
        // A fetch replaces the previous fetch's values, so the details match the preview.
        setDraft((draft) => ({
          ...draft,
          identifier: issue.key,
          title: issue.summary,
          owner: issue.assignee?.displayName || '',
          tags: nextTags
        }));
      }
		} catch (err) {
			if (controller.signal.aborted) return;
			if (!active()) return;
      setJiraLookup(null);
      setError(err instanceof Error ? err.message : 'Jira lookup failed');
    } finally {
		if (active()) setJiraLookupLoading(false);
    }
  };

  const create = async () => {
    setCreating(true);
    setError('');
    try {
      const source = draft.source || sourcesShownIn(workspace, 'workstream')[0] || 'plans';
      const jiraIssue = draft.origin === 'jira' && jiraLookup?.state === 'available' ? jiraLookup.issue : undefined;
      const result = await api.createItem({
        workspaceId: workspace.id,
        source,
        scope: draft.scope.trim() || source,
        identifier: draft.identifier.trim(),
        title: draft.title.trim() || undefined,
        status: draft.status,
        owner: draft.owner.trim() || undefined,
        tags: parseTags(draft.tags),
        jiraKey: jiraIssue?.key,
        initialReadme: jiraIssue ? jiraReadme(jiraIssue) : undefined
      });
      notifyReliabilityChanged();
      onCreated(result.item);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Item creation failed');
    } finally {
      setCreating(false);
    }
  };

  return (
    <ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !creating) close(); }}>
      <section ref={dialog.ref} className={draft.origin === 'jira' ? 'modal-panel new-work-item-modal jira-mode' : 'modal-panel new-work-item-modal'} role="dialog" aria-modal="true" aria-label="Create new work item">
        <header>
          <h2>New Work Item</h2>
				<button data-autofocus type="button" className="icon-button" onClick={() => {
				close();
          }}><X size={16} /></button>
        </header>
        <div className={draft.origin === 'jira' ? 'metadata-form jira-work-item-form' : 'metadata-form'}>
          <div className="segmented-control" role="group" aria-label="Item origin">
            <button
              type="button"
              className={draft.origin === 'blank' ? 'active' : ''}
              onClick={() => {
					jiraLookupAbortRef.current?.abort();
					jiraLookupAbortRef.current = null;
					setJiraLookupLoading(false);
					setDraft((draft) => ({ ...emptyNewWorkItemDraft(), source: draft.source, origin: 'blank' }));
                setJiraLookup(null);
                setError('');
              }}
            >
              Blank
            </button>
            <button
              type="button"
              className={draft.origin === 'jira' ? 'active' : ''}
              onClick={() => {
                setDraft((draft) => ({ ...draft, origin: 'jira' }));
                setError('');
              }}
            >
              From Jira
            </button>
          </div>
          {draft.origin === 'blank' ? (
            <>
              <label>Source<select value={draft.source || sourcesShownIn(workspace, 'workstream')[0] || ''} onChange={(event) => setDraft((draft) => ({ ...draft, source: event.target.value }))}>
                {sourcesShownIn(workspace, 'workstream').map((directory) => <option value={directory} key={directory}>{directory}</option>)}
              </select></label>
              <label>Item name<input value={draft.identifier} onChange={(event) => setDraft((draft) => ({ ...draft, identifier: event.target.value }))} placeholder="Any item name" /></label>
              <label>Status<StatusMenu value={draft.status} onChange={(status) => setDraft((draft) => ({ ...draft, status }))} /></label>
            </>
          ) : (
            <div className="jira-create-layout">
              <section className="jira-ticket-column">
                <div className="jira-column-heading">
                  <strong>Jira ticket</strong>
                  <span>Fetch the source issue and review its context.</span>
                </div>
                <label>Source<select value={draft.source || sourcesShownIn(workspace, 'workstream')[0] || ''} onChange={(event) => setDraft((draft) => ({ ...draft, source: event.target.value }))}>
                  {sourcesShownIn(workspace, 'workstream').map((directory) => <option value={directory} key={directory}>{directory}</option>)}
                </select></label>
                <div className="jira-intake">
                  <label>Jira key<input
                    value={draft.jiraKey}
                    onChange={(event) => {
                      setDraft((draft) => ({ ...draft, jiraKey: event.target.value.toUpperCase() }));
                      setJiraLookup(null);
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== 'Enter' || jiraLookupLoading || !draft.jiraKey.trim()) return;
                      event.preventDefault();
                      void fetchJiraIssue();
                    }}
                    placeholder="PM-025"
                  /></label>
                  <button type="button" className="secondary" disabled={jiraLookupLoading || !draft.jiraKey.trim()} onClick={() => void fetchJiraIssue()}>
                    <Search size={15} />
                    {jiraLookupLoading ? 'Fetching...' : 'Fetch Jira'}
                  </button>
                </div>
                {jiraLookup && (
                  <section className={jiraLookup.state === 'available' ? 'jira-issue-preview' : 'jira-issue-preview warning'} aria-label="Jira issue preview">
                    {jiraLookup.issue ? (
                      <>
                        <header className="jira-issue-preview-header">
                          <strong>{jiraLookup.issue.key}: {jiraLookup.issue.summary}</strong>
                          {jiraLookup.issue.browserUrl && <a href={jiraLookup.issue.browserUrl} target="_blank" rel="noreferrer">Open Jira</a>}
                        </header>
                        <div className="jira-issue-preview-body">
                          <strong className="jira-issue-preview-label">Description</strong>
                          {jiraLookup.issue.description
                            ? <div className="jira-issue-preview-description"><MarkdownPreview content={jiraLookup.issue.description} /></div>
                            : <p className="jira-issue-preview-description empty">This Jira ticket has no description.</p>}
                        </div>
                        <dl className="jira-issue-preview-meta">
                          <div><dt>Type</dt><dd>{jiraLookup.issue.issueType || 'Issue'}</dd></div>
                          <div><dt>Status</dt><dd>{jiraLookup.issue.status || 'Unknown'}</dd></div>
                          <div><dt>Assignee</dt><dd>{jiraLookup.issue.assignee?.displayName || 'Unassigned'}</dd></div>
                          {jiraLookup.issue.priority && <div><dt>Priority</dt><dd>{jiraLookup.issue.priority}</dd></div>}
                        </dl>
                        {jiraLookup.issue.attachments.length > 0 && <small>{jiraLookup.issue.attachments.length} attachment{jiraLookup.issue.attachments.length === 1 ? '' : 's'} referenced</small>}
                      </>
                    ) : (
                      <>
                        <strong>{jiraLookup.message || jiraLookup.state}</strong>
                        {jiraLookup.recoveryHint && <span>{jiraLookup.recoveryHint}</span>}
                      </>
                    )}
                  </section>
                )}
              </section>
              <section className="jira-details-column">
                <div className="jira-column-heading">
                  <strong>Work item details</strong>
                  <span>Review the values imported from Jira before creation.</span>
                </div>
                <div className="jira-field-row">
                  <label>Item name<input value={draft.identifier} onChange={(event) => setDraft((draft) => ({ ...draft, identifier: event.target.value }))} placeholder="Jira key or local item name" /></label>
                  <label>Status<StatusMenu value={draft.status} onChange={(status) => setDraft((draft) => ({ ...draft, status }))} /></label>
                </div>
                <label>Title<input value={draft.title} onChange={(event) => setDraft((draft) => ({ ...draft, title: event.target.value }))} placeholder="Jira summary" /></label>
                <label>Owner<input value={draft.owner} onChange={(event) => setDraft((draft) => ({ ...draft, owner: event.target.value }))} placeholder="Assignee" /></label>
                <label>Tags<input value={draft.tags} onChange={(event) => setDraft((draft) => ({ ...draft, tags: event.target.value }))} placeholder="story, priority-high" /></label>
              </section>
            </div>
          )}
        </div>
        {error && <p className="error">{error}</p>}
        <footer className="modal-actions">
				<button type="button" className="ghost" onClick={close}>Cancel</button>
          <button
            type="button"
            className="primary"
            disabled={creating || !draft.identifier.trim() || (draft.origin === 'jira' && jiraLookup?.state !== 'available')}
            onClick={create}
          >
            {creating ? 'Creating...' : 'Create Item'}
          </button>
        </footer>
      </section>
    </div></ModalLayer>
  );
}
