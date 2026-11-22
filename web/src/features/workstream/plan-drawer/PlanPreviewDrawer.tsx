import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Bot, PanelRightClose, PanelRightOpen, Code2, FileText, GitBranch, GripVertical, Info, RefreshCw, Ticket, X } from 'lucide-react';
import { FileMenu } from '../../../components/FileMenu';
import { useResizableDrawer } from '../../../components/useResizableDrawer';
import { writePreference, readTogglePreference } from '../../../shared/preferences/store';
import { ItemGitPanel } from '../../item-git/ItemGitPanel';
import { useItemGit } from '../../item-git/useItemGit';
import { StatusMenu } from '../../../components/StatusMenu';
import { ContentViewer } from '../../content-viewer/ContentViewer';
import { JiraItemPanel } from '../../jira/JiraItemPanel';
import { api, statusLabels } from '../../../shared/api';
import type { FileContent, FileNode, ItemDetail, ItemMetadataUpdateInput, ItemStatus } from '../../../lib/types';
import { isDocumentationMetadataSource, labels, metadataSourceLabel as genericMetadataSourceLabel } from '../../../lib/vocabulary';
import { notifyReliabilityChanged } from '../../reliability/hooks';
import { AgentPane, AgentsTab, agentFocusKey, useHeldAgentSessions, type AgentFocus, type AgentPaneHarness, type AgentsMode } from '../AgentsPanel';
import { HarnessRunTab } from '../HarnessRunTab';
import { takeOverByHand } from '../takeOver';
import { type HarnessController } from '../../harness/useHarness';
import { type ItemAgents } from '../useItemSessions';
import { type CardWork } from '../harnessJoin';
import { clearTimeoutRef, operationErrorMessage, sameJSON } from '../workstreamUtils';
import { type DrawerTab, type DrawerSideTab, type DrawerFileOption } from './drawerTypes';

