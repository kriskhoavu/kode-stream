import { type DragEvent, type Dispatch, type FormEvent, type SetStateAction, useState } from 'react';
import { CheckCircle2, ExternalLink, FolderGit2, FolderOpen, HardDrive, Pencil, Plus, RotateCw, SlidersHorizontal, Trash2, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { WorkspaceHealthPanel } from '../components/ReliabilityPanels';
import { api } from '../lib/api';
import type { WorkspaceConfig, SourceStructureSettings, SourceStructureCard, SourceStructurePreview, SourceStructureProposal, SourceSettingsResult, ScanResult } from '../lib/types';
import { labels } from '../lib/vocabulary';
import { applySegmentRole, inferCompatibilityFields, lastPathSegment, normalizeDroppedPath, parseSources, previewPathSegments } from '../features/workspaces/sourceSettings';
import { notifyReliabilityChanged } from '../features/reliability/hooks';

export { applySegmentRole, inferCompatibilityFields, normalizeDroppedPath, parseSources, previewPathSegments };

const DEFAULT_SOURCES = ['docs', 'plans'];
const UNSORTED_SELECTION_ID = 'unsorted';
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

export function WorkspacesPage({ workspaces, onChanged }: { workspaces: WorkspaceConfig[]; onChanged: () => void | Promise<void> }) {
  const [name, setName] = useState('Plan Manager');
  const [path, setPath] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('main');
  const [sources, setSources] = useState('');
  const [notice, setNotice] = useState<WorkspaceNotice | null>(null);
  const [busy, setBusy] = useState(false);
  const [pathDragging, setPathDragging] = useState(false);
  const [editingId, setEditingId] = useState('');
  const [editDraft, setEditDraft] = useState({ name: '', path: '', baselineBranch: '', sources: '' });
  const [repoToRemove, setRepoToRemove] = useState<WorkspaceConfig | null>(null);
  const [settingsEditor, setSettingsEditor] = useState<SettingsEditorState | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNotice(null);
    try {
      await api.createWorkspace({
        name,
        path,
        baselineBranch,
        sources: parseSources(sources)
      });
      setNotice({ tone: 'success', title: 'Workspace registered', details: [name || 'New workspace'] });
      setName('');
      setPath('');
      setBaselineBranch('main');
      setSources('');
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: 'Registration failed', details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const scan = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setNotice({ tone: 'info', title: `Scanning ${repo.name}` });
    try {
      const result = await api.scan(repo.id);
      notifyReliabilityChanged();
      setNotice(scanNotice(repo, result));
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: `Scan failed for ${repo.name}`, details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const scanAll = async () => {
    if (workspaces.length === 0) return;
    setBusy(true);
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
      setBusy(false);
    }
  };

  const startEdit = (repo: WorkspaceConfig) => {
    setEditingId(repo.id);
    setEditDraft({
      name: repo.name,
      path: repo.path,
      baselineBranch: repo.baselineBranch,
      sources: repo.sources.join(', ')
    });
    setNotice(null);
  };

  const saveEdit = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setNotice(null);
    try {
      await api.updateWorkspace(repo.id, {
        name: editDraft.name,
        path: editDraft.path,
        baselineBranch: editDraft.baselineBranch,
        sources: parseSources(editDraft.sources)
      });
      setEditingId('');
      setNotice({ tone: 'success', title: 'Workspace updated', details: [editDraft.name || repo.name] });
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: `Update failed for ${repo.name}`, details: [errorMessage(err)] });
    } finally {
      setBusy(false);
    }
  };

  const removeRepo = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setNotice(null);
    try {
      await api.deleteWorkspace(repo.id);
      setEditingId('');
      setNotice({ tone: 'success', title: 'Workspace removed', details: [repo.name] });
      onChanged();
    } catch (err) {
      setNotice({ tone: 'error', title: `Remove failed for ${repo.name}`, details: [errorMessage(err)] });
    } finally {
      setBusy(false);
      setRepoToRemove(null);
    }
  };

  const browsePath = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const selection = await api.selectDirectory();
      setPath(selection.path);
      if (!name || name === 'Plan Manager') {
        setName(lastPathSegment(selection.path));
      }
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
    event.preventDefault();
    setPathDragging(false);
    const droppedPath = pathFromDrop(event);
    if (droppedPath) {
      setPath(droppedPath);
      if (!name || name === 'Plan Manager') {
        setName(lastPathSegment(droppedPath));
      }
      return;
    }
    setNotice({ tone: 'error', title: 'Drop a folder path or file URL' });
  };

  return (
    <section className="workspaces-page">
      <div className="page-title">
        <div>
          <h1>Workspaces</h1>
          <span>Register local Git workspaces and scan sources.</span>
        </div>
      </div>

      <div className="workspaces-layout">
        <form className="repo-form repo-create-panel" onSubmit={submit}>
          <header>
            <FolderGit2 size={18} />
            <h2>Register Workspace</h2>
          </header>
          <label className="repo-field">Workspace Name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="Discovery" /></label>
          <label
            className={pathDragging ? 'repo-field path-field dragging' : 'repo-field path-field'}
            onDragOver={(event) => {
              event.preventDefault();
              setPathDragging(true);
            }}
            onDragLeave={() => setPathDragging(false)}
            onDrop={dropPath}
          >
            Local Path
            <div className="path-input-row">
              <input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/Users/me/workspace/repo" />
              <button className="secondary icon-action" type="button" onClick={browsePath} disabled={busy} title="Browse">
                <FolderOpen size={16} />
              </button>
              <button className="secondary icon-action" type="button" onClick={() => revealPath(path)} disabled={busy || !path} title="Reveal">
                <ExternalLink size={16} />
              </button>
            </div>
          </label>
          <div className="repo-field-grid">
            <label className="repo-field">Base Branch<input value={baselineBranch} onChange={(event) => setBaselineBranch(event.target.value)} /></label>
            <SourcesField value={sources} onChange={setSources} />
          </div>
          <button className="primary repo-submit" disabled={busy}><FolderGit2 size={16} /> Register Workspace</button>
        </form>

        <div className="repo-list-panel">
          <header>
            <div>
              <h2>Registered</h2>
              <span>{workspaces.length} workspaces</span>
            </div>
            <button className="secondary" type="button" onClick={() => void scanAll()} disabled={busy || workspaces.length === 0}>
              <RotateCw size={16} /> Scan all
            </button>
          </header>
          {notice && <WorkspaceNoticePanel notice={notice} onDismiss={() => setNotice(null)} />}
          <div className="repo-list">
            {workspaces.map((repo) => (
              <article className="repo-row" key={repo.id}>
                <div className="repo-row-icon">
                  <HardDrive size={18} />
                  <span className="repo-baseline-badge" title={`Baseline branch: ${repo.baselineBranch}`}>{repo.baselineBranch}</span>
                </div>
                {editingId === repo.id ? (
                  <>
                    <div className="repo-row-main repo-edit-form">
                      <label className="repo-field">Workspace Name<input value={editDraft.name} onChange={(event) => setEditDraft({ ...editDraft, name: event.target.value })} /></label>
                      <label className="repo-field">Local Path<input value={editDraft.path} onChange={(event) => setEditDraft({ ...editDraft, path: event.target.value })} /></label>
                      <div className="repo-field-grid">
                        <label className="repo-field">Base Branch<input value={editDraft.baselineBranch} onChange={(event) => setEditDraft({ ...editDraft, baselineBranch: event.target.value })} /></label>
                        <SourcesField value={editDraft.sources} onChange={(value) => setEditDraft({ ...editDraft, sources: value })} />
                      </div>
                    </div>
                    <div className="repo-row-actions">
                      <button className="secondary" type="button" onClick={() => setEditingId('')} disabled={busy}>Cancel</button>
                      <button className="primary" type="button" onClick={() => saveEdit(repo)} disabled={busy}>Save</button>
                      <button className="secondary danger" type="button" onClick={() => setRepoToRemove(repo)} disabled={busy}><Trash2 size={16} /> Remove</button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="repo-row-main">
                      <h2>{repo.name}</h2>
                      <button className="repo-path-link" type="button" onClick={() => revealPath(repo.path)} disabled={busy} title={repo.path}>{repo.path}</button>
                      <div className="repo-directory-list">
                        {repo.sources.map((directory) => (
                          <div className="repo-directory-chip" key={directory}>
                            <span className="repo-directory-name">{directory}</span>
                            <button type="button" onClick={() => void openSourceSettings(repo, directory)} disabled={busy} aria-label={`Configure ${directory}`} title={`Configure ${directory}`}>
                              <SlidersHorizontal size={12} />
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="repo-row-actions">
                      <button className="secondary icon-action" onClick={() => revealPath(repo.path)} disabled={busy} title="Reveal">
                        <ExternalLink size={16} />
                      </button>
                      <button className="secondary icon-action" onClick={() => startEdit(repo)} disabled={busy} title="Edit">
                        <Pencil size={16} />
                      </button>
                      <button className="secondary" onClick={() => scan(repo)} disabled={busy}><RotateCw size={16} /> Scan</button>
                    </div>
                  </>
                )}
                <WorkspaceHealthPanel workspaceId={repo.id} />
              </article>
            ))}
            {workspaces.length === 0 && <div className="empty-inline repo-empty"><CheckCircle2 size={18} /> No workspaces registered.</div>}
          </div>
        </div>
      </div>
      {repoToRemove && (
        <ConfirmDialog
          title="Remove workspace"
          message={`Remove ${repoToRemove.name}? Cached items for this workspace will be removed from the board.`}
          confirmLabel={busy ? 'Removing...' : 'Remove'}
          busy={busy}
          danger
          onCancel={() => setRepoToRemove(null)}
          onConfirm={() => void removeRepo(repoToRemove)}
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
              Define how this source should be split into Kanban items.
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
