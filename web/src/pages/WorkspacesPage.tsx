import { type DragEvent, type FormEvent, type RefObject, useEffect, useRef, useState } from 'react';
import { ArrowLeft, CheckCircle2, Download, ExternalLink, FolderGit2, FolderOpen, HardDrive, Plus, RotateCw, Trash2, Upload, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { WorkspaceHealthPanel } from '../components/ReliabilityPanels';
import { api } from '../shared/api';
import type { RuntimeContext, WorkspaceConfig, SourceSurface, SystemConfigPaths } from '../lib/types';
import { workspaceSources } from '../lib/sources';
import { lastPathSegment } from '../features/workspaces/sourceSettings';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { WorkspaceList } from '../features/workspaces/WorkspaceManagerShell';
import { WorkspaceOwnSettings } from '../features/workspaces/WorkspaceOwnSettings';
import { SettingsFileDialog, type SettingsFileMode } from '../features/settings/SettingsFileDialog';
import { useModalDialog } from '../components/overlay';
import { ModalLayer } from '../components/ModalLayer';
import { useWorkspaceRegistrationDraft } from '../features/workspaces/useWorkspaceRegistrationDraft';
import { useWorkspaceImport } from '../features/workspaces/useWorkspaceImport';
import { WorkspaceImportReview, WorkspaceImportResults } from '../features/workspaces/workspaceImportReview';
import { CloudAgentRegistration } from '../features/workspaces/CloudAgentRegistration';
import { SourceSettingsDialog } from '../features/workspaces/SourceSettingsDialog';
import { useWorkspaceEdit } from '../features/workspaces/useWorkspaceEdit';
import { sourceSurfaceLabels, WorkspaceGeneralSection, WorkspaceIntegrations, WorkspaceSourcesSection } from '../features/workspaces/WorkspaceDetailSections';
import { type WorkspaceNotice, WorkspaceNoticePanel, scanNotice, scanSummary, scanWarnings } from '../features/workspaces/workspaceNotice';
import { errorMessage, buildWorkspaceInput, normalizeKnowledgeSettings, inferWorkspaceNameFromRemoteURL, workspaceRemovalMessage, pathFromDrop, settingsEditorFromResult, toggleSourceSurface, type SettingsEditorState } from '../features/workspaces/workspaceDraft';
import { SourcesField, BranchField, JiraConnectionFields, OverviewSection } from '../features/workspaces/workspaceFields';
import { type WorkspaceDetailTab, detailTabs, detailTabLabels, type OverviewSectionKey, workspaceDetailSubtitle, isIntegrationEditSection } from '../features/workspaces/workspaceDetailTabs';



const maxRegistrationLogChars = 256 * 1024;
const localRuntimeContext: RuntimeContext = {
  mode: 'local',
  role: 'admin',
  capabilities: { read: true, write: true, workspace_registration: true, git: true, system: true, terminal: true, ai: true, runtime: true, verification: true },
  agent: { available: true, status: 'local' }
};
export function WorkspacesPage({ workspaces, runtimeContext = localRuntimeContext, onChanged, showHarness = true }: { workspaces: WorkspaceConfig[]; runtimeContext?: RuntimeContext; onChanged: () => void | Promise<void>; /** The harness's settings are absent on the extension surface. */ showHarness?: boolean }) {
  const [systemConfig, setSystemConfig] = useState<SystemConfigPaths | null>(null);
  const [notice, setNotice] = useState<WorkspaceNotice | null>(null);
  const [settingsFile, setSettingsFile] = useState<SettingsFileMode | null>(null);
  const [settingsImports, setSettingsImports] = useState(0);
  const [registrationLog, setRegistrationLog] = useState('');
  const [registrationLogOpen, setRegistrationLogOpen] = useState(false);
  const [pendingOperations, setPendingOperations] = useState<string[]>([]);
  const [pathDragging, setPathDragging] = useState(false);
  const [selectedWorkspaceIds, setSelectedWorkspaceIds] = useState<string[]>([]);
  const [workspacesToRemove, setWorkspacesToRemove] = useState<WorkspaceConfig[] | null>(null);
  const [settingsEditor, setSettingsEditor] = useState<SettingsEditorState | null>(null);
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState(workspaces[0]?.id ?? '');
  const [workspaceQuery, setWorkspaceQuery] = useState('');
  const [bulkMode, setBulkMode] = useState(false);
  const [registrationOpen, setRegistrationOpen] = useState(false);
  const [agentDeepLink, setAgentDeepLink] = useState('');
  const [agentStatusLabel, setAgentStatusLabel] = useState('');
  const [activeDetailTab, setActiveDetailTab] = useState<WorkspaceDetailTab>('overview');
  const [collapsedOverviewSections, setCollapsedOverviewSections] = useState<Record<OverviewSectionKey, boolean>>({ general: false, sources: false });
  const draft = useWorkspaceRegistrationDraft();
  const { name, setName, registrationMode, setRegistrationMode, path, setPath, remoteUrl, setRemoteUrl, cloneRoot, setCloneRoot, baselineBranch, setBaselineBranch, sources, setSources, jira, setJira, registrationStep, setRegistrationStep, registrationNameEdited, setRegistrationNameEdited } = draft;
  const selectAllRef = useRef<HTMLInputElement | null>(null);
  const { importSourcePath, importState, importPreview, importSelection, importResults, importError, importConfirmOpen, setImportConfirmOpen, importStatusRef, resetWorkspaceImport, changeImportSourcePath, toggleImportCandidate, toggleAllImportCandidates, importExistingWorkspaces, browseImportSource, previewExistingWorkspaces } = useWorkspaceImport({ onChanged });
  const registrationController = useRef<AbortController | null>(null);
  const registrationDialog = useModalDialog(closeRegistration, true, registrationOpen);

  const selectedWorkspaces = workspaces.filter((workspace) => selectedWorkspaceIds.includes(workspace.id));
  const allSelected = workspaces.length > 0 && selectedWorkspaces.length === workspaces.length;
const busy = pendingOperations.length > 0;
  const cloudMode = runtimeContext.mode === 'cloud';
  const registrationLocationReady = registrationMode === 'local_path' ? Boolean(path.trim()) : registrationMode === 'remote_clone' ? Boolean(remoteUrl.trim()) : Boolean(importSourcePath.trim());
  const operationBusy = (operation: string) => pendingOperations.includes(operation);
  const canRunWorkspaceCommands = !cloudMode || runtimeContext.agent.available;
  const canScan = canRunWorkspaceCommands && runtimeContext.capabilities.verification;
  const canWrite = !cloudMode || runtimeContext.capabilities.write;
  const canRegisterWorkspace = !cloudMode || runtimeContext.capabilities.workspace_registration;
  const setBusy = (pending: boolean, operation = 'workspace-form') => {
    setPendingOperations((current) => pending
      ? current.includes(operation) ? current : [...current, operation]
      : current.filter((item) => item !== operation));
  };
  const edit = useWorkspaceEdit({ setBusy, onNotice: setNotice, onChanged });
  const { editingId, editingSection, discardEdit, confirmDiscardEdit } = edit;


  useEffect(() => {
    setSelectedWorkspaceIds((current) => current.filter((id) => workspaces.some((workspace) => workspace.id === id)));
    setSelectedWorkspaceId((current) => workspaces.some((workspace) => workspace.id === current) ? current : workspaces[0]?.id ?? '');
  }, [workspaces]);

  useEffect(() => {
    if (!selectAllRef.current) return;
    selectAllRef.current.indeterminate = selectedWorkspaces.length > 0 && !allSelected;
  }, [allSelected, selectedWorkspaces.length]);

  useEffect(() => {
    const controller = new AbortController();
    void api.systemConfigPaths(controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      setSystemConfig(result);
      setCloneRoot((current) => current || result.cloneRootDir);
    }).catch(() => undefined);
    return () => controller.abort();
  }, []);

  useEffect(() => () => registrationController.current?.abort(), []);


  const submit = async (event: FormEvent) => {
    event.preventDefault();
	if (registrationMode === 'existing_workspace') {
		void previewExistingWorkspaces();
		return;
	}
    setBusy(true);
    setNotice(null);
    setRegistrationLog('');
    setRegistrationLogOpen(false);
    try {
      const input = buildWorkspaceInput({ name, registrationMode, path, remoteUrl, cloneRoot, baselineBranch, sources, jira });
      const result = registrationMode === 'remote_clone'
        ? await api.createWorkspaceStream(input, (chunk) => {
          setRegistrationLog((current) => `${current}${chunk}`.slice(-maxRegistrationLogChars));
          setRegistrationLogOpen(true);
        }, (registrationController.current = new AbortController()).signal)
        : await api.createWorkspace(input);
      setNotice({ tone: 'success', title: 'Workspace registered', details: [name || 'New workspace'] });
      if (result.operationLog.trim()) {
        setRegistrationLog(result.operationLog);
        setRegistrationLogOpen(true);
      }
      setName('');
      setRegistrationMode('local_path');
      setPath('');
      setRemoteUrl('');
      setCloneRoot(systemConfig?.cloneRootDir ?? '');
      setBaselineBranch('master');
      setSources('');
      setJira(null);
      setRegistrationStep(1);
      setRegistrationNameEdited(false);
      setRegistrationOpen(false);
      setSelectedWorkspaceId(result.workspace.id);
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: registrationMode === 'remote_clone' ? 'Remote workspace registration failed' : 'Local workspace registration failed', details: [errorMessage(err)] });
      if (err instanceof Error && 'operationLog' in err && typeof (err as { operationLog?: string }).operationLog === 'string' && (err as { operationLog?: string }).operationLog?.trim()) {
        setRegistrationLog((err as { operationLog?: string }).operationLog ?? '');
        setRegistrationLogOpen(true);
      }
    } finally {
      registrationController.current = null;
      setBusy(false);
    }
  };

  const scan = async (repo: WorkspaceConfig) => {
    const operation = `scan:${repo.id}`;
    setBusy(true, operation);
    setNotice({ tone: 'info', title: `Scanning ${repo.name}` });
    try {
      const result = await api.scan(repo.id);
      notifyReliabilityChanged();
      setNotice(scanNotice(repo, result));
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: `Scan failed for ${repo.name}`, details: [errorMessage(err)] });
    } finally {
      setBusy(false, operation);
    }
  };

  const scanAll = async () => {
    if (workspaces.length === 0) return;
    setBusy(true, 'scan-all');
    setNotice({ tone: 'info', title: `Scanning ${workspaces.length} workspace${workspaces.length === 1 ? '' : 's'}` });
    const details: string[] = [];
    let failures = 0;
    try {
      for (const repo of workspaces) {
        try {
          const result = await api.scan(repo.id);
          details.push(scanSummary(repo, result));
          scanWarnings(result).slice(0, 2).forEach((warning) => {
            details.push(`${repo.name} warning${warning.itemPath ? ` (${warning.itemPath})` : ''}: ${warning.message}`);
          });
        } catch (err) {
          failures += 1;
          details.push(`${repo.name}: ${errorMessage(err)}`);
        }
      }
      notifyReliabilityChanged();
      setNotice({
        tone: failures > 0 ? 'error' : 'success',
        title: failures > 0 ? `Scan finished with ${failures} failure${failures === 1 ? '' : 's'}` : 'All workspaces scanned',
        details
      });
      await onChanged();
    } finally {
      setBusy(false, 'scan-all');
    }
  };

  const toggleOverviewSection = (section: OverviewSectionKey) => {
    setCollapsedOverviewSections((current) => ({ ...current, [section]: !current[section] }));
  };


  const selectWorkspace = (workspaceId: string) => {
    if (workspaceId === selectedWorkspaceId) return;
    if (!confirmDiscardEdit()) return;
    discardEdit();
    setSelectedWorkspaceId(workspaceId);
    setActiveDetailTab('overview');
  };

  const selectDetailTab = (tab: WorkspaceDetailTab) => {
    if (tab === activeDetailTab) return;
    if (!confirmDiscardEdit()) return;
    discardEdit();
    setActiveDetailTab(tab);
  };

  function closeRegistration() {
	if (busy) {
		registrationController.current?.abort();
		setRegistrationOpen(false);
		return;
	}
	const dirty = draft.dirty || Boolean(importSourcePath.trim());
    if (dirty && !window.confirm('Discard this workspace registration draft?')) return;
    registrationController.current?.abort();
    draft.reset(systemConfig?.cloneRootDir ?? '');
	resetWorkspaceImport();
    setRegistrationOpen(false);
  }

  const advanceRegistration = (event: FormEvent) => {
    event.preventDefault();
	if (registrationMode === 'existing_workspace') {
		void previewExistingWorkspaces();
		return;
	}
    if (!registrationLocationReady || !name.trim()) return;
    setRegistrationStep(2);
  };

  const removeWorkspaces = async (targets: WorkspaceConfig[]) => {
    if (targets.length === 0) {
      setWorkspacesToRemove(null);
      return;
    }
    setBusy(true);
    setNotice(null);
    let removedCount = 0;
    let failureCount = 0;
    const details: string[] = [];
    const failedWorkspaceIds = new Set<string>();
    try {
      for (const workspace of targets) {
        try {
          await api.deleteWorkspace(workspace.id);
          removedCount += 1;
        } catch (err) {
          failureCount += 1;
          failedWorkspaceIds.add(workspace.id);
          details.push(`${workspace.name}: ${errorMessage(err)}`);
        }
      }
      discardEdit();
      setSelectedWorkspaceIds((current) => Array.from(new Set(
        current.filter((id) => !targets.some((workspace) => workspace.id === id)).concat(Array.from(failedWorkspaceIds))
      )));
      if (failureCount > 0) {
        setNotice({
          tone: 'error',
          title: `Removed ${removedCount} workspace${removedCount === 1 ? '' : 's'} with ${failureCount} failure${failureCount === 1 ? '' : 's'}`,
          details
        });
      } else {
        setNotice({
          tone: 'success',
          title: removedCount === 1 ? 'Workspace removed' : `${removedCount} workspaces removed`,
          details: targets.map((workspace) => workspace.name)
        });
      }
      if (removedCount > 0) {
        await onChanged();
      }
    } finally {
      setBusy(false);
      setWorkspacesToRemove(null);
    }
  };

  const toggleWorkspaceSelection = (workspaceId: string) => {
    setSelectedWorkspaceIds((current) => current.includes(workspaceId)
      ? current.filter((id) => id !== workspaceId)
      : [...current, workspaceId]);
  };

  const toggleAllWorkspaceSelection = () => {
    setSelectedWorkspaceIds((current) => {
      if (workspaces.length === 0) return [];
      const selectedCount = workspaces.filter((workspace) => current.includes(workspace.id)).length;
      if (selectedCount === workspaces.length) return [];
      return workspaces.map((workspace) => workspace.id);
    });
  };

  const browsePath = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const selection = await api.selectDirectory();
      setPath(selection.path);
      setName(lastPathSegment(selection.path));
      setRegistrationNameEdited(false);
    } catch (err) {
      setNotice({ tone: 'error', title: 'Directory selection failed', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const browseCloneRoot = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const selection = await api.selectDirectory();
      setCloneRoot(selection.path);
    } catch (err) {
      setNotice({ tone: 'error', title: 'Directory selection failed', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const revealPath = async (targetPath: string) => {
    setBusy(true);
    setNotice(null);
    try {
      await api.openPath(targetPath);
    } catch (err) {
      setNotice({ tone: 'error', title: 'Path failed to open', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const connectCloudAgent = async () => {
    setBusy(true, 'connect-agent');
    setNotice(null);
    setAgentStatusLabel('connecting');
    try {
      const token = await api.createAgentConnectToken({ name: 'Cloud Agent', platform: navigator.platform });
      setAgentDeepLink(token.deepLink);
      setAgentStatusLabel('connecting');
      if (!navigator.userAgent.toLowerCase().includes('jsdom')) {
        window.location.href = token.deepLink;
      }
    } catch (err) {
      setAgentStatusLabel('unsupported');
      setNotice({ tone: 'error', title: 'Cloud Agent connection failed', details: [errorMessage(err)] });
    } finally {
      setBusy(false, 'connect-agent');
    }
  };

  const saveShownIn = async (repo: WorkspaceConfig, path: string, surface: SourceSurface) => {
    const sources = toggleSourceSurface(workspaceSources(repo), path, surface);
    if (!sources) return;
    setBusy(true);
    setNotice(null);
    try {
      await api.updateWorkspace(repo.id, {
        name: repo.name,
        path: repo.path,
        baselineBranch: repo.baselineBranch,
        sources,
        registrationMode: repo.registrationMode,
        remoteUrl: repo.remoteUrl,
        jira: repo.jira,
        knowledge: normalizeKnowledgeSettings({ enabled: repo.knowledge?.enabled ?? true, enrichExecutable: repo.knowledge?.enrichExecutable ?? '', enrichArgs: [...(repo.knowledge?.enrichArgs ?? [])] })
      });
      setNotice({ tone: 'success', title: 'Source updated', details: [`${path} shows in ${sources.find((source) => source.path === path)?.shownIn.map((item) => sourceSurfaceLabels[item]).join(' and ') ?? ''}`] });
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: 'Could not update the source', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const openSourceSettings = async (repo: WorkspaceConfig, directory: string) => {
    setBusy(true);
    setNotice(null);
    try {
      const result = await api.sourceStructure(repo.id, directory);
      setSettingsEditor(settingsEditorFromResult(repo, directory, result));
    } catch (err) {
      setNotice({ tone: 'error', title: 'Settings failed to load', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const dropPath = (event: DragEvent<HTMLLabelElement>) => {
    if (registrationMode !== 'local_path') {
      event.preventDefault();
      setPathDragging(false);
      return;
    }
    event.preventDefault();
    setPathDragging(false);
    const droppedPath = pathFromDrop(event);
    if (droppedPath) {
      setPath(droppedPath);
      setName(lastPathSegment(droppedPath));
      setRegistrationNameEdited(false);
      return;
    }
    setNotice({ tone: 'error', title: 'Drop a folder path or file URL' });
  };

  return (
    <section className="workspaces-page">
      <div className="page-title">
        <div>
          <h1>Workspaces</h1>
          <span>Browse, configure, and scan registered workspaces.</span>
        </div>
        <div className="workspace-manager-page-actions">
          <button className="secondary" type="button" onClick={() => void scanAll()} disabled={operationBusy('scan-all') || workspaces.length === 0 || !canScan}>
            <RotateCw size={16} /> Scan all
          </button>
          <button className="primary" type="button" onClick={() => setRegistrationOpen(true)} disabled={!canRegisterWorkspace}>
            <Plus size={16} /> Add workspace
          </button>
        </div>
      </div>

      <div className="workspaces-layout">
        <WorkspaceList
          workspaces={workspaces}
          selectedWorkspaceId={selectedWorkspaceId}
          selectedWorkspaceIds={selectedWorkspaceIds}
          query={workspaceQuery}
          bulkMode={bulkMode}
          onQueryChange={setWorkspaceQuery}
          onSelect={selectWorkspace}
          onToggleBulkMode={() => {
            setBulkMode((current) => !current);
            setSelectedWorkspaceIds([]);
          }}
          onToggleSelection={toggleWorkspaceSelection}
        />

        <div className="repo-list-panel workspace-manager-detail">
          {bulkMode && <div className="workspace-manager-bulk-bar">
            <label className="repo-select-all">
              <input
                ref={selectAllRef}
                type="checkbox"
                checked={allSelected}
                onChange={toggleAllWorkspaceSelection}
                disabled={busy || workspaces.length === 0}
              />
              Select all ({selectedWorkspaces.length} selected)
            </label>
            <button className="secondary danger" type="button" onClick={() => setWorkspacesToRemove(selectedWorkspaces)} disabled={busy || selectedWorkspaces.length === 0 || !canWrite}>
              <Trash2 size={16} /> Remove selected
            </button>
          </div>}
          {notice && <WorkspaceNoticePanel notice={notice} onDismiss={() => setNotice(null)} />}
          {selectedWorkspaceId && <div className="workspace-detail-tabs">
            <div className="workspace-detail-tab-list" role="tablist" aria-label="Workspace settings">
              {detailTabs.map((tab) => (
                <button
                  key={tab}
                  id={`workspace-tab-${tab}`}
                  type="button"
                  role="tab"
                  aria-selected={activeDetailTab === tab}
                  aria-controls="workspace-detail-panel"
                  className={activeDetailTab === tab ? 'active' : undefined}
                  onClick={() => selectDetailTab(tab)}
                  onKeyDown={(event) => {
                    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
                    event.preventDefault();
                    const tabs = detailTabs;
                    const offset = event.key === 'ArrowRight' ? 1 : -1;
                    const nextTab = tabs[(tabs.indexOf(tab) + offset + tabs.length) % tabs.length];
                    selectDetailTab(nextTab);
                    requestAnimationFrame(() => document.getElementById(`workspace-tab-${nextTab}`)?.focus());
                  }}
                >
                  {detailTabLabels[tab]}
                </button>
              ))}
            </div>
            {editingId === selectedWorkspaceId && activeDetailTab === 'integrations' && isIntegrationEditSection(editingSection) && (
              <button className="secondary workspace-detail-back-button" type="button" onClick={discardEdit} disabled={busy}><ArrowLeft size={15} /> Back</button>
            )}
          </div>}
          <div className="repo-list">
            {workspaces.filter((repo) => repo.id === selectedWorkspaceId).map((repo) => (
              <article id="workspace-detail-panel" className="workspace-detail-panel" key={repo.id} role="tabpanel" aria-labelledby={`workspace-tab-${activeDetailTab}`}>
                <header className="workspace-detail-heading">
                  <div className="repo-row-icon"><HardDrive size={18} /></div>
                  <div><h2>{repo.name}</h2><span>{workspaceDetailSubtitle(activeDetailTab)}</span></div>
                  <div className="workspace-detail-heading-actions">
                    {showHarness && <>
                      <button className="secondary" type="button" onClick={() => setSettingsFile('export')} title="Write this workspace's settings into .kode-stream/settings.yaml"><Upload size={16} /> Export settings</button>
                      <button className="secondary" type="button" onClick={() => setSettingsFile('import')} title="Take settings from .kode-stream/settings.yaml"><Download size={16} /> Import settings</button>
                    </>}
                    <button className="secondary" type="button" onClick={() => scan(repo)} disabled={operationBusy(`scan:${repo.id}`) || operationBusy('scan-all') || !canScan}><RotateCw size={16} /> Scan workspace</button>
                    <button className="secondary danger" type="button" onClick={() => setWorkspacesToRemove([repo])} disabled={!canWrite}><Trash2 size={16} /> Remove</button>
                  </div>
                </header>

                {activeDetailTab === 'overview' && <section className="workspace-overview-sections">
                  <OverviewSection title="General" collapsed={collapsedOverviewSections.general} onToggle={() => toggleOverviewSection('general')}>
                    <WorkspaceGeneralSection repo={repo} edit={edit} busy={busy} onReveal={(path) => void revealPath(path)} />
                  </OverviewSection>

                  <OverviewSection title="Sources" collapsed={collapsedOverviewSections.sources} onToggle={() => toggleOverviewSection('sources')}>
                    <WorkspaceSourcesSection repo={repo} edit={edit} busy={busy} canWrite={canWrite} onShownIn={(path, surface) => void saveShownIn(repo, path, surface)} onConfigure={(path) => void openSourceSettings(repo, path)} />
                  </OverviewSection>
                </section>}

                {activeDetailTab === 'health' && <section className="workspace-detail-section">
                  <WorkspaceHealthPanel workspaceId={repo.id} embedded />
                </section>}

                {activeDetailTab === 'integrations' && <section className="workspace-detail-section">
                  <WorkspaceIntegrations repo={repo} edit={edit} busy={busy} showHarness={showHarness} settingsKey={`${repo.id}:${settingsImports}`} />
                </section>}

                {activeDetailTab === 'agents' && <WorkspaceOwnSettings key={`${repo.id}:${settingsImports}`} workspace={repo} showHarness={showHarness} />}

                {settingsFile && <SettingsFileDialog mode={settingsFile} workspaceId={repo.id} workspaceName={repo.name} onClose={() => setSettingsFile(null)} onImported={() => setSettingsImports((value) => value + 1)} />}

              </article>
            ))}
            {workspaces.length === 0 && <div className="empty-inline repo-empty"><CheckCircle2 size={18} /> No workspaces registered.</div>}
          </div>
        </div>
      </div>

      {registrationOpen && <ModalLayer><section className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) closeRegistration(); }}>
        <div ref={registrationDialog.ref as RefObject<HTMLDivElement>} className="modal-panel workspace-registration-modal" role="dialog" aria-modal="true" aria-labelledby="add-workspace-title">
          <header>
            <div><h2 id="add-workspace-title">Add workspace</h2><span>Register, clone, or import existing workspace definitions.</span></div>
            <button className="icon-button" type="button" onClick={closeRegistration} aria-label="Close add workspace"><X size={16} /></button>
          </header>
          {cloudMode ? <CloudAgentRegistration
            status={agentStatusLabel || runtimeContext.agent.status}
            deepLink={agentDeepLink}
            busy={operationBusy('connect-agent')}
            onConnect={() => void connectCloudAgent()}
          /> : <>
          {registrationMode !== 'existing_workspace' && <ol className="workspace-registration-steps" aria-label="Workspace registration progress">
            <li className={registrationStep === 1 ? 'active' : 'complete'}><span>1</span><div><strong>Repository</strong><small>Location and content</small></div></li>
            <li className={registrationStep === 2 ? 'active' : ''}><span>2</span><div><strong>Jira</strong><small>Optional integration</small></div></li>
          </ol>}
          <form className="workspace-registration-form" onSubmit={registrationStep === 1 ? advanceRegistration : submit}>
            {registrationStep === 1 ? <>
              <div className="registration-mode-toggle" role="radiogroup" aria-label="Workspace registration mode">
                <button className={registrationMode === 'local_path' ? 'secondary active' : 'secondary'} type="button" role="radio" aria-checked={registrationMode === 'local_path'} onClick={() => {
                  setRegistrationMode('local_path'); setPathDragging(false); setRegistrationNameEdited(false); setName(lastPathSegment(path));
                }}>Local Path</button>
                <button className={registrationMode === 'remote_clone' ? 'secondary active' : 'secondary'} type="button" role="radio" aria-checked={registrationMode === 'remote_clone'} onClick={() => {
                  setRegistrationMode('remote_clone'); setPathDragging(false); setRegistrationNameEdited(false); setName(inferWorkspaceNameFromRemoteURL(remoteUrl));
                  if (!cloneRoot && systemConfig?.cloneRootDir) setCloneRoot(systemConfig.cloneRootDir);
                }}>Remote Git URL</button>
				<button className={registrationMode === 'existing_workspace' ? 'secondary active' : 'secondary'} type="button" role="radio" aria-checked={registrationMode === 'existing_workspace'} onClick={() => {
					setRegistrationMode('existing_workspace'); setPathDragging(false); setRegistrationNameEdited(false); setName('');
				}}>Existing Workspaces</button>
              </div>
				{registrationMode === 'existing_workspace' ? <>
					<label className="repo-field path-field">Import Source
						<div className="path-input-row">
							<input aria-label="Import source path" value={importSourcePath} onChange={(event) => changeImportSourcePath(event.target.value)} placeholder="/path/to/workspaces.yaml" disabled={importState === 'importing'} autoFocus />
							<button className="secondary icon-action" type="button" onClick={() => void browseImportSource()} disabled={importState === 'previewing' || importState === 'importing'} title="Select YAML file"><FolderOpen size={16} /></button>
						</div>
					</label>
					{importError && <div className="metadata-callout" role="alert"><strong>{importPreview ? 'Workspace import failed' : 'Import preview failed'}</strong><span>{importError}</span></div>}
					<div ref={importStatusRef} tabIndex={-1}>{importPreview && importState !== 'complete' && <WorkspaceImportReview preview={importPreview} selectedKeys={importSelection} onToggle={toggleImportCandidate} onToggleAll={toggleAllImportCandidates} />}
					{importState === 'complete' && <WorkspaceImportResults results={importResults} />}</div>
					<div className="workspace-registration-actions">
						<button className="secondary" type="button" onClick={closeRegistration}>{importState === 'complete' ? 'Done' : 'Cancel'}</button>
						{importState !== 'complete' && <button className={importPreview ? 'secondary' : 'primary'} type="submit" disabled={!importSourcePath.trim() || importState === 'previewing' || importState === 'importing'}>{importState === 'previewing' ? 'Loading preview...' : importPreview ? 'Refresh preview' : 'Preview workspaces'}</button>}
						{importPreview && importState !== 'complete' && <button className="primary" type="button" onClick={() => setImportConfirmOpen(true)} disabled={importSelection.length === 0 || importState === 'previewing' || importState === 'importing'}>{importState === 'importing' ? 'Importing...' : `Import ${importSelection.length} selected`}</button>}
					</div>
				</> : <>
				{registrationMode === 'local_path' ? <label className={pathDragging ? 'repo-field path-field dragging' : 'repo-field path-field'} onDragOver={(event) => { event.preventDefault(); setPathDragging(true); }} onDragLeave={() => setPathDragging(false)} onDrop={dropPath}>
                Local Path
                <div className="path-input-row">
                  <input value={path} onChange={(event) => { const next = event.target.value; setPath(next); if (!registrationNameEdited) setName(lastPathSegment(next)); }} placeholder="/Users/me/workspace/repo" autoFocus />
                  <button className="secondary icon-action" type="button" onClick={browsePath} disabled={busy} title="Browse"><FolderOpen size={16} /></button>
                  <button className="secondary icon-action" type="button" onClick={() => revealPath(path)} disabled={busy || !path} title="Reveal"><ExternalLink size={16} /></button>
                </div>
              </label> : <label className="repo-field">Remote Git URL<input value={remoteUrl} onChange={(event) => {
                const next = event.target.value; setRemoteUrl(next); if (!registrationNameEdited) setName(inferWorkspaceNameFromRemoteURL(next));
              }} placeholder="git@bitbucket.org:team/repo.git" autoFocus /></label>}

              {registrationLocationReady && <div className="workspace-registration-revealed">
                <label className="repo-field">Workspace Name<input value={name} onChange={(event) => { setName(event.target.value); setRegistrationNameEdited(true); }} placeholder="Discovery" /></label>
                {registrationMode === 'remote_clone' && <>
                  <label className="repo-field">Clone Root
                    <div className="path-input-row">
                      <input value={cloneRoot} onChange={(event) => setCloneRoot(event.target.value)} placeholder={systemConfig?.cloneRootDir ?? '/path/to/kode-stream/clone-root'} />
                      <button className="secondary icon-action" type="button" onClick={browseCloneRoot} disabled={busy} title="Browse"><FolderOpen size={16} /></button>
                      <button className="secondary icon-action" type="button" onClick={() => revealPath(cloneRoot)} disabled={busy || !cloneRoot} title="Reveal"><ExternalLink size={16} /></button>
                    </div>
                  </label>
                  {systemConfig && <span className="repo-remote-default">Default clone root: {systemConfig.cloneRootDir}</span>}
                </>}
                <div className="repo-field-grid"><BranchField value={baselineBranch} onChange={setBaselineBranch} /><SourcesField value={sources} onChange={setSources} /></div>
              </div>}
              <div className="workspace-registration-actions">
                <button className="secondary" type="button" onClick={closeRegistration}>Cancel</button>
                <button className="primary" type="submit" disabled={!registrationLocationReady || !name.trim()}>Next: Jira <span aria-hidden="true">→</span></button>
              </div>
				</>}
            </> : <>
              <div className="workspace-registration-step-heading"><span>Optional</span><h3>Connect Jira</h3><p>Configure Jira now, or leave it disabled and connect later from Integrations.</p></div>
              <JiraConnectionFields value={jira} onChange={setJira} />
              {registrationLog && <section className="registration-log-panel"><button className="secondary" type="button" onClick={() => setRegistrationLogOpen((open) => !open)}>{registrationLogOpen ? 'Hide logs' : 'Show logs'}</button>{registrationLogOpen && <pre>{registrationLog}</pre>}</section>}
              <div className="workspace-registration-actions">
                <button className="secondary" type="button" onClick={() => setRegistrationStep(1)} disabled={busy}>Back</button>
                <button className="primary" type="submit" disabled={busy}><FolderGit2 size={16} /> {jira ? 'Register with Jira' : 'Register workspace'}</button>
              </div>
            </>}
          </form>
          </>}
        </div>
      </section></ModalLayer>}
		{importConfirmOpen && <ConfirmDialog
			title="Import selected workspaces"
			message={`Import ${importSelection.length} workspace${importSelection.length === 1 ? '' : 's'} into ${importPreview?.destinationPath ?? 'the effective registry'} and start indexing?`}
			confirmLabel={`Import ${importSelection.length} workspace${importSelection.length === 1 ? '' : 's'}`}
			busy={importState === 'importing'}
			onCancel={() => setImportConfirmOpen(false)}
			onConfirm={() => void importExistingWorkspaces()}
		/>}
      {workspacesToRemove && (
        <ConfirmDialog
          title={workspacesToRemove.length === 1 ? 'Remove workspace' : 'Remove selected workspaces'}
          message={workspaceRemovalMessage(workspacesToRemove)}
          confirmLabel={busy ? 'Removing...' : workspacesToRemove.length === 1 ? 'Remove workspace' : `Remove ${workspacesToRemove.length} workspaces`}
          busy={busy}
          danger
          onCancel={() => setWorkspacesToRemove(null)}
          onConfirm={() => void removeWorkspaces(workspacesToRemove)}
        />
      )}
      {settingsEditor && (
        <SourceSettingsDialog editor={settingsEditor} setEditor={setSettingsEditor} busy={busy} setBusy={setBusy} onNotice={setNotice} onChanged={onChanged} />
      )}
    </section>
  );
}