export function PlanPreviewDrawer({ itemId, refreshKey, workspaceId, agents, agentFocus, onAgentFocus, onAgentsChanged, work, harness, onRunWithHarness, onClose, onOpenFull, onOpenRun, onChanged }: { itemId: string; refreshKey: number; workspaceId?: string; agents?: ItemAgents; agentFocus?: AgentFocus; onAgentFocus: (focus: AgentFocus | undefined) => void; onAgentsChanged: () => Promise<unknown> | void; work?: CardWork; harness: HarnessController; /** Offered while the harness is on and does not hold this card. */ onRunWithHarness?: () => void; onClose: () => void; onOpenFull: () => void; onOpenRun: () => void; onChanged: () => void | Promise<void> }) {
  const [agentsMode, setAgentsMode] = useState<AgentsMode>('run');
  const harnessRun = work?.run ? <HarnessRunTab work={work} harness={harness} onOpenConversation={(id) => onAgentFocus({ kind: 'harness', id })} onOpenPlan={(id) => onAgentFocus({ kind: 'harness-plan', id })} onOpenFull={onOpenRun} onTakeOver={() => void takeOverByHand(work, harness).then(({ agentRunId }) => { if (agentRunId) { onAgentFocus({ kind: 'harness', id: agentRunId }); } else { setAgentsMode('sessions'); onAgentFocus({ kind: 'launch' }); } })} /> : undefined;
  const paneHarness: AgentPaneHarness = { client: harness.client, run: work?.run, detail: work?.run ? harness.state.details[work.run.id] : undefined, reload: () => (work?.run ? harness.loadRun(work.run.id) : Promise.resolve()) };
  const [plan, setPlan] = useState<ItemDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [editorContent, setEditorContent] = useState('');
  const [savedContent, setSavedContent] = useState('');
  const [savingFile, setSavingFile] = useState(false);
  const [autoSaveState, setAutoSaveState] = useState<'idle' | 'pending' | 'saving' | 'saved' | 'error'>('idle');
  const [metadataDraft, setMetadataDraft] = useState<ItemMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<DrawerTab>('preview');
  const [sideTab, setSideTab] = useState<DrawerSideTab>(() => agentFocus ? 'agents' : 'info');
  // Collapsed, the Work Item panel shrinks to a rail of tab icons and the main
  // pane takes its width; the choice is remembered across drawers.
  const [workItemCollapsed, setWorkItemCollapsed] = useState(() => readTogglePreference('workstream.drawer.workItemCollapsed'));
  const collapseWorkItem = (collapsed: boolean) => {
    setWorkItemCollapsed(collapsed);
    writePreference('workstream.drawer.workItemCollapsed', collapsed ? '1' : '0');
  };
  const openSideTab = (tab: DrawerSideTab) => {
    setSideTab(tab);
    if (workItemCollapsed) collapseWorkItem(false);
  };
  // Live sessions opened in this drawer stay connected until it closes or
  // moves to another card, then go to the terminal dock.
  const { heldLiveSessionIds, release: releaseHeldSession } = useHeldAgentSessions(agents, agentFocus, itemId);
  const [error, setError] = useState('');
  const { width, style: drawerStyle, startResize } = useResizableDrawer({ initial: 1120 });
  const autoSaveTimerRef = useRef<number | null>(null);
  // The Raw editor grows with the document so the whole file shows, as the
  // preview does, and the drawer scrolls instead of a small box.
  const rawEditorRef = useRef<HTMLTextAreaElement | null>(null);
  const autoSaveSettledTimerRef = useRef<number | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const drawerGenerationRef = useRef(0);
  const fileLoadAbortRef = useRef<AbortController | null>(null);
  const autoSaveRefreshAbortRef = useRef<AbortController | null>(null);
  const compact = width < 700;
  const dirtyFile = file !== null && editorContent !== savedContent;
  const activityPath = plan?.itemPath || '';
  const git = useItemGit({
    workspaceId: plan?.workspaceId,
    activityPath,
    onChanged: async (operation) => {
      await onChanged();
      if (operation !== 'commit') return;
      api.diff(itemId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
    },
    onProblem: (problem) => setError('result' in problem ? problem.result.message ?? '' : operationErrorMessage(problem.caught, problem.fallback))
  });
  const fileOptions = useMemo(() => flattenFileOptions(files), [files]);
  const dirtyMetadata = Boolean(plan) && (
    (metadataDraft.title ?? '') !== (plan?.title ?? '') ||
    (metadataDraft.scope ?? '') !== (plan?.scope ?? '') ||
    (metadataDraft.identifier ?? '') !== (plan?.identifier ?? '') ||
    (metadataDraft.status ?? '') !== (plan?.status ?? '') ||
    (metadataDraft.owner ?? '') !== (plan?.owner ?? '') ||
    (metadataDraft.tags ?? []).join('\n') !== (plan?.tags ?? []).join('\n')
  );

  const clearDrawerAutoSaveTimers = () => {
    clearTimeoutRef(autoSaveTimerRef);
    clearTimeoutRef(autoSaveSettledTimerRef);
    clearTimeoutRef(autoSaveRefreshTimerRef);
  };

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      // Escape typed into an agent's terminal belongs to the agent (it
      // interrupts Claude, for one); it must not close the drawer.
      if (event.target instanceof Element && event.target.closest('.agent-pane-body')) return;
      closeDrawer();
    };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [agentFocus, agents, dirtyFile, editorContent, file, onClose, savedContent]);

  useLayoutEffect(() => {
    const editor = rawEditorRef.current;
    if (tab !== 'raw' || !editor) return;
    editor.style.height = 'auto';
    editor.style.height = `${editor.scrollHeight + 2}px`;
  }, [tab, editorContent, width]);

  useEffect(() => {
    if (agentFocus) setSideTab('agents');
  }, [agentFocusKey(agentFocus)]);

  useEffect(() => {
    const controller = new AbortController();
    const generation = ++drawerGenerationRef.current;
    fileLoadAbortRef.current?.abort();
    autoSaveRefreshAbortRef.current?.abort();
    setPlan(null);
    setFile(null);
    setFiles([]);
    setDiff('');
    setError('');
    api.item(itemId, controller.signal).then((payload) => {
      if (!controller.signal.aborted) setPlan(payload);
    }).catch((err: Error) => {
      if (!controller.signal.aborted) setError(err.message);
    });
    api.files(itemId, undefined, controller.signal).then(async (tree) => {
      if (controller.signal.aborted) return;
      setFiles(tree);
      const previewFile = preferredPreviewFile(tree);
      if (previewFile) {
        try {
          const content = await api.file(itemId, previewFile.id, undefined, controller.signal);
          if (!controller.signal.aborted) {
            setFile(content);
            setEditorContent(content.content);
            setSavedContent(content.content);
            setAutoSaveState('idle');
          }
        } catch (err) {
          if (!controller.signal.aborted) setError(err instanceof Error ? err.message : 'File failed to load');
        }
      }
    }).catch((err: Error) => {
      if (!controller.signal.aborted) setError(err.message);
    });
    api.diff(itemId, controller.signal).then((payload) => {
      if (!controller.signal.aborted) setDiff(payload.diff || 'No local changes.');
    }).catch(() => {
      if (!controller.signal.aborted) setDiff('No diff available.');
    });
    return () => {
      controller.abort();
      if (drawerGenerationRef.current === generation) drawerGenerationRef.current += 1;
      fileLoadAbortRef.current?.abort();
      autoSaveRefreshAbortRef.current?.abort();
      clearDrawerAutoSaveTimers();
    };
  }, [itemId]);

  // A refreshKey bump is the app refreshing (on window focus, among others),
  // not a new item: refresh the item, its files, diff and git status in place,
  // and leave the open file and any unsaved edit alone.
  const refreshKeyLoadedRef = useRef(refreshKey);
  useEffect(() => {
    if (refreshKeyLoadedRef.current === refreshKey) return;
    refreshKeyLoadedRef.current = refreshKey;
    const controller = new AbortController();
    const generation = drawerGenerationRef.current;
    const isCurrent = () => !controller.signal.aborted && drawerGenerationRef.current === generation;
    api.item(itemId, controller.signal).then((payload) => {
      if (!isCurrent()) return;
      // An unchanged item keeps its identity, so the Info tab's draft survives.
      setPlan((current) => sameJSON(current, payload) ? current : payload);
      void git.loadStatus();
    }).catch(() => undefined);
    api.files(itemId, undefined, controller.signal).then((tree) => { if (isCurrent()) setFiles((current) => sameJSON(current, tree) ? current : tree); }).catch(() => undefined);
    api.diff(itemId, controller.signal).then((payload) => { if (isCurrent()) setDiff(payload.diff || 'No local changes.'); }).catch(() => undefined);
    return () => controller.abort();
  }, [refreshKey]);

  useEffect(() => {
    if (!plan) return;
    setMetadataDraft({
      title: plan.title,
      scope: plan.scope,
      identifier: plan.identifier,
      status: plan.status,
      owner: plan.owner ?? '',
      tags: plan.tags
    });
  }, [plan]);

  useEffect(() => {
    if (!file) {
      setAutoSaveState('idle');
      return;
    }
    if (editorContent === savedContent) {
      if (autoSaveState === 'pending') setAutoSaveState('idle');
      return;
    }
    if (savingFile) {
      setAutoSaveState('pending');
      return;
    }
    clearTimeoutRef(autoSaveTimerRef);
    setAutoSaveState('pending');
    autoSaveTimerRef.current = window.setTimeout(() => {
      void saveDrawerFile(file, editorContent);
    }, 900);
    return () => clearTimeoutRef(autoSaveTimerRef);
  }, [file, editorContent, savedContent, savingFile]);

  const saveDrawerFileNow = async () => {
    if (!file || editorContent === savedContent) return true;
    return saveDrawerFile(file, editorContent);
  };

  const saveDrawerFile = async (targetFile: FileContent, content: string) => {
    const generation = drawerGenerationRef.current;
    const activeItemId = itemId;
    const isCurrent = () => drawerGenerationRef.current === generation;
    clearTimeoutRef(autoSaveTimerRef);
    clearTimeoutRef(autoSaveSettledTimerRef);
    setSavingFile(true);
    setAutoSaveState('saving');
    setError('');
    try {
      const updated = await api.saveFile(activeItemId, targetFile.id, { content, expectedHash: targetFile.hash });
      notifyReliabilityChanged();
      if (!isCurrent()) return false;
      setFile(updated);
      setSavedContent(content);
      setAutoSaveState('saved');
      autoSaveSettledTimerRef.current = window.setTimeout(() => setAutoSaveState('idle'), 1400);
      clearTimeoutRef(autoSaveRefreshTimerRef);
      autoSaveRefreshTimerRef.current = window.setTimeout(() => {
        if (!isCurrent()) return;
        autoSaveRefreshAbortRef.current?.abort();
        const controller = new AbortController();
        autoSaveRefreshAbortRef.current = controller;
        api.diff(activeItemId, controller.signal)
          .then((payload) => {
            if (!controller.signal.aborted && isCurrent()) setDiff(payload.diff || 'No local changes.');
          })
          .catch(() => {
            if (!controller.signal.aborted && isCurrent()) setDiff('No diff available.');
          });
        if (isCurrent()) void git.loadStatus();
      }, 600);
      return true;
    } catch (err) {
      if (!isCurrent()) return false;
      setError(operationErrorMessage(err, 'File save failed'));
      setAutoSaveState('error');
      return false;
    } finally {
      if (isCurrent()) setSavingFile(false);
    }
  };

  const loadDrawerFile = async (fileId: string) => {
    fileLoadAbortRef.current?.abort();
    const controller = new AbortController();
    fileLoadAbortRef.current = controller;
    const generation = drawerGenerationRef.current;
    try {
      const content = await api.file(itemId, fileId, undefined, controller.signal);
      if (controller.signal.aborted || drawerGenerationRef.current !== generation) return;
      setFile(content);
      setEditorContent(content.content);
      setSavedContent(content.content);
      setAutoSaveState('idle');
    } catch (err) {
      if (!controller.signal.aborted && drawerGenerationRef.current === generation) setError(err instanceof Error ? err.message : 'File failed to load');
    } finally {
      if (fileLoadAbortRef.current === controller) fileLoadAbortRef.current = null;
    }
  };

  const selectDrawerFile = async (fileId: string) => {
    if (!fileId || fileId === file?.id) return;
    if (dirtyFile && !(await saveDrawerFileNow())) return;
    await loadDrawerFile(fileId);
  };

  const closeDrawer = () => {
    if (!dirtyFile) {
      onClose();
      return;
    }
    void saveDrawerFileNow().then((saved) => {
      if (saved) onClose();
    });
  };

  const openFullDetails = () => {
    if (!dirtyFile) {
      onOpenFull();
      return;
    }
    void saveDrawerFileNow().then((saved) => {
      if (saved) onOpenFull();
    });
  };

  const saveMetadata = async () => {
    if (!plan) return;
    const generation = drawerGenerationRef.current;
    const isCurrent = () => drawerGenerationRef.current === generation;
    setSavingMetadata(true);
    setError('');
    try {
		const result = await api.saveMetadata(itemId, { ...metadataDraft, expectedRevision: plan.metadataRevision });
      notifyReliabilityChanged();
      if (!isCurrent()) return;
      setPlan(result.item);
      await git.loadStatus();
      if (!isCurrent()) return;
      await onChanged();
    } catch (err) {
      if (isCurrent()) setError(operationErrorMessage(err, 'Metadata save failed'));
    } finally {
      if (isCurrent()) setSavingMetadata(false);
    }
  };

  return (
    <>
      <button className="drawer-scrim" style={drawerStyle} type="button" aria-label="Close preview" onClick={closeDrawer} />
      <aside className={compact ? 'plan-drawer compact' : 'plan-drawer'} style={drawerStyle} aria-label="Item preview">
        <button className="drawer-resize-handle" type="button" aria-label="Resize preview panel" onPointerDown={startResize}>
          <GripVertical size={16} />
        </button>
        <header className="plan-drawer-header">
          <div>
            <span className="drawer-kicker">{plan?.identifier ?? 'Loading'}</span>
            <h2>{plan?.title ?? 'Loading item'}</h2>
            <p>{plan ? `${plan.scope} / ${plan.branch}` : ''}</p>
          </div>
          <div className="drawer-actions">
            {onRunWithHarness && !work && <button type="button" className="secondary" onClick={onRunWithHarness}><Bot size={14} /> Run with the harness</button>}
            <button type="button" className="secondary drawer-open-details" onClick={openFullDetails}>Open details</button>
            <button type="button" className="icon-button" aria-label="Close preview" onClick={closeDrawer}><X size={16} /></button>
          </div>
        </header>
        {error && <p className="error drawer-error">{error}</p>}
        <div className={['plan-drawer-body', agentFocus || sideTab === 'agents' ? 'agents-open' : '', workItemCollapsed ? 'work-item-collapsed' : ''].filter(Boolean).join(' ')}>
          {(agentFocus || heldLiveSessionIds.length > 0) && (
          <section className="drawer-main drawer-agent-main" hidden={!agentFocus}>
            <AgentPane focus={agentFocus} heldSessionIds={heldLiveSessionIds} itemId={itemId} workspaceId={workspaceId ?? plan?.workspaceId} agents={agents} harness={paneHarness} onBack={() => onAgentFocus(undefined)} onFocus={onAgentFocus} onRelease={releaseHeldSession} onReload={onAgentsChanged} />
          </section>
          )}
          {agentFocus ? null : (
          <section className="drawer-main">
            <section className="drawer-section">
              <label className="drawer-file-picker">
                <span>Document</span>
                <FileMenu value={file?.id ?? ''} options={fileOptions} tree={files} onChange={selectDrawerFile} />
              </label>
              <div className="drawer-tabs">
                <div className="segmented-control segmented-control-compact" role="tablist" aria-label="Document view">
                  <button className={tab === 'preview' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'preview'} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
                  <button className={tab === 'raw' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'raw'} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
                  <button className={tab === 'diff' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'diff'} onClick={() => setTab('diff')}><RefreshCw size={15} /> Diff</button>
                </div>
                <span className={`autosave-state ${autoSaveState}`}>{autoSaveLabel(autoSaveState)}</span>
              </div>
              {dirtyFile && <div className="edit-state-banner">{autoSaveLabel(autoSaveState)}</div>}
              {tab === 'preview' && (file ? <ContentViewer file={file} content={editorContent} compact modeSwitch={false} /> : <div className="drawer-empty">No readable file selected.</div>)}
              {tab === 'raw' && (
                <textarea
                  ref={rawEditorRef}
                  className="drawer-raw drawer-raw-editor"
                  value={file ? editorContent : 'No readable file selected.'}
                  onChange={(event) => setEditorContent(event.target.value)}
                  disabled={!file || !file.editable}
                  spellCheck={false}
                />
              )}
              {tab === 'diff' && <pre className="drawer-raw">{diff || 'Loading diff...'}</pre>}
              <div className="drawer-file-note">{file?.path ?? 'No file selected'}</div>
            </section>
          </section>
          )}
          {workItemCollapsed ? (
          <aside className="drawer-work-item-rail" aria-label="Work item">
            <button type="button" className="icon-button" aria-label="Expand work item panel" title="Expand work item panel" onClick={() => collapseWorkItem(false)}><PanelRightOpen size={16} /></button>
            <span className="drawer-work-item-rail-divider" aria-hidden="true" />
            {([['info', Info, 'Info'], ['git', GitBranch, 'Git'], ['jira', Ticket, 'Jira'], ['agents', Bot, 'Agents']] as const).map(([tab, Icon, label]) => (
              <button key={tab} type="button" className={sideTab === tab ? 'icon-button active' : 'icon-button'} aria-label={`Open ${label}`} title={label} onClick={() => openSideTab(tab)}>
                <Icon size={16} />{tab === 'agents' && (agents?.live.length ?? 0) > 0 && <span className="side-tab-count" aria-hidden="true">{agents?.live.length}</span>}
              </button>
            ))}
          </aside>
          ) : (
          <aside className="drawer-meta drawer-work-item">
            <h3><Info size={15} /> Work Item<button type="button" className="icon-button drawer-work-item-collapse" aria-label="Collapse work item panel" title="Collapse work item panel" onClick={() => collapseWorkItem(true)}><PanelRightClose size={15} /></button></h3>
            <div className="side-panel-tabs" role="tablist" aria-label="Work item side panel">
              <button type="button" className={sideTab === 'info' ? 'active' : ''} onClick={() => setSideTab('info')}>
                <Info size={14} /> Info
              </button>
              <button type="button" className={sideTab === 'git' ? 'active' : ''} onClick={() => setSideTab('git')}>
                <GitBranch size={14} /> Git
              </button>
              <button type="button" className={sideTab === 'jira' ? 'active' : ''} onClick={() => setSideTab('jira')}>
                <Ticket size={14} /> Jira
              </button>
              <button type="button" className={sideTab === 'agents' ? 'active' : ''} onClick={() => setSideTab('agents')}>
                <Bot size={14} /> Agents{(agents?.live.length ?? 0) > 0 && <span className="side-tab-count" aria-label={`${agents?.live.length} live`}>{agents?.live.length}</span>}
              </button>
            </div>
            <div className="drawer-work-item-content">
              {sideTab === 'info' && (
                <>
                  <dl>
                    <dt>{labels.workspace}</dt><dd>{plan?.workspaceName ?? '-'}</dd>
                    <dt>{labels.source}</dt><dd>{plan ? plan.itemPath?.split('/').filter(Boolean)[0] || plan.scope || '-' : '-'}</dd>
                    <dt>{labels.identifier}</dt><dd>{plan?.identifier ?? '-'}</dd>
                    <dt>Branch</dt><dd>{plan?.branch ?? '-'}</dd>
                    <dt>Status</dt><dd>{plan?.status ? <DrawerStatusBadge status={plan.status} /> : '-'}</dd>
                    <dt>Metadata</dt><dd>{metadataSourceLabel(plan?.metadataSource)}</dd>
                    <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                    <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
                  </dl>
                  {!isDocumentationMetadataSource(plan?.metadataSource) && (
                    <div className="metadata-form drawer-metadata-form">
                      <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
                      <label>{labels.identifier}<input value={metadataDraft.identifier ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, identifier: event.target.value }))} /></label>
                      <label>Status<StatusMenu value={metadataDraft.status ?? 'draft'} onChange={(status) => setMetadataDraft((draft) => ({ ...draft, status }))} /></label>
                      <label>Owner<input value={metadataDraft.owner ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, owner: event.target.value }))} /></label>
                      <label>Tags<input value={(metadataDraft.tags ?? []).join(', ')} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label>
                    </div>
                  )}
                  <div className="workspace-actions">
                    <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || isDocumentationMetadataSource(plan?.metadataSource)} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
                  </div>
                  {(plan?.tags?.length ?? 0) > 0 && <div className="tags">{plan?.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
                </>
              )}
              {sideTab === 'git' && (
                <ItemGitPanel git={git} className="drawer-git-panel" activityOpenPreference="workstream.drawer.gitActivityOpen" branchPlaceholder="new-branch-name" unavailable={{ className: 'metadata-callout', hint: 'Open details to run Git actions.' }} />
              )}
              {sideTab === 'jira' && <JiraItemPanel itemId={itemId} />}
              {sideTab === 'agents' && <AgentsTab itemId={itemId} agents={agents} focus={agentFocus} onFocus={onAgentFocus} onError={setError} harnessRun={harnessRun} mode={agentsMode} onModeChange={setAgentsMode} onLaunched={(result) => { void Promise.resolve(onAgentsChanged()).then(() => onAgentFocus({ kind: 'session', id: result.session.id })); }} />}
            </div>
          </aside>
          )}
        </div>
      </aside>
    </>
  );
}

