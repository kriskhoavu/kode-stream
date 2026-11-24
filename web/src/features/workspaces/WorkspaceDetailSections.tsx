import { ExternalLink, Pencil, SlidersHorizontal } from 'lucide-react';
import type { SourceSurface, WorkspaceConfig } from '../../lib/types';
import { sourcePaths, sourceSurfaces, workspaceSources } from '../../lib/sources';
import { HarnessSettingsSection } from '../workstream/HarnessSettingsSection';
import { BranchField, JiraConnectionFields, KnowledgeRoots, KnowledgeSettingsFields, RuntimeSettingsFields, SourcesField } from './workspaceFields';
import { integrationSaveLabel, isIntegrationEditSection } from './workspaceDetailTabs';
import type { useWorkspaceEdit } from './useWorkspaceEdit';

type WorkspaceEdit = ReturnType<typeof useWorkspaceEdit>;

export const sourceSurfaceLabels: Record<SourceSurface, string> = { workstream: 'Workstream', knowledge: 'Knowledge' };

export function WorkspaceGeneralSection({ repo, edit, busy, onReveal }: { repo: WorkspaceConfig; edit: WorkspaceEdit; busy: boolean; onReveal: (path: string) => void }) {
  return (
    <>
      {edit.editingId === repo.id && edit.editingSection === 'general' ? <>
        <div className="repo-edit-form">
          <label className="repo-field">Workspace Name<input value={edit.editDraft.name} onChange={(event) => edit.setEditDraft({ ...edit.editDraft, name: event.target.value })} /></label>
          <label className="repo-field">Local Path<input value={edit.editDraft.path} disabled={Boolean(repo.clonePathManaged)} onChange={(event) => edit.setEditDraft({ ...edit.editDraft, path: event.target.value })} />{repo.clonePathManaged && <small>Managed clone paths are immutable; remove and register the workspace again to use another checkout.</small>}</label>
          <BranchField value={edit.editDraft.baselineBranch} onChange={(value) => edit.setEditDraft({ ...edit.editDraft, baselineBranch: value })} />
        </div>
        <div className="repo-row-actions">
          <button className="secondary" type="button" onClick={edit.discardEdit} disabled={busy}>Cancel</button>
          <button className="primary" type="button" onClick={() => edit.saveEdit(repo)} disabled={busy}>Save general settings</button>
        </div>
      </> : <>
        <dl className="workspace-overview-grid">
          <div><dt>Location</dt><dd>{repo.location === 'cloud_agent' ? (repo.localRootLabel || 'Cloud Agent workspace') : repo.location === 'cloud_remote_snapshot' ? 'Remote Snapshot (read-only)' : <button className="repo-path-link" type="button" onClick={() => onReveal(repo.path)} title={repo.path}>{repo.path}</button>}</dd></div>
          <div><dt>Base branch</dt><dd>{repo.baselineBranch}</dd></div>
          <div><dt>Registration</dt><dd>{repo.location === 'cloud_agent' ? 'Cloud Agent' : repo.location === 'cloud_remote_snapshot' ? 'Remote Snapshot' : repo.registrationMode === 'remote_clone' ? 'Remote clone' : repo.registrationMode === 'existing_workspace' ? 'Imported workspace' : 'Local folder'}</dd></div>
          {repo.agentId && <div><dt>Agent</dt><dd>{repo.agentId}</dd></div>}
          {repo.location === 'cloud_remote_snapshot' && <><div><dt>Repository</dt><dd>{repo.providerRepository || 'Not selected'}</dd></div><div><dt>Ref</dt><dd>{repo.selectedRef || 'Not selected'}</dd></div><div><dt>Commit</dt><dd>{repo.resolvedCommitSha || 'Resolved when opened'}</dd></div></>}
          {repo.scanStatus && <div><dt>Scan</dt><dd>{repo.scanStatus}</dd></div>}
          {repo.remoteUrl && <div><dt>Remote URL</dt><dd>{repo.remoteUrl}</dd></div>}
          <div><dt>Created</dt><dd>{repo.createdAt ? new Date(repo.createdAt).toLocaleString() : 'Unknown'}</dd></div>
        </dl>
        <div className="repo-row-actions workspace-detail-actions">
        {repo.location !== 'cloud_agent' && repo.location !== 'cloud_remote_snapshot' && <button className="secondary" type="button" onClick={() => onReveal(repo.path)}><ExternalLink size={16} /> Reveal folder</button>}
        {repo.location !== 'cloud_agent' && repo.location !== 'cloud_remote_snapshot' && <button className="primary" type="button" onClick={() => edit.startEdit(repo, 'general')}><Pencil size={16} /> Edit general</button>}
        </div>
        {repo.location === 'cloud_remote_snapshot' && <p className="workspace-inline-notice">This is a read-only provider snapshot. Run local Git or terminal work on your machine; Cloud cannot launch or observe that terminal.</p>}
      </>}
    </>
  );
}

