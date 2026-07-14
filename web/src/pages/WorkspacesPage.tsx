import { type DragEvent, type Dispatch, type FormEvent, type ReactNode, type SetStateAction, useEffect, useRef, useState } from 'react';
import { ArrowLeft, CheckCircle2, ChevronDown, ChevronRight, ExternalLink, FolderGit2, FolderOpen, HardDrive, Link2, Pencil, Plus, RotateCw, SlidersHorizontal, Trash2, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { WorkspaceHealthPanel } from '../components/ReliabilityPanels';
import { ApiError, api } from '../lib/api';
import type { JiraConnection, KnowledgeSettings, RuntimeContext, RuntimeType, WorkspaceConfig, WorkspaceImportCandidate, WorkspaceImportPreview, WorkspaceImportResult, WorkspaceInput, WorkspaceRegistrationMode, WorkspaceRuntimeConfig, SourceStructureSettings, SourceStructureCard, SourceStructurePreview, SourceStructureProposal, SourceSettingsResult, ScanResult, SystemConfigPaths } from '../lib/types';
import { labels } from '../lib/vocabulary';
import { applySegmentRole, inferCompatibilityFields, lastPathSegment, normalizeDroppedPath, parseSources, previewPathSegments } from '../features/workspaces/sourceSettings';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { WorkspaceList } from '../features/workspaces/WorkspaceManagerShell';

export { applySegmentRole, inferCompatibilityFields, normalizeDroppedPath, parseSources, previewPathSegments };

const DEFAULT_SOURCES = ['docs', 'plans'];
const UNSORTED_SELECTION_ID = 'unsorted';
const emptyJiraConnection = (): JiraConnection => ({ deploymentType: 'cloud', baseUrl: '', projectKey: '', accountEmail: '', tokenEnvVar: 'JIRA_API_TOKEN' });
const defaultRuntimeConfig = (): WorkspaceRuntimeConfig => ({
  type: 'docker-compose',
  configPath: '',
  rebuildPolicy: 'changed-only',
  commands: {
    up: '',
    down: '',
    rebuildChanged: '',
    verify: { smoke: '', critical: '', full: '' }
  },
  healthChecks: [],
  artifacts: { paths: [] },
  automation: {
    enabled: false,
    repositoryPath: '',
    runner: 'cypress',
    defaultEnvironment: 'local',
    commandTemplate: 'CYPRESS_EPSAP_ENVIRONMENT={env} npx cypress run --spec "{specs}"',
    artifactPaths: ['cypress/reports', 'cypress/screenshots', 'cypress/videos']
  }
});
const localRuntimeContext: RuntimeContext = {
  mode: 'local',
  role: 'admin',
  capabilities: { read: true, write: true, workspace_registration: true, git: true, system: true, terminal: true, ai: true, runtime: true, verification: true },
  agent: { available: true, status: 'local' }
};
type WorkspaceNotice = {
  tone: 'success' | 'error' | 'info';
  title: string;
  details?: string[];
};
type SettingsEditorState = {
  repo: WorkspaceConfig;
  directory: string;
  exists: boolean;
  mode?: string;
  card: SourceStructureCard;
  warnings: string[];
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  unsortedPreview: SourceStructurePreview[];
  preview: SourceStructurePreview[];
};
type WorkspaceDetailTab = 'overview' | 'health' | 'integrations';
type WorkspaceEditSection = 'general' | 'sources' | 'jira' | 'knowledge' | 'runtime' | '';
type OverviewSectionKey = 'general' | 'sources';
export type WorkspaceImportState = 'selecting' | 'previewing' | 'reviewing' | 'importing' | 'complete' | 'error';

export function defaultWorkspaceImportSelection(preview: WorkspaceImportPreview): string[] {
	return preview.candidates.filter((candidate) => candidate.status === 'valid' && candidate.selected).map((candidate) => candidate.candidateKey);
}

export function WorkspacesPage({ workspaces, runtimeContext = localRuntimeContext, onChanged }: { workspaces: WorkspaceConfig[]; runtimeContext?: RuntimeContext; onChanged: () => void | Promise<void> }) {
  const [name, setName] = useState('');
  const [registrationMode, setRegistrationMode] = useState<WorkspaceRegistrationMode>('local_path');
  const [path, setPath] = useState('');
  const [remoteUrl, setRemoteUrl] = useState('');
  const [cloneRoot, setCloneRoot] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('master');
  const [sources, setSources] = useState('');
  const [jira, setJira] = useState<JiraConnection | null>(null);
  const [systemConfig, setSystemConfig] = useState<SystemConfigPaths | null>(null);
  const [notice, setNotice] = useState<WorkspaceNotice | null>(null);
  const [registrationLog, setRegistrationLog] = useState('');
  const [registrationLogOpen, setRegistrationLogOpen] = useState(false);
  const [pendingOperations, setPendingOperations] = useState<string[]>([]);
  const [pathDragging, setPathDragging] = useState(false);
  const [editingId, setEditingId] = useState('');
  const [editingSection, setEditingSection] = useState<WorkspaceEditSection>('');
  const [editDraft, setEditDraft] = useState<{ name: string; path: string; baselineBranch: string; sources: string; jira: JiraConnection | null; knowledge: KnowledgeSettings; runtime: WorkspaceRuntimeConfig | null }>({ name: '', path: '', baselineBranch: '', sources: '', jira: null, knowledge: {}, runtime: null });
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
  const [registrationStep, setRegistrationStep] = useState<1 | 2>(1);
  const [registrationNameEdited, setRegistrationNameEdited] = useState(false);
	const [importSourcePath, setImportSourcePath] = useState('');
	const [importState, setImportState] = useState<WorkspaceImportState>('selecting');
	const [importPreview, setImportPreview] = useState<WorkspaceImportPreview | null>(null);
	const [importSelection, setImportSelection] = useState<string[]>([]);
	const [importResults, setImportResults] = useState<WorkspaceImportResult[]>([]);
	const [importError, setImportError] = useState('');
	const [importConfirmOpen, setImportConfirmOpen] = useState(false);
  const [collapsedOverviewSections, setCollapsedOverviewSections] = useState<Record<OverviewSectionKey, boolean>>({ general: false, sources: false });
  const selectAllRef = useRef<HTMLInputElement | null>(null);
	const importStatusRef = useRef<HTMLDivElement | null>(null);

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

  useEffect(() => {
    setSelectedWorkspaceIds((current) => current.filter((id) => workspaces.some((workspace) => workspace.id === id)));
    setSelectedWorkspaceId((current) => workspaces.some((workspace) => workspace.id === current) ? current : workspaces[0]?.id ?? '');
  }, [workspaces]);

  useEffect(() => {
    if (!selectAllRef.current) return;
    selectAllRef.current.indeterminate = selectedWorkspaces.length > 0 && !allSelected;
  }, [allSelected, selectedWorkspaces.length]);

  useEffect(() => {
    let active = true;
    void api.systemConfigPaths().then((result) => {
      if (!active) return;
      setSystemConfig(result);
      setCloneRoot((current) => current || result.cloneRootDir);
    }).catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

	useEffect(() => {
		if (importState === 'reviewing' || importState === 'complete') importStatusRef.current?.focus();
	}, [importState]);

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
          setRegistrationLog((current) => `${current}${chunk}`);
          setRegistrationLogOpen(true);
        })
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

  const startEdit = (repo: WorkspaceConfig, section: Exclude<WorkspaceEditSection, ''>) => {
    setEditingId(repo.id);
    setEditingSection(section);
      setEditDraft({
        name: repo.name,
        path: repo.path,
        baselineBranch: repo.baselineBranch,
        sources: repo.sources.join(', '),
        jira: repo.jira ? { ...repo.jira } : null,
        knowledge: { enabled: repo.knowledge?.enabled ?? true, enrichExecutable: repo.knowledge?.enrichExecutable ?? '', enrichArgs: [...(repo.knowledge?.enrichArgs ?? [])] },
        runtime: cloneRuntimeConfig(repo.runtime)
      });
    setNotice(null);
  };

  const discardEdit = () => {
    setEditingId('');
    setEditingSection('');
  };

  const toggleOverviewSection = (section: OverviewSectionKey) => {
    setCollapsedOverviewSections((current) => ({ ...current, [section]: !current[section] }));
  };

  const confirmDiscardEdit = () => !editingId || window.confirm('Discard unsaved workspace changes?');

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

  const closeRegistration = () => {
	const dirty = Boolean(name.trim() || path.trim() || remoteUrl.trim() || importSourcePath.trim() || sources.trim() || jira);
    if (dirty && !window.confirm('Discard this workspace registration draft?')) return;
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
	resetWorkspaceImport();
    setRegistrationOpen(false);
  };

  const advanceRegistration = (event: FormEvent) => {
    event.preventDefault();
	if (registrationMode === 'existing_workspace') {
		void previewExistingWorkspaces();
		return;
	}
    if (!registrationLocationReady || !name.trim()) return;
    setRegistrationStep(2);
  };

	const resetWorkspaceImport = () => {
		setImportSourcePath('');
		setImportState('selecting');
		setImportPreview(null);
		setImportSelection([]);
		setImportResults([]);
		setImportError('');
		setImportConfirmOpen(false);
	};

	const changeImportSourcePath = (value: string) => {
		setImportSourcePath(value);
		setImportState('selecting');
		setImportPreview(null);
		setImportSelection([]);
		setImportResults([]);
		setImportError('');
	};

	const toggleImportCandidate = (candidateKey: string) => {
		setImportSelection((current) => current.includes(candidateKey) ? current.filter((key) => key !== candidateKey) : [...current, candidateKey]);
	};

	const toggleAllImportCandidates = () => {
		if (!importPreview) return;
		const selectable = importPreview.candidates.filter((candidate) => candidate.status === 'valid').map((candidate) => candidate.candidateKey);
		setImportSelection((current) => selectable.every((key) => current.includes(key)) ? [] : selectable);
	};

	const importExistingWorkspaces = async () => {
		if (!importPreview || importSelection.length === 0) return;
		setImportConfirmOpen(false);
		setImportState('importing');
		setImportError('');
		try {
			const results = await api.importWorkspaces({ sourcePath: importPreview.sourcePath, candidateKeys: importSelection });
			setImportResults(results);
			setImportState('complete');
			if (results.some((result) => result.workspace)) await onChanged();
		} catch (err) {
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

	const browseImportSource = async () => {
		setImportError('');
		try {
			const selection = await api.selectYAMLFile();
			if (!selection.path) return;
			changeImportSourcePath(selection.path);
		} catch (err) {
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

	const previewExistingWorkspaces = async () => {
		if (!importSourcePath.trim()) return;
		setImportState('previewing');
		setImportError('');
		try {
			const preview = await api.previewWorkspaceImport(importSourcePath.trim());
			setImportPreview(preview);
			setImportSourcePath(preview.sourcePath);
			setImportSelection(defaultWorkspaceImportSelection(preview));
			setImportState('reviewing');
		} catch (err) {
			setImportPreview(null);
			setImportSelection([]);
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

  const saveEdit = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setNotice(null);
    try {
      await api.updateWorkspace(repo.id, {
        name: editDraft.name,
        path: editDraft.path,
        baselineBranch: editDraft.baselineBranch,
        sources: parseSources(editDraft.sources),
        registrationMode: repo.registrationMode,
        remoteUrl: repo.remoteUrl,
        jira: editDraft.jira ?? undefined,
        knowledge: normalizeKnowledgeSettings(editDraft.knowledge),
        runtime: editDraft.runtime ?? undefined
      });
      setEditingId('');
      setEditingSection('');
      setNotice({ tone: 'success', title: 'Workspace updated', details: [editDraft.name || repo.name] });
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: `Update failed for ${repo.name}`, details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
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
      setEditingId('');
      setEditingSection('');
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

  const saveSourceSettings = async () => {
    if (!settingsEditor) return;
    setBusy(true);
    setNotice(null);
    try {
      if (settingsEditor.selectedProposalId === UNSORTED_SELECTION_ID) {
        if (settingsEditor.exists) {
          const result = await api.resetSourceStructure(settingsEditor.repo.id, settingsEditor.directory);
          setNotice(sourceSettingsNotice('Source structure reset', settingsEditor.repo, result.scan));
        } else {
          const result = await api.scan(settingsEditor.repo.id);
          setNotice(scanNotice(settingsEditor.repo, result, 'Source kept unsorted'));
        }
        notifyReliabilityChanged();
        setSettingsEditor(null);
        await onChanged();
        return;
      }
      const settings: SourceStructureSettings = {
        version: 1,
        cards: [withInferredCompatibilityFields(settingsEditor.card, settingsEditor.directory)]
      };
      const result = await api.saveSourceStructure(settingsEditor.repo.id, settingsEditor.directory, settings);
      notifyReliabilityChanged();
      setSettingsEditor(null);
      setNotice(sourceSettingsNotice('Source structure saved', settingsEditor.repo, result.scan));
      await onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: 'Settings failed to save', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const resetSourceSettings = async () => {
    if (!settingsEditor) return;
    const confirmed = window.confirm(`Reset Source Items for ${settingsEditor.directory}? This removes workspace-settings.yaml and scans the source again.`);
    if (!confirmed) return;
    setBusy(true);
    setNotice(null);
    try {
      const { repo, directory } = settingsEditor;
      const result = await api.resetSourceStructure(repo.id, directory);
      notifyReliabilityChanged();
      setSettingsEditor(settingsEditorFromResult(repo, directory, result));
      setNotice(sourceSettingsNotice('Source structure reset', repo, result.scan));
      await onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: 'Settings reset failed', details: [errorMessage(err)] });
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
              {(['overview', 'health', 'integrations'] as WorkspaceDetailTab[]).map((tab) => (
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
                    const tabs: WorkspaceDetailTab[] = ['overview', 'health', 'integrations'];
                    const offset = event.key === 'ArrowRight' ? 1 : -1;
                    const nextTab = tabs[(tabs.indexOf(tab) + offset + tabs.length) % tabs.length];
                    selectDetailTab(nextTab);
                    requestAnimationFrame(() => document.getElementById(`workspace-tab-${nextTab}`)?.focus());
                  }}
                >
                  {tab[0].toUpperCase() + tab.slice(1)}
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
                    <button className="secondary" type="button" onClick={() => scan(repo)} disabled={operationBusy(`scan:${repo.id}`) || operationBusy('scan-all') || !canScan}><RotateCw size={16} /> Scan workspace</button>
                    <button className="secondary danger" type="button" onClick={() => setWorkspacesToRemove([repo])} disabled={!canWrite}><Trash2 size={16} /> Remove</button>
                  </div>
                </header>

                {activeDetailTab === 'overview' && <section className="workspace-overview-sections">
                  <OverviewSection title="General" collapsed={collapsedOverviewSections.general} onToggle={() => toggleOverviewSection('general')}>
                    {editingId === repo.id && editingSection === 'general' ? <>
                      <div className="repo-edit-form">
                        <label className="repo-field">Workspace Name<input value={editDraft.name} onChange={(event) => setEditDraft({ ...editDraft, name: event.target.value })} /></label>
                        <label className="repo-field">Local Path<input value={editDraft.path} onChange={(event) => setEditDraft({ ...editDraft, path: event.target.value })} /></label>
                        <BranchField value={editDraft.baselineBranch} onChange={(value) => setEditDraft({ ...editDraft, baselineBranch: value })} />
                      </div>
                      <div className="repo-row-actions">
                        <button className="secondary" type="button" onClick={discardEdit} disabled={busy}>Cancel</button>
                        <button className="primary" type="button" onClick={() => saveEdit(repo)} disabled={busy}>Save general settings</button>
                      </div>
                    </> : <>
                      <dl className="workspace-overview-grid">
                        <div><dt>Location</dt><dd>{repo.location === 'cloud_agent' ? (repo.localRootLabel || 'Cloud Agent workspace') : <button className="repo-path-link" type="button" onClick={() => revealPath(repo.path)} title={repo.path}>{repo.path}</button>}</dd></div>
                        <div><dt>Base branch</dt><dd>{repo.baselineBranch}</dd></div>
                        <div><dt>Registration</dt><dd>{repo.location === 'cloud_agent' ? 'Cloud Agent' : repo.registrationMode === 'remote_clone' ? 'Remote clone' : repo.registrationMode === 'existing_workspace' ? 'Imported workspace' : 'Local folder'}</dd></div>
                        {repo.agentId && <div><dt>Agent</dt><dd>{repo.agentId}</dd></div>}
                        {repo.scanStatus && <div><dt>Scan</dt><dd>{repo.scanStatus}</dd></div>}
                        {repo.remoteUrl && <div><dt>Remote URL</dt><dd>{repo.remoteUrl}</dd></div>}
                        <div><dt>Created</dt><dd>{repo.createdAt ? new Date(repo.createdAt).toLocaleString() : 'Unknown'}</dd></div>
                      </dl>
                      <div className="repo-row-actions workspace-detail-actions">
                        {repo.location !== 'cloud_agent' && <button className="secondary" type="button" onClick={() => revealPath(repo.path)}><ExternalLink size={16} /> Reveal folder</button>}
                        {repo.location !== 'cloud_agent' && <button className="primary" type="button" onClick={() => startEdit(repo, 'general')}><Pencil size={16} /> Edit general</button>}
                      </div>
                    </>}
                  </OverviewSection>

                  <OverviewSection title="Sources" collapsed={collapsedOverviewSections.sources} onToggle={() => toggleOverviewSection('sources')}>
                    {editingId === repo.id && editingSection === 'sources' ? <>
                      <SourcesField value={editDraft.sources} onChange={(value) => setEditDraft({ ...editDraft, sources: value })} />
                      <div className="repo-row-actions"><button className="secondary" type="button" onClick={discardEdit}>Cancel</button><button className="primary" type="button" onClick={() => saveEdit(repo)}>Save sources</button></div>
                    </> : <>
                      <div className="workspace-source-list">
                        {repo.sources.map((directory) => <div className="workspace-source-row" key={directory}>
                          <div><strong>{directory}</strong><span>Workspace source directory</span></div>
                          <button className="secondary" type="button" onClick={() => void openSourceSettings(repo, directory)} disabled={!canWrite}><SlidersHorizontal size={15} /> Configure structure</button>
                        </div>)}
                        {repo.sources.length === 0 && <div className="empty-inline">No sources configured.</div>}
                      </div>
                      <div className="workspace-detail-actions"><button className="primary" type="button" onClick={() => startEdit(repo, 'sources')} disabled={!canWrite}><Pencil size={16} /> Edit sources</button></div>
                    </>}
                  </OverviewSection>
                </section>}

                {activeDetailTab === 'health' && <section className="workspace-detail-section">
                  <WorkspaceHealthPanel workspaceId={repo.id} embedded />
                </section>}

                {activeDetailTab === 'integrations' && <section className="workspace-detail-section">
                  {editingId === repo.id && isIntegrationEditSection(editingSection) ? <>
                    {editingSection === 'jira' && <JiraConnectionFields value={editDraft.jira} onChange={(value) => setEditDraft({ ...editDraft, jira: value })} workspaceId={repo.id} />}
                    {editingSection === 'knowledge' && <KnowledgeSettingsFields value={editDraft.knowledge} onChange={(value) => setEditDraft({ ...editDraft, knowledge: value })} />}
                    {editingSection === 'runtime' && <RuntimeSettingsFields value={editDraft.runtime} onChange={(value) => setEditDraft({ ...editDraft, runtime: value })} />}
                    <div className="repo-row-actions"><button className="secondary" type="button" onClick={discardEdit} disabled={busy}>Cancel</button><button className="primary" type="button" onClick={() => saveEdit(repo)} disabled={busy}>{integrationSaveLabel(editingSection)}</button></div>
                  </> : <><div className="workspace-integration-card">
                    <div><strong>Jira</strong><span>{repo.jira ? `${repo.jira.projectKey} · ${repo.jira.deploymentType === 'cloud' ? 'Cloud' : 'Server / Data Center'}` : 'Not configured'}</span></div>
                    <button className="secondary" type="button" onClick={() => startEdit(repo, 'jira')}>{repo.jira ? 'Configure' : 'Connect Jira'}</button>
                  </div><div className="workspace-integration-card"><div><strong>Knowledge</strong><span>{repo.knowledge?.enabled === false ? 'Disabled' : repo.knowledge?.enrichExecutable ? `Enabled · ${repo.knowledge.enrichExecutable}` : 'Enabled · Enrichment not configured'}</span><KnowledgeRoots workspaceId={repo.id} enabled={repo.knowledge?.enabled !== false} /></div><button className="secondary" type="button" onClick={() => startEdit(repo, 'knowledge')}>Configure</button></div><div className="workspace-integration-card"><div><strong>Runtime and verification</strong><span>{repo.runtime ? `${repo.runtime.type} · ${repo.runtime.rebuildPolicy ?? 'changed-only'}` : 'Not configured'}</span></div><button className="secondary" type="button" onClick={() => startEdit(repo, 'runtime')}>{repo.runtime ? 'Configure' : 'Set runtime'}</button></div></>}
                </section>}

              </article>
            ))}
            {workspaces.length === 0 && <div className="empty-inline repo-empty"><CheckCircle2 size={18} /> No workspaces registered.</div>}
          </div>
        </div>
      </div>

      {registrationOpen && <section className="modal-backdrop" role="presentation">
        <div className="modal-panel workspace-registration-modal" role="dialog" aria-modal="true" aria-labelledby="add-workspace-title">
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
      </section>}
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
        <section className="modal-backdrop" role="presentation">
          <div className="modal-panel source-structure-modal" role="dialog" aria-modal="true" aria-label={labels.sourceStructure}>
            <header>
              <div>
                <h2>{labels.sourceStructure}</h2>
                <span>{settingsEditor.repo.name} / {settingsEditor.directory}</span>
              </div>
              <button className="icon-button" type="button" onClick={() => setSettingsEditor(null)} disabled={busy} aria-label="Close source items">
                <X size={16} />
              </button>
            </header>
            <p className="modal-help">
              Define how this source should be split into board items.
            </p>
            {!settingsEditor.exists && settingsEditor.mode === 'structured' && (
              <div className="metadata-callout source-structure-supported">
                <strong>Built-in structure detected</strong>
                <span>This source already follows a supported item layout. Saving here creates an optional override.</span>
              </div>
            )}
            {!settingsEditor.exists && settingsEditor.mode !== 'structured' && (
              <div className="metadata-callout">
                <strong>No settings file yet</strong>
                <span>Saving creates workspace-settings.yaml inside this source.</span>
              </div>
            )}
            {settingsEditor.warnings.length > 0 && (
              <div className="plan-warnings">
                <h3>Warnings</h3>
                {settingsEditor.warnings.map((warning) => <p key={warning}>{warning}</p>)}
              </div>
            )}
            <SourceStructureProposalList
              proposals={settingsEditor.proposals}
              selectedProposalId={settingsEditor.selectedProposalId}
              onSelect={(proposal) => applySettingsProposal(setSettingsEditor, proposal)}
              onClear={() => clearSettingsProposal(setSettingsEditor)}
            />
            <SourceStructurePreviewTable
              preview={settingsEditor.preview}
              onChangeField={(path, field, value) => updateSettingsPreviewField(setSettingsEditor, path, field, value)}
            />
            <footer className="modal-actions">
              {settingsEditor.exists && (
                <button className="secondary danger" type="button" onClick={() => void resetSourceSettings()} disabled={busy}>
                  Reset config
                </button>
              )}
              <button className="secondary" type="button" onClick={() => setSettingsEditor(null)} disabled={busy}>Cancel</button>
              <button className="primary" type="button" onClick={() => void saveSourceSettings()} disabled={busy}>
                <SlidersHorizontal size={15} />
                {busy ? 'Saving...' : settingsEditor.selectedProposalId === UNSORTED_SELECTION_ID ? 'Scan Unsorted' : 'Save and Scan'}
              </button>
            </footer>
          </div>
        </section>
      )}
    </section>
  );
}

function WorkspaceImportReview({ preview, selectedKeys, onToggle, onToggleAll }: {
	preview: WorkspaceImportPreview;
	selectedKeys: string[];
	onToggle: (candidateKey: string) => void;
	onToggleAll: () => void;
}) {
	const selectable = preview.candidates.filter((candidate) => candidate.status === 'valid');
	const allSelected = selectable.length > 0 && selectable.every((candidate) => selectedKeys.includes(candidate.candidateKey));
	return <section className="workspace-import-review" aria-label="Workspace import review">
		<div className="workspace-import-summary" aria-live="polite">
			<div><strong>{preview.summary.valid} workspace{preview.summary.valid === 1 ? '' : 's'} ready to import</strong><span>{selectedKeys.length} selected from {preview.candidates.length} candidates.</span></div>
			<button className="secondary" type="button" onClick={onToggleAll} disabled={selectable.length === 0}>{allSelected ? 'Clear selectable' : 'Select all valid'}</button>
		</div>
		<div className="workspace-import-candidates">
			{preview.candidates.map((candidate) => <WorkspaceImportCandidateCard key={candidate.candidateKey} candidate={candidate} selected={selectedKeys.includes(candidate.candidateKey)} onToggle={onToggle} />)}
		</div>
	</section>;
}

function CloudAgentRegistration({ status, deepLink, busy, onConnect }: { status: string; deepLink: string; busy: boolean; onConnect: () => void }) {
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

function cloudAgentStatusLabel(status: string) {
  switch (status) {
    case 'connected': return 'Connected';
    case 'connecting': return 'Connecting';
    case 'stale': return 'Stale';
    case 'unsupported': return 'Agent not installed or blocked';
    default: return 'Offline';
  }
}

function WorkspaceImportCandidateCard({ candidate, selected, onToggle }: { candidate: WorkspaceImportCandidate; selected: boolean; onToggle: (candidateKey: string) => void }) {
	const workspace = candidate.workspace;
	const selectable = candidate.status === 'valid';
	const issues = workspaceImportVisibleIssues(candidate);
	return <article className={`workspace-import-candidate status-${candidate.status}`}>
		<header>
			<label><input type="checkbox" checked={selected} disabled={!selectable} onChange={() => onToggle(candidate.candidateKey)} /><span><strong>{workspace.name || `Candidate ${candidate.position}`}</strong><small>{workspace.path || 'No workspace path'}</small></span></label>
			<span className="workspace-import-status">{workspaceImportStatusLabel(candidate.status)}</span>
		</header>
		<dl>
			<div className="workspace-import-detail-card"><dt>Base branch</dt><dd>{workspace.baselineBranch || 'Not set'}</dd></div>
			<div className="workspace-import-detail-card"><dt>Sources</dt><dd>{workspace.sources.join(', ') || 'None'}</dd></div>
			{workspace.remoteUrl && <div className="workspace-import-detail-card workspace-import-detail-card-wide"><dt>Original remote</dt><dd>{workspace.remoteUrl}</dd></div>}
			{workspace.jira && <WorkspaceImportJiraSummary jira={workspace.jira} />}
			{workspace.knowledge && <div className="workspace-import-detail-card workspace-import-detail-card-wide"><dt>Knowledge</dt><dd>{workspace.knowledge.enabled === false ? 'Disabled' : `Enabled${workspace.knowledge.enrichExecutable ? ` · ${workspace.knowledge.enrichExecutable}` : ''}${workspace.knowledge.enrichArgs?.length ? ` · arguments: ${workspace.knowledge.enrichArgs.join(', ')}` : ''}`}</dd></div>}
		</dl>
		{issues.length > 0 && <ul className="workspace-import-issues" aria-label={`${workspace.name || `Candidate ${candidate.position}`} issues`}>{issues.map((issue, index) => <li key={`${issue.code}-${index}`}><strong>{issue.field}</strong><span>{issue.message}</span></li>)}</ul>}
	</article>;
}

function WorkspaceImportJiraSummary({ jira }: { jira: JiraConnection }) {
	return <div className="workspace-import-detail-card workspace-import-jira">
		<dt>Jira</dt>
		<dd>
			<span><strong>Deployment</strong><em>{jira.deploymentType}</em></span>
			<span><strong>Project</strong><em>{jira.projectKey || 'Not set'}</em></span>
			<span className="wide"><strong>URL</strong><em>{jira.baseUrl}</em></span>
			{jira.accountEmail && <span className="wide"><strong>Account</strong><em>{jira.accountEmail}</em></span>}
			<span className="wide"><strong>Token env</strong><em>{jira.tokenEnvVar}</em></span>
		</dd>
	</div>;
}

function workspaceImportVisibleIssues(candidate: WorkspaceImportCandidate): WorkspaceImportCandidate['issues'] {
	if (candidate.status !== 'already_registered') return candidate.issues;
	return candidate.issues.filter((issue) => issue.field !== 'path');
}

function WorkspaceImportResults({ results }: { results: WorkspaceImportResult[] }) {
	return <section className="workspace-import-results" aria-label="Workspace import results" aria-live="polite">
		<header><h3>Import complete</h3><span>{results.filter((result) => result.workspace).length} registered</span></header>
		{results.length === 0 && <div className="empty-inline">No candidates were imported.</div>}
		<ul>{results.map((result) => <li key={result.candidateKey} className={`status-${result.status}`}>
			<span><strong>{result.workspace?.name ?? 'Selected candidate'}</strong><small>{result.message || workspaceImportResultLabel(result.status)}</small></span>
			<span className="workspace-import-status">{workspaceImportResultLabel(result.status)}</span>
		</li>)}</ul>
	</section>;
}

function workspaceImportStatusLabel(status: WorkspaceImportCandidate['status']): string {
	switch (status) {
	case 'already_registered': return 'Already registered';
	case 'duplicate': return 'Duplicate';
	case 'invalid': return 'Invalid';
	default: return 'Valid';
	}
}

function workspaceImportResultLabel(status: WorkspaceImportResult['status']): string {
	switch (status) {
	case 'indexed': return 'Indexed';
	case 'scan_failed': return 'Scan failed';
	case 'skipped': return 'Skipped';
	default: return 'Failed';
	}
}

function SourceStructureProposalList({
  proposals,
  selectedProposalId,
  onSelect,
  onClear
}: {
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  onSelect: (proposal: SourceStructureProposal) => void;
  onClear: () => void;
}) {
  if (proposals.length === 0) return null;
  return (
    <section className="source-proposals" aria-label="Source structure proposals">
      <div className="source-structure-section-heading">
        <strong>Suggested structures</strong>
        <span>Choose a structure, or keep the source unsorted.</span>
      </div>
      <div className="source-proposal-grid">
        <button className={selectedProposalId === UNSORTED_SELECTION_ID ? 'source-proposal-card active' : 'source-proposal-card'} type="button" onClick={onClear}>
          <strong>Unsorted</strong>
          <span>Keep this source as one unstructured item in the Unsorted lane.</span>
        </button>
        {proposals.map((proposal) => {
          const selected = selectedProposalId === proposal.id;
          return (
            <button className={selected ? 'source-proposal-card active' : 'source-proposal-card'} type="button" key={proposal.id} onClick={() => onSelect(proposal)}>
              <strong>{proposal.label}</strong>
              <span>{proposal.summary}</span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

function SourceStructurePreviewTable({ preview, onChangeField }: {
  preview: SourceStructurePreview[];
  onChangeField: (path: string, field: 'item' | 'title' | 'status', value: string) => void;
}) {
  const [mode, setMode] = useState<'table' | 'tree'>('table');
  return (
    <section className="source-preview" aria-label="Source structure preview">
      <div className="source-structure-section-heading">
        <strong>Item mapping</strong>
        <div className="source-preview-heading-actions">
          <span>{preview.length === 0 ? 'No matching card directories yet.' : `${preview.length} sample cards`}</span>
          {preview.length > 0 && (
            <button
              type="button"
              className="source-preview-mode-toggle"
              onClick={() => setMode((current) => current === 'table' ? 'tree' : 'table')}
            >
              {mode === 'table' ? 'Tree view' : 'Table view'}
            </button>
          )}
        </div>
      </div>
      {preview.length > 0 && mode === 'table' && (
        <div className="source-preview-table">
          <div className="source-preview-row heading">
            <span>Path</span>
            <span>Source</span>
            <span>Item</span>
            <span>Title</span>
            <span>Status</span>
          </div>
          {preview.map((row) => (
            <div className="source-preview-row" key={row.path}>
              <span title={row.path}>{row.path}</span>
              <span>{row.source ?? row.scope}</span>
              <span><input value={row.item ?? row.identifier ?? ''} onChange={(event) => onChangeField(row.path, 'item', event.target.value)} /></span>
              <span><input value={row.title ?? ''} onChange={(event) => onChangeField(row.path, 'title', event.target.value)} /></span>
              <span>
                <select value={row.status ?? 'draft'} onChange={(event) => onChangeField(row.path, 'status', event.target.value)}>
                  <option value="unsorted">Unsorted</option>
                  <option value="draft">Draft</option>
                  <option value="in_progress">In Progress</option>
                  <option value="review">Review</option>
                  <option value="done">Done</option>
                </select>
              </span>
            </div>
          ))}
        </div>
      )}
      {preview.length > 0 && mode === 'tree' && <SourcePreviewTree preview={preview} />}
    </section>
  );
}

type PreviewTreeNode = {
  name: string;
  path: string;
  row?: SourceStructurePreview;
  children: PreviewTreeNode[];
};

function SourcePreviewTree({ preview }: { preview: SourceStructurePreview[] }) {
  return (
    <div className="source-preview-tree" role="tree" aria-label="Source item tree preview">
      {buildSourcePreviewTree(preview).map((node) => <SourcePreviewTreeNodeView key={node.path} node={node} />)}
    </div>
  );
}

function SourcePreviewTreeNodeView({ node }: { node: PreviewTreeNode }) {
  return (
    <div className="source-preview-tree-node" role="treeitem" aria-label={node.path}>
      <span className="source-preview-tree-label">{node.name}</span>
      {node.row && (
        <small>
          item: {node.row.item ?? node.row.identifier} - title: {node.row.title} - status: {node.row.status}
        </small>
      )}
      {node.children.length > 0 && (
        <div className="source-preview-tree-children" role="group">
          {node.children.map((child) => <SourcePreviewTreeNodeView key={child.path} node={child} />)}
        </div>
      )}
    </div>
  );
}

function buildSourcePreviewTree(preview: SourceStructurePreview[]): PreviewTreeNode[] {
  type MutableTreeNode = PreviewTreeNode & { childMap: Map<string, MutableTreeNode> };
  const roots = new Map<string, MutableTreeNode>();
  for (const row of preview) {
    const segments = row.path.split('/').filter(Boolean);
    let pathSoFar = '';
    let scope = roots;
    let currentNode: MutableTreeNode | null = null;
    for (const segment of segments) {
      pathSoFar = pathSoFar ? `${pathSoFar}/${segment}` : segment;
      if (!scope.has(segment)) {
        scope.set(segment, { name: segment, path: pathSoFar, children: [], childMap: new Map() });
      }
      currentNode = scope.get(segment) ?? null;
      scope = currentNode?.childMap ?? new Map();
    }
    if (currentNode) currentNode.row = row;
  }

  const toImmutable = (nodes: Map<string, MutableTreeNode>): PreviewTreeNode[] => Array.from(nodes.values())
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { numeric: true, sensitivity: 'base' }))
    .map((node) => ({
      name: node.name,
      path: node.path,
      row: node.row,
      children: toImmutable(node.childMap)
    }));

  return toImmutable(roots);
}

function WorkspaceNoticePanel({ notice, onDismiss }: { notice: WorkspaceNotice; onDismiss: () => void }) {
  return (
    <section className={`workspace-notice ${notice.tone}`} role="status" aria-live="polite">
      <div>
        <strong>{notice.title}</strong>
        {notice.details && notice.details.length > 0 && (
          <ul>
            {notice.details.map((detail, index) => <li key={`${detail}-${index}`}>{detail}</li>)}
          </ul>
        )}
      </div>
      <button className="icon-button" type="button" onClick={onDismiss} aria-label="Dismiss notification">
        <X size={15} />
      </button>
    </section>
  );
}

function scanNotice(repo: WorkspaceConfig, result: ScanResult, title = 'Workspace scanned'): WorkspaceNotice {
  const warnings = scanWarnings(result);
  return {
    tone: warnings.length > 0 ? 'info' : 'success',
    title,
    details: [
      scanSummary(repo, result),
      ...warnings.slice(0, 3).map((warning) => `Warning${warning.itemPath ? ` (${warning.itemPath})` : ''}: ${warning.message}`)
    ]
  };
}

function sourceSettingsNotice(title: string, repo: WorkspaceConfig, scan?: ScanResult): WorkspaceNotice {
  return scan ? scanNotice(repo, scan, title) : { tone: 'success', title, details: [repo.name] };
}

function OverviewSection({ title, collapsed, onToggle, children }: { title: string; collapsed: boolean; onToggle: () => void; children: ReactNode }) {
  return <section className="workspace-overview-section">
    <button className="workspace-overview-section-toggle" type="button" onClick={onToggle} aria-expanded={!collapsed}>
      {collapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}
      <span>{title}</span>
    </button>
    {!collapsed && <div className="workspace-overview-section-content">{children}</div>}
  </section>;
}

function scanSummary(repo: WorkspaceConfig, result: ScanResult): string {
  const warningCount = scanWarnings(result).length;
  return `${repo.name}: ${result.itemCount} item${result.itemCount === 1 ? '' : 's'} indexed at ${formatScanTime(result.scannedAt)}${warningCount > 0 ? ` with ${warningCount} warning${warningCount === 1 ? '' : 's'}` : ''}.`;
}

function scanWarnings(result: ScanResult): ScanResult['warnings'] {
  return Array.isArray(result.warnings) ? result.warnings : [];
}

function formatScanTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : 'Unexpected error';
}

function SourcesField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const directories = parseSources(value);
  const customDirectories = directories.filter((directory) => !DEFAULT_SOURCES.includes(directory));

  return (
    <label className="repo-field">{labels.sources}
      <div className="directory-input">
        <div className="directory-chips">
          {DEFAULT_SOURCES.map((directory) => {
            const selected = directories.includes(directory);
            return (
              <button type="button" className={selected ? undefined : 'add-directory-chip'} key={directory} onClick={() => onChange(toggleSource(value, directory))}>
                {selected ? <X size={13} /> : <Plus size={13} />}
                {directory}
              </button>
            );
          })}
          {customDirectories.map((directory) => (
            <button type="button" key={directory} onClick={() => onChange(removeSource(value, directory))}>
              {directory}
              <X size={13} />
            </button>
          ))}
        </div>
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="Add source" />
      </div>
    </label>
  );
}

function BranchField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const normalized = value.trim().toLowerCase();
  return (
    <label className="repo-field">Base Branch
      <div className="directory-input">
        <div className="directory-chips branch-chips">
          <button type="button" className={normalized === 'master' ? undefined : 'add-directory-chip'} onClick={() => onChange('master')}>
            {normalized === 'master' ? <X size={13} /> : <Plus size={13} />}
            master
          </button>
          <button type="button" className={normalized === 'main' ? undefined : 'add-directory-chip'} onClick={() => onChange('main')}>
            {normalized === 'main' ? <X size={13} /> : <Plus size={13} />}
            main
          </button>
        </div>
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="Base branch" />
      </div>
    </label>
  );
}

export function buildWorkspaceInput(input: {
  name: string;
  registrationMode: 'local_path' | 'remote_clone';
  path: string;
  remoteUrl: string;
  cloneRoot: string;
  baselineBranch: string;
  sources: string;
  jira?: JiraConnection | null;
}): WorkspaceInput {
  const payload = {
    name: input.name,
    baselineBranch: input.baselineBranch,
    sources: parseSources(input.sources),
    registrationMode: input.registrationMode
  } as WorkspaceInput;
  if (input.jira) payload.jira = normalizeJiraDraft(input.jira);
  if (input.registrationMode === 'remote_clone') {
    payload.remoteUrl = input.remoteUrl.trim();
    if (input.cloneRoot.trim()) {
      payload.cloneRoot = input.cloneRoot.trim();
    }
    return payload;
  }
  payload.path = input.path.trim();
  return payload;
}

function normalizeJiraDraft(value: JiraConnection): JiraConnection {
  return { deploymentType: value.deploymentType, baseUrl: value.baseUrl.trim().replace(/\/$/, ''), projectKey: value.projectKey.trim().toUpperCase(), accountEmail: value.deploymentType === 'cloud' ? value.accountEmail?.trim() : undefined, tokenEnvVar: value.tokenEnvVar.trim() };
}

export function normalizeKnowledgeSettings(value: KnowledgeSettings): KnowledgeSettings {
	return { enabled: value.enabled !== false, enrichExecutable: value.enrichExecutable?.trim() ?? '', enrichArgs: [...(value.enrichArgs ?? [])] };
}

function cloneRuntimeConfig(runtime: WorkspaceRuntimeConfig | undefined): WorkspaceRuntimeConfig | null {
  if (!runtime) return null;
  return {
    ...runtime,
    commands: { ...runtime.commands, verify: { ...runtime.commands.verify } },
    healthChecks: [...(runtime.healthChecks ?? [])],
    artifacts: { paths: [...(runtime.artifacts?.paths ?? [])] },
    automation: runtime.automation ? { ...runtime.automation, artifactPaths: [...(runtime.automation.artifactPaths ?? [])] } : undefined
  };
}

function workspaceDetailSubtitle(tab: WorkspaceDetailTab): string {
  switch (tab) {
  case 'health':
    return 'Workspace health and diagnostics';
  case 'integrations':
    return 'Connected services';
  default:
    return 'General and sources';
  }
}

function isIntegrationEditSection(section: WorkspaceEditSection): section is 'jira' | 'knowledge' | 'runtime' {
  return section === 'jira' || section === 'knowledge' || section === 'runtime';
}

function integrationSaveLabel(section: 'jira' | 'knowledge' | 'runtime'): string {
  switch (section) {
  case 'jira':
    return 'Save Jira';
  case 'knowledge':
    return 'Save Knowledge';
  default:
    return 'Save runtime';
  }
}

function KnowledgeSettingsFields({ value, onChange }: { value: KnowledgeSettings; onChange: (value: KnowledgeSettings) => void }) {
	const args = value.enrichArgs ?? [];
	const updateArg = (index: number, argument: string) => onChange({ ...value, enrichArgs: args.map((current, currentIndex) => currentIndex === index ? argument : current) });
	const moveArg = (index: number, offset: number) => { const target=index+offset; if(target<0||target>=args.length)return; const next=[...args]; [next[index],next[target]]=[next[target],next[index]]; onChange({...value,enrichArgs:next}); };
	return <section className="knowledge-settings-fields" aria-label="Knowledge settings"><label className="repo-field jira-connection-toggle"><span className="jira-connection-title"><input type="checkbox" checked={value.enabled !== false} onChange={(event) => onChange({ ...value, enabled: event.target.checked })} />Knowledge Wiki detection</span></label><label className="repo-field">Enrichment executable<input aria-label="Knowledge enrichment executable" value={value.enrichExecutable ?? ''} onChange={(event) => onChange({ ...value, enrichExecutable: event.target.value })} placeholder="wiki-enrich" /><small>This is a command name or path, not a secret.</small></label><div className="repo-field"><span>Literal arguments</span>{args.map((argument,index)=><div className="path-input-row" key={index}><input aria-label={`Knowledge enrichment argument ${index+1}`} value={argument} onChange={(event)=>updateArg(index,event.target.value)} /><button className="secondary icon-action" type="button" aria-label={`Move argument ${index+1} up`} disabled={index===0} onClick={()=>moveArg(index,-1)}>↑</button><button className="secondary icon-action" type="button" aria-label={`Move argument ${index+1} down`} disabled={index===args.length-1} onClick={()=>moveArg(index,1)}>↓</button><button className="secondary icon-action" type="button" aria-label={`Remove argument ${index+1}`} onClick={()=>onChange({...value,enrichArgs:args.filter((_,currentIndex)=>currentIndex!==index)})}>×</button></div>)}<button className="secondary" type="button" onClick={()=>onChange({...value,enrichArgs:[...args,'']})}>Add argument</button><small>Arguments are passed exactly as listed. Shell expansion and environment values are not supported.</small></div></section>;
}

function RuntimeSettingsFields({ value, onChange }: { value: WorkspaceRuntimeConfig | null; onChange: (value: WorkspaceRuntimeConfig | null) => void }) {
  const runtime = value ?? defaultRuntimeConfig();
  const [activeRuntimeTab, setActiveRuntimeTab] = useState<'runtime' | 'verification'>('runtime');
  const [automationPickerBusy, setAutomationPickerBusy] = useState(false);
  const [automationPickerError, setAutomationPickerError] = useState('');
  const [verificationInputsEnabled, setVerificationInputsEnabled] = useState(true);
  const update = (patch: Partial<WorkspaceRuntimeConfig>) => onChange({ ...runtime, ...patch });
  const updateCommands = (patch: Partial<WorkspaceRuntimeConfig['commands']>) => onChange({ ...runtime, commands: { ...runtime.commands, ...patch } });
  const updateVerify = (patch: Partial<WorkspaceRuntimeConfig['commands']['verify']>) => onChange({ ...runtime, commands: { ...runtime.commands, verify: { ...runtime.commands.verify, ...patch } } });
  const healthChecks = runtime.healthChecks ?? [];
  const paths = runtime.artifacts?.paths ?? [];
  const automation = runtime.automation ?? defaultRuntimeConfig().automation!;
  const updateAutomation = (patch: Partial<NonNullable<WorkspaceRuntimeConfig['automation']>>) => update({ automation: { ...automation, ...patch } });
  const automationArtifactPaths = automation.artifactPaths ?? [];

  useEffect(() => {
    if (!value) {
      setVerificationInputsEnabled(true);
      return;
    }
    const verify = value.commands?.verify;
    const hasVerificationCommands = Boolean((verify?.smoke ?? '').trim() || (verify?.critical ?? '').trim() || (verify?.full ?? '').trim());
    setVerificationInputsEnabled(hasVerificationCommands);
  }, [value]);

  const browseAutomationRepository = async () => {
    setAutomationPickerBusy(true);
    setAutomationPickerError('');
    try {
      const selection = await api.selectDirectory();
      updateAutomation({ repositoryPath: selection.path });
    } catch (err) {
      setAutomationPickerError(errorMessage(err));
    } finally {
      setAutomationPickerBusy(false);
    }
  };

  return <section className="knowledge-settings-fields runtime-settings-fields" aria-label="Runtime settings">
    <div className="settings-subtabs" role="tablist" aria-label="Runtime and verification settings">
      <button type="button" role="tab" aria-selected={activeRuntimeTab === 'runtime'} className={activeRuntimeTab === 'runtime' ? 'active' : undefined} onClick={() => setActiveRuntimeTab('runtime')}>Runtime</button>
      <button type="button" role="tab" aria-selected={activeRuntimeTab === 'verification'} className={activeRuntimeTab === 'verification' ? 'active' : undefined} onClick={() => setActiveRuntimeTab('verification')}>Verification</button>
    </div>
    {activeRuntimeTab === 'runtime' && <>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={value !== null} onChange={(event) => onChange(event.target.checked ? runtime : null)} />
          Runtime
        </span>
      </label>
      {value && <>
        <div className="registration-mode-toggle" role="radiogroup" aria-label="Runtime type">
          {(['docker-compose', 'procfile', 'makefile', 'custom'] as RuntimeType[]).map((option) => (
            <button key={option} type="button" role="radio" aria-checked={runtime.type === option} className={runtime.type === option ? 'secondary active' : 'secondary'} onClick={() => update({ type: option })}>{option}</button>
          ))}
        </div>
        <label className="repo-field">Runtime config path<input value={runtime.configPath ?? ''} onChange={(event) => update({ configPath: event.target.value })} placeholder="infra/docker-compose.yaml" /></label>
        <label className="repo-field">Startup command<input value={runtime.commands.up} onChange={(event) => updateCommands({ up: event.target.value })} placeholder="docker compose up -d --no-build" /></label>
        <label className="repo-field">Teardown command<input value={runtime.commands.down} onChange={(event) => updateCommands({ down: event.target.value })} placeholder="docker compose down" /></label>
        <label className="repo-field">Changed-only rebuild command<input value={runtime.commands.rebuildChanged ?? ''} onChange={(event) => updateCommands({ rebuildChanged: event.target.value })} placeholder="docker compose build service-a service-b" /></label>
        <label className="repo-field">Rebuild policy<select value={runtime.rebuildPolicy ?? 'changed-only'} onChange={(event) => update({ rebuildPolicy: event.target.value as WorkspaceRuntimeConfig['rebuildPolicy'] })}><option value="never">Never</option><option value="changed-only">Changed only</option><option value="always">Always</option></select></label>
        <div className="repo-field"><span>Health checks</span>{healthChecks.map((check, index) => <div className="path-input-row" key={`${check.target}-${index}`}><select value={check.type} onChange={(event) => onChange({ ...runtime, healthChecks: healthChecks.map((current, currentIndex) => currentIndex === index ? { ...current, type: event.target.value as 'http' | 'command' } : current) })}><option value="http">HTTP</option><option value="command">Command</option></select><input value={check.target} onChange={(event) => onChange({ ...runtime, healthChecks: healthChecks.map((current, currentIndex) => currentIndex === index ? { ...current, target: event.target.value } : current) })} placeholder={check.type === 'http' ? 'http://localhost:3000/health' : 'curl -f http://localhost:3000/health'} /><button className="secondary icon-action" type="button" aria-label={`Remove health check ${index + 1}`} onClick={() => onChange({ ...runtime, healthChecks: healthChecks.filter((_, currentIndex) => currentIndex !== index) })}>×</button></div>)}<button className="secondary" type="button" onClick={() => onChange({ ...runtime, healthChecks: [...healthChecks, { type: 'http', target: '', timeoutSeconds: 30 }] })}>Add health check</button></div>
        <div className="repo-field"><span>Artifact paths</span>{paths.map((artifactPath, index) => <div className="path-input-row" key={`${artifactPath}-${index}`}><input value={artifactPath} onChange={(event) => onChange({ ...runtime, artifacts: { paths: paths.map((current, currentIndex) => currentIndex === index ? event.target.value : current) } })} placeholder="playwright-report" /><button className="secondary icon-action" type="button" aria-label={`Remove artifact path ${index + 1}`} onClick={() => onChange({ ...runtime, artifacts: { paths: paths.filter((_, currentIndex) => currentIndex !== index) } })}>×</button></div>)}<button className="secondary" type="button" onClick={() => onChange({ ...runtime, artifacts: { paths: [...paths, ''] } })}>Add artifact path</button></div>
      </>}
    </>}
    {activeRuntimeTab === 'verification' && <>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={verificationInputsEnabled} onChange={(event) => setVerificationInputsEnabled(event.target.checked)} />
          Runtime tests
        </span>
      </label>
      <fieldset className="runtime-settings-section-fields" disabled={!verificationInputsEnabled}>
        <div className="repo-field-grid">
          <label className="repo-field">Verify smoke<input value={runtime.commands.verify.smoke} onChange={(event) => updateVerify({ smoke: event.target.value })} placeholder="pnpm test:e2e --grep @smoke" /></label>
          <label className="repo-field">Verify critical<input value={runtime.commands.verify.critical ?? ''} onChange={(event) => updateVerify({ critical: event.target.value })} placeholder="pnpm test:e2e --grep @critical" /></label>
        </div>
        <label className="repo-field">Verify full<input value={runtime.commands.verify.full ?? ''} onChange={(event) => updateVerify({ full: event.target.value })} placeholder="pnpm test:e2e" /></label>
      </fieldset>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={automation.enabled} onChange={(event) => updateAutomation({ enabled: event.target.checked })} />
          Automation tests
        </span>
      </label>
      <fieldset className="runtime-settings-section-fields" disabled={!automation.enabled}>
        <div className="repo-field-grid automation-field-grid">
          <label className="repo-field automation-repository-field">Automation repository
            <div className="path-input-row automation-repository-row">
              <input value={automation.repositoryPath ?? ''} onChange={(event) => { setAutomationPickerError(''); updateAutomation({ repositoryPath: event.target.value }); }} placeholder="Repository path" />
              <button className="secondary" type="button" onClick={() => void browseAutomationRepository()} disabled={automationPickerBusy || !automation.enabled}><FolderOpen size={15} /> Browse</button>
            </div>
            {automationPickerError && <small className="settings-error">{automationPickerError}</small>}
          </label>
          <label className="repo-field">Automation runner<select value={automation.runner ?? 'cypress'} onChange={(event) => updateAutomation({ runner: event.target.value as NonNullable<WorkspaceRuntimeConfig['automation']>['runner'] })}><option value="cypress">Cypress</option><option value="playwright">Playwright</option></select></label>
        </div>
        <div className="repo-field-grid">
          <label className="repo-field">Default environment<input value={automation.defaultEnvironment ?? 'local'} onChange={(event) => updateAutomation({ defaultEnvironment: event.target.value })} placeholder="local" /></label>
          <label className="repo-field">Automation command template<input value={automation.commandTemplate ?? ''} onChange={(event) => updateAutomation({ commandTemplate: event.target.value })} placeholder={'CYPRESS_EPSAP_ENVIRONMENT={env} npx cypress run --spec "{specs}"'} /></label>
        </div>
        <div className="repo-field"><span>Automation artifact paths</span>{automationArtifactPaths.map((artifactPath, index) => <div className="path-input-row" key={`automation-${artifactPath}-${index}`}><input value={artifactPath} onChange={(event) => updateAutomation({ artifactPaths: automationArtifactPaths.map((current, currentIndex) => currentIndex === index ? event.target.value : current) })} placeholder="cypress/videos" /><button className="secondary icon-action" type="button" aria-label={`Remove automation artifact path ${index + 1}`} onClick={() => updateAutomation({ artifactPaths: automationArtifactPaths.filter((_, currentIndex) => currentIndex !== index) })}>×</button></div>)}<button className="secondary" type="button" onClick={() => updateAutomation({ artifactPaths: [...automationArtifactPaths, ''] })}>Add automation artifact path</button></div>
      </fieldset>
    </>}
  </section>;
}

function KnowledgeRoots({ workspaceId, enabled }: { workspaceId: string; enabled: boolean }) {
	const [roots, setRoots] = useState<string[]>([]);
	useEffect(() => {
		let active = true;
		if (!enabled) { setRoots([]); return; }
		const loadWikis = api.knowledgeWikis;
		if (typeof loadWikis !== 'function') { setRoots([]); return; }
		void loadWikis(workspaceId).then((wikis) => { if (active) setRoots(wikis.map((wiki) => wiki.root)); }).catch(() => { if (active) setRoots([]); });
		return () => { active = false; };
	}, [enabled, workspaceId]);
	return <small>{enabled ? roots.length ? `Detected: ${roots.join(', ')}` : 'No compatible Wiki roots detected yet.' : 'Detection is disabled.'}</small>;
}

function JiraConnectionFields({ value, onChange, workspaceId }: { value: JiraConnection | null; onChange: (value: JiraConnection | null) => void; workspaceId?: string }) {
  const [testing, setTesting] = useState(false);
  const [resultTone, setResultTone] = useState<'success' | 'error' | null>(null);
  const [result, setResult] = useState('');
  const update = (patch: Partial<JiraConnection>) => value && onChange({ ...value, ...patch });
  const test = async () => {
    if (!value || !workspaceId) return;
    setTesting(true); setResultTone(null); setResult('');
    try {
      const response = await api.testJiraConnection(workspaceId, normalizeJiraDraft(value));
      setResultTone('success');
      setResult(response.message);
    } catch (caught) {
      setResultTone('error');
      setResult(caught instanceof ApiError && caught.recoveryHint ? `${caught.message} ${caught.recoveryHint}` : errorMessage(caught));
    } finally {
      setTesting(false);
    }
  };
  return <section className="jira-connection-fields">
    <label className="repo-field jira-connection-toggle">
      <span className="jira-connection-title">
        <input type="checkbox" checked={value !== null} onChange={(event) => { setResultTone(null); setResult(''); onChange(event.target.checked ? emptyJiraConnection() : null); }} />
        Jira integration
      </span>
    </label>
    {value && <>
      <div className="registration-mode-toggle" role="radiogroup" aria-label="Jira deployment type"><button type="button" role="radio" aria-checked={value.deploymentType === 'cloud'} className={value.deploymentType === 'cloud' ? 'secondary active' : 'secondary'} onClick={() => update({ deploymentType: 'cloud' })}>Cloud</button><button type="button" role="radio" aria-checked={value.deploymentType === 'server'} className={value.deploymentType === 'server' ? 'secondary active' : 'secondary'} onClick={() => update({ deploymentType: 'server', accountEmail: '' })}>Server / Data Center</button></div>
      <div className="repo-field-grid"><label className="repo-field">Base URL<input aria-label="Jira base URL" value={value.baseUrl} onChange={(event) => update({ baseUrl: event.target.value })} placeholder="https://company.atlassian.net" /></label><label className="repo-field">Project Key<input aria-label="Jira project key" value={value.projectKey} onChange={(event) => update({ projectKey: event.target.value.toUpperCase() })} placeholder="DI" /></label></div>
      {value.deploymentType === 'cloud' && <label className="repo-field">Account Email<input aria-label="Jira account email" value={value.accountEmail ?? ''} onChange={(event) => update({ accountEmail: event.target.value })} /></label>}
      <label className="repo-field">Token Environment Variable<input aria-label="Jira token environment variable" value={value.tokenEnvVar} onChange={(event) => update({ tokenEnvVar: event.target.value })} /><small>Store the token in this environment variable before starting Kode Stream.</small></label>
      {workspaceId && <div className="jira-test-row"><button className="secondary" type="button" disabled={testing} onClick={() => void test()}>{testing ? 'Testing...' : 'Test Jira connection'}</button>{result && <span className={`jira-connection-status ${resultTone ?? 'success'}`} role="status"><span className={`jira-connection-status-dot ${resultTone ?? 'success'}`} aria-hidden="true" />{result}</span>}</div>}
    </>}
  </section>;
}

export function inferWorkspaceNameFromRemoteURL(remoteUrl: string): string {
  const value = remoteUrl.trim();
  if (!value) return '';
  const parsed = /[:/]([^/:?#]+?)(?:\.git)?$/.exec(value);
  return parsed?.[1] ?? '';
}

export function workspaceRemovalMessage(workspaces: WorkspaceConfig[]): string {
  if (workspaces.length === 0) return 'No workspaces selected.';
  if (workspaces.length === 1) {
    const [workspace] = workspaces;
    return `Remove ${workspace.name}? Cached items will be removed from the board${workspace.clonePathManaged ? ', and the managed cloned repository folder will be deleted.' : '.'}`;
  }
  const managedCloneCount = workspaces.filter((workspace) => workspace.clonePathManaged).length;
  return `Remove ${workspaces.length} selected workspaces? Cached items will be removed from the board${managedCloneCount > 0 ? `, and ${managedCloneCount} managed cloned repository folder${managedCloneCount === 1 ? '' : 's'} will be deleted.` : '.'}`;
}

function pathFromDrop(event: DragEvent<HTMLElement>): string {
  const explicitPath = event.dataTransfer.getData('text/plain').trim();
  const uriList = event.dataTransfer.getData('text/uri-list').split('\n').find((line) => line.trim() && !line.startsWith('#'))?.trim();
  const filePath = (event.dataTransfer.files[0] as (File & { path?: string }) | undefined)?.path;
  return normalizeDroppedPath(filePath || uriList || explicitPath);
}

function addSource(value: string, directory: string): string {
  return [...parseSources(value), directory].join(', ');
}

function removeSource(value: string, directory: string): string {
  return parseSources(value).filter((item) => item !== directory).join(', ');
}

function toggleSource(value: string, directory: string): string {
  const sources = parseSources(value);
  return sources.includes(directory) ? removeSource(value, directory) : addSource(value, directory);
}

export function settingsEditorFromResult(repo: WorkspaceConfig, directory: string, result: SourceSettingsResult): SettingsEditorState {
  const proposals = result.proposals ?? [];
  const selectedProposal = !result.exists && proposals.length > 0 ? proposals[0] : undefined;
  const unsortedPreview = [unsortedSourcePreview(directory)];
  const selectedProposalId = selectedProposal?.id ?? (!result.exists ? UNSORTED_SELECTION_ID : undefined);
  return {
    repo,
    directory,
    exists: result.exists,
    mode: result.mode,
    card: normalizeSettingsCard(selectedProposal?.card ?? result.settings?.cards?.[0], directory),
    warnings: (result.warnings ?? []).map((warning) => warning.message),
    proposals,
    selectedProposalId,
    unsortedPreview,
    preview: selectedProposal?.preview ?? (!result.exists ? unsortedPreview : result.preview ?? [])
  };
}

function unsortedSourcePreview(directory: string): SourceStructurePreview {
  const sourceName = lastPathSegment(directory) || 'source';
  return {
    path: directory,
    source: sourceName,
    item: sourceName,
    scope: sourceName,
    identifier: sourceName,
    title: sourceName,
    status: 'unsorted',
    tags: [sourceName]
  };
}

function normalizeSettingsCard(card?: SourceStructureCard, directory = 'source'): SourceStructureCard {
  const legacyFields = card?.fields as SourceStructureCard['fields'] & { service?: string; ticket?: string } | undefined;
  return withInferredCompatibilityFields({
    pathPattern: genericTemplate(card?.pathPattern || '{folder}/feature/{item}'),
    fields: {
      source: genericTemplate(legacyFields?.source || legacyFields?.scope || legacyFields?.service || directory),
      item: genericTemplate(legacyFields?.item || legacyFields?.identifier || legacyFields?.ticket || '{item}'),
      scope: genericTemplate(legacyFields?.source || legacyFields?.scope || legacyFields?.service || directory),
      identifier: genericTemplate(legacyFields?.item || legacyFields?.identifier || legacyFields?.ticket || '{item}'),
      title: card?.fields?.title || 'readme_heading',
      status: card?.fields?.status || 'draft',
      owner: card?.fields?.owner || '',
      tags: Array.isArray(card?.fields?.tags) ? card.fields.tags : ['docs']
    }
  }, directory);
}

function genericTemplate(value: string): string {
  return value
    .replaceAll('{service}', '{folder}')
    .replaceAll('{scope}', '{folder}')
    .replaceAll('{ticket}', '{item}')
    .replaceAll('{identifier}', '{item}');
}

function withInferredCompatibilityFields(card: SourceStructureCard, directory: string): SourceStructureCard {
  return {
    ...card,
    fields: {
      ...card.fields,
      source: inferCompatibilityFields(card.pathPattern, directory).scope,
      item: inferCompatibilityFields(card.pathPattern, directory).identifier,
      ...inferCompatibilityFields(card.pathPattern, directory)
    }
  };
}

function applySettingsProposal(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>,
  proposal: SourceStructureProposal
) {
  setSettingsEditor((current) => {
    if (!current) return current;
    return {
      ...current,
      card: normalizeSettingsCard(proposal.card, current.directory),
      selectedProposalId: proposal.id,
      preview: proposal.preview
    };
  });
}

function clearSettingsProposal(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>
) {
  setSettingsEditor((current) => current ? {
    ...current,
    selectedProposalId: UNSORTED_SELECTION_ID,
    preview: current.unsortedPreview
  } : current);
}

function updateSettingsPreviewField(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>,
  path: string,
  field: 'item' | 'title' | 'status',
  value: string
) {
  setSettingsEditor((current) => {
    if (!current) return current;
    const normalized = value.trim();
    const nextCard = { ...current.card, fields: { ...current.card.fields } };
    let nextPreview: SourceStructurePreview[] = current.preview.map((row) => ({
      ...row,
      item: row.path === path && field === 'item' ? value : row.item,
      identifier: row.path === path && field === 'item' ? value : row.identifier,
      title: row.path === path && field === 'title' ? value : row.title,
      status: row.path === path && field === 'status' ? value as SourceStructurePreview['status'] : row.status
    }));
    if (field === 'item') {
      nextCard.fields.item = normalized;
      nextCard.fields.identifier = normalized;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, true);
      if (suggestedTemplate) {
        nextCard.fields.item = suggestedTemplate;
        nextCard.fields.identifier = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? normalized : row.item ?? row.identifier);
          return { ...row, item: resolved, identifier: resolved };
        });
      }
    }
    if (field === 'title') {
      nextCard.fields.title = value;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, false);
      if (suggestedTemplate) {
        nextCard.fields.title = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? value : row.title);
          return { ...row, item: row.item, identifier: row.identifier, title: resolved };
        });
      }
    }
    if (field === 'status') {
      nextCard.fields.status = value;
    }

    return {
      ...current,
      selectedProposalId: undefined,
      card: nextCard,
      preview: nextPreview
    };
  });
}

function suggestTemplateFromValue(directory: string, pathPattern: string, rowPath: string, value: string, allowMultiSegment: boolean): string | null {
  if (!value) return null;
  const captures = pathPatternCaptures(directory, pathPattern, rowPath);
  if (!captures) return null;
  const segments = value.split('/').map((segment) => segment.trim()).filter(Boolean);
  if (segments.length === 0) return null;
  if (!allowMultiSegment && segments.length > 1) return null;

  const used = new Set<string>();
  const templateSegments: string[] = [];
  for (const segment of segments) {
    const options = Object.entries(captures)
      .filter(([name, value]) => value === segment && !used.has(name));
    if (options.length !== 1) return null;
    used.add(options[0][0]);
    templateSegments.push(`{${options[0][0]}}`);
  }
  return templateSegments.join('/');
}

function pathPatternCaptures(directory: string, pathPattern: string, rowPath: string): Record<string, string> | null {
  const patternSegments = pathPattern.split('/').map((segment) => segment.trim()).filter(Boolean);
  const rowSegments = previewPathSegments(rowPath, directory);
  if (patternSegments.length === 0 || patternSegments.length !== rowSegments.length) return null;

  const captures: Record<string, string> = {};
  for (let index = 0; index < patternSegments.length; index += 1) {
    const patternSegment = patternSegments[index];
    const rowSegment = rowSegments[index];
    const variable = patternSegment.match(/^\{([A-Za-z][A-Za-z0-9_]*)\}$/)?.[1];
    if (variable) {
      captures[variable] = rowSegment;
      continue;
    }
    if (patternSegment !== rowSegment) return null;
  }
  return captures;
}

function renderTemplateWithCaptures(template: string, captures: Record<string, string>): string {
  return Object.entries(captures).reduce((result, [name, value]) => result.replaceAll(`{${name}}`, value), template).trim();
}