export function firstFile(nodes: FileNode[]): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file') return node;
    const child = firstFile(node.children ?? []);
    if (child) return child;
  }
  return null;
}

export function preferredPreviewFile(nodes: FileNode[]): FileNode | null {
  return findReadme(nodes, true) ?? findReadme(nodes, false) ?? firstFile(nodes);
}

export function flattenFileOptions(nodes: FileNode[], parentPath = ''): DrawerFileOption[] {
  return nodes.flatMap((node) => {
    const path = parentPath ? `${parentPath}/${node.name}` : node.name;
    if (node.type === 'file') {
      return [{ id: node.id, path: node.path || path, label: node.path || path }];
    }
    return flattenFileOptions(node.children ?? [], path);
  });
}

export function findReadme(nodes: FileNode[], rootOnly: boolean): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file' && node.name.toLowerCase() === 'readme.md') return node;
    if (!rootOnly && node.type === 'directory') {
      const child = findReadme(node.children ?? [], false);
      if (child) return child;
    }
  }
  return null;
}

export function metadataSourceLabel(source?: string): string {
  return genericMetadataSourceLabel(source);
}

export function DrawerStatusBadge({ status }: { status: ItemStatus }) {
  return <span className={`status-badge ${status}`}>{statusLabels[status]}</span>;
}

export function autoSaveLabel(state: 'idle' | 'pending' | 'saving' | 'saved' | 'error'): string {
  switch (state) {
    case 'pending':
      return 'Autosave pending';
    case 'saving':
      return 'Saving...';
    case 'saved':
      return 'Saved';
    case 'error':
      return 'Autosave failed';
    case 'idle':
    default:
      return 'Autosave on';
  }
}