export function WorkspaceSourcesSection({ repo, edit, busy, canWrite, onShownIn, onConfigure }: { repo: WorkspaceConfig; edit: WorkspaceEdit; busy: boolean; canWrite: boolean; onShownIn: (path: string, surface: SourceSurface) => void; onConfigure: (path: string) => void }) {
  return (
    <>
      {edit.editingId === repo.id && edit.editingSection === 'sources' ? <>
        <SourcesField value={edit.editDraft.sources} onChange={(value) => edit.setEditDraft({ ...edit.editDraft, sources: value })} />
        <div className="repo-row-actions"><button className="secondary" type="button" onClick={edit.discardEdit}>Cancel</button><button className="primary" type="button" onClick={() => edit.saveEdit(repo)}>Save sources</button></div>
      </> : <>
        <div className="workspace-source-list">
          {workspaceSources(repo).map((source) => <div className="workspace-source-row" key={source.path}>
            <div><strong>{source.path}</strong><span>Workspace source directory</span></div>
            <fieldset className="workspace-source-shown-in" aria-label={`Shown in for ${source.path}`}>
              <legend>Shown in</legend>
              {sourceSurfaces.map((surface) => <label key={surface}>
                <input type="checkbox" checked={source.shownIn.includes(surface)} disabled={!canWrite || busy || (source.shownIn.length === 1 && source.shownIn[0] === surface)} onChange={() => onShownIn(source.path, surface)} />
                {sourceSurfaceLabels[surface]}
              </label>)}
            </fieldset>
            <button className="secondary" type="button" onClick={() => onConfigure(source.path)} disabled={!canWrite}><SlidersHorizontal size={15} /> Configure structure</button>
          </div>)}
          {sourcePaths(repo).length === 0 && <div className="empty-inline">No sources configured.</div>}
        </div>
        <div className="workspace-detail-actions"><button className="primary" type="button" onClick={() => edit.startEdit(repo, 'sources')} disabled={!canWrite}><Pencil size={16} /> Edit sources</button></div>
      </>}
    </>
  );
}

export function WorkspaceIntegrations({ repo, edit, busy, showHarness, settingsKey }: { repo: WorkspaceConfig; edit: WorkspaceEdit; busy: boolean; showHarness: boolean; settingsKey: string }) {
  return (
    <>
      {edit.editingId === repo.id && isIntegrationEditSection(edit.editingSection) ? <>
        {edit.editingSection === 'jira' && <JiraConnectionFields value={edit.editDraft.jira} onChange={(value) => edit.setEditDraft({ ...edit.editDraft, jira: value })} workspaceId={repo.id} />}
        {edit.editingSection === 'knowledge' && <KnowledgeSettingsFields value={edit.editDraft.knowledge} onChange={(value) => edit.setEditDraft({ ...edit.editDraft, knowledge: value })} />}
        {edit.editingSection === 'runtime' && <RuntimeSettingsFields value={edit.editDraft.runtime} onChange={(value) => edit.setEditDraft({ ...edit.editDraft, runtime: value })} />}
        <div className="repo-row-actions"><button className="secondary" type="button" onClick={edit.discardEdit} disabled={busy}>Cancel</button><button className="primary" type="button" onClick={() => edit.saveEdit(repo)} disabled={busy}>{integrationSaveLabel(edit.editingSection)}</button></div>
      </> : <><div className="workspace-integration-card">
        <div><strong>Jira</strong><span>{repo.jira ? `${repo.jira.projectKey} · ${repo.jira.deploymentType === 'cloud' ? 'Cloud' : 'Server / Data Center'}` : 'Not configured'}</span></div>
        <button className="secondary" type="button" onClick={() => edit.startEdit(repo, 'jira')}>{repo.jira ? 'Configure' : 'Connect Jira'}</button>
      </div><div className="workspace-integration-card"><div><strong>Knowledge</strong><span>{repo.knowledge?.enabled === false ? 'Disabled' : repo.knowledge?.enrichExecutable ? `Enabled · ${repo.knowledge.enrichExecutable}` : 'Enabled · Enrichment not configured'}</span><KnowledgeRoots workspaceId={repo.id} enabled={repo.knowledge?.enabled !== false} /></div><button className="secondary" type="button" onClick={() => edit.startEdit(repo, 'knowledge')}>Configure</button></div><div className="workspace-integration-card"><div><strong>Runtime and verification</strong><span>{repo.runtime ? `${repo.runtime.type} · ${repo.runtime.rebuildPolicy ?? 'changed-only'}` : 'Not configured'}</span></div><button className="secondary" type="button" onClick={() => edit.startEdit(repo, 'runtime')}>{repo.runtime ? 'Configure' : 'Set runtime'}</button></div>
      {showHarness && <HarnessSettingsSection key={settingsKey} workspaceId={repo.id} part="sources" />}</>}
    </>
  );
}
