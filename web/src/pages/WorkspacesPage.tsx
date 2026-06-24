import { type DragEvent, type Dispatch, type FormEvent, type SetStateAction, useState } from 'react';
import { CheckCircle2, ExternalLink, FolderGit2, FolderOpen, HardDrive, Pencil, Plus, RotateCw, SlidersHorizontal, Trash2, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { WorkspaceHealthPanel } from '../components/ReliabilityPanels';
import { api } from '../lib/api';
import type { WorkspaceConfig, SourceStructureSettings, SourceStructureCard, SourceStructurePreview, SourceStructureProposal, SourceSettingsResult } from '../lib/types';
import { labels } from '../lib/vocabulary';
import { applySegmentRole, inferCompatibilityFields, lastPathSegment, normalizeDroppedPath, parseSources, previewPathSegments } from '../features/workspaces/sourceSettings';
import type { SourceStructureSegmentRole } from '../features/workspaces/sourceSettings';
import { notifyReliabilityChanged } from '../features/reliability/hooks';

export { applySegmentRole, inferCompatibilityFields, normalizeDroppedPath, parseSources, previewPathSegments };

const DEFAULT_SOURCES = ['docs', 'plans'];
type SettingsEditorState = {
  repo: WorkspaceConfig;
  directory: string;
  exists: boolean;
  mode?: string;
  card: SourceStructureCard;
  warnings: string[];
  proposals: SourceStructureProposal[];
  preview: SourceStructurePreview[];
};

export function WorkspacesPage({ workspaces, onChanged }: { workspaces: WorkspaceConfig[]; onChanged: () => void | Promise<void> }) {
  const [name, setName] = useState('Plan Manager');
  const [path, setPath] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('main');
  const [sources, setSources] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [pathDragging, setPathDragging] = useState(false);
  const [editingId, setEditingId] = useState('');
  const [editDraft, setEditDraft] = useState({ name: '', path: '', baselineBranch: '', sources: '' });
  const [repoToRemove, setRepoToRemove] = useState<WorkspaceConfig | null>(null);
  const [settingsEditor, setSettingsEditor] = useState<SettingsEditorState | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      await api.createWorkspace({
        name,
        path,
        baselineBranch,
        sources: parseSources(sources)
      });
      setMessage('Workspace registered');
      setName('');
      setPath('');
      setBaselineBranch('main');
      setSources('');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setBusy(false);
    }
  };

  const scan = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setMessage(`Scanning ${repo.name}`);
    try {
      const result = await api.scan(repo.id);
      notifyReliabilityChanged();
      setMessage(`${result.itemCount} items indexed`);
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Scan failed');
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
    setMessage('');
  };

  const saveEdit = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setMessage('');
    try {
      await api.updateWorkspace(repo.id, {
        name: editDraft.name,
        path: editDraft.path,
        baselineBranch: editDraft.baselineBranch,
        sources: parseSources(editDraft.sources)
      });
      setEditingId('');
      setMessage('Workspace updated');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Update failed');
    } finally {
      setBusy(false);
    }
  };

  const removeRepo = async (repo: WorkspaceConfig) => {
    setBusy(true);
    setMessage('');
    try {
      await api.deleteWorkspace(repo.id);
      setEditingId('');
      setMessage('Workspace removed');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Remove failed');
    } finally {
      setBusy(false);
      setRepoToRemove(null);
    }
  };

  const browsePath = async () => {
    setBusy(true);
    setMessage('');
    try {
      const selection = await api.selectDirectory();
      setPath(selection.path);
      if (!name || name === 'Plan Manager') {
        setName(lastPathSegment(selection.path));
      }
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Directory selection failed');
    } finally {
      setBusy(false);
    }
  };

  const revealPath = async (targetPath: string) => {
    setBusy(true);
    setMessage('');
    try {
      await api.openPath(targetPath);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Path failed to open');
    } finally {
      setBusy(false);
    }
  };

  const openSourceSettings = async (repo: WorkspaceConfig, directory: string) => {
    setBusy(true);
    setMessage('');
    try {
      const result = await api.sourceStructure(repo.id, directory);
      setSettingsEditor(settingsEditorFromResult(repo, directory, result));
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Settings failed to load');
    } finally {
      setBusy(false);
    }
  };

  const saveSourceSettings = async () => {
    if (!settingsEditor) return;
    setBusy(true);
    setMessage('');
    try {
      const settings: SourceStructureSettings = {
        version: 1,
        cards: [withInferredCompatibilityFields(settingsEditor.card, settingsEditor.directory)]
      };
      const result = await api.saveSourceStructure(settingsEditor.repo.id, settingsEditor.directory, settings);
      notifyReliabilityChanged();
      setSettingsEditor(null);
      setMessage(`Source structure saved; ${result.scan?.itemCount ?? 0} items indexed`);
      await onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Settings failed to save');
    } finally {
      setBusy(false);
    }
  };

  const resetSourceSettings = async () => {
    if (!settingsEditor) return;
    const confirmed = window.confirm(`Reset Source Structure for ${settingsEditor.directory}? This removes workspace-settings.yaml and scans the source again.`);
    if (!confirmed) return;
    setBusy(true);
    setMessage('');
    try {
      const { repo, directory } = settingsEditor;
      const result = await api.resetSourceStructure(repo.id, directory);
      notifyReliabilityChanged();
      setSettingsEditor(settingsEditorFromResult(repo, directory, result));
      setMessage(`Source structure reset; ${result.scan?.itemCount ?? 0} items indexed`);
      await onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Settings reset failed');
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
    setMessage('Drop a folder path or file URL');
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
          {message && <p className={message.includes('failed') || message.includes('invalid') || message.includes('cancelled') ? 'error' : 'success'}>{message}</p>}
        </form>

        <div className="repo-list-panel">
          <header>
            <div>
              <h2>Registered</h2>
              <span>{workspaces.length} workspaces</span>
            </div>
          </header>
          <div className="repo-list">
            {workspaces.map((repo) => (
              <article className="repo-row" key={repo.id}>
                <div className="repo-row-icon"><HardDrive size={18} /></div>
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
                      <span>{repo.baselineBranch}</span>
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
              <button className="icon-button" type="button" onClick={() => setSettingsEditor(null)} disabled={busy} aria-label="Close source structure">
                <X size={16} />
              </button>
            </header>
            <p className="modal-help">
              Define how this source should be split into item cards. Scope and identifier are inferred from the path pattern.
            </p>
            {!settingsEditor.exists && settingsEditor.mode === 'structured' && (
              <div className="metadata-callout source-structure-supported">
                <strong>Built-in structure detected</strong>
                <span>This source already follows the supported scope/identifier layout. Saving here creates an optional override.</span>
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
              selectedCard={settingsEditor.card}
              onSelect={(proposal) => applySettingsProposal(setSettingsEditor, proposal)}
            />
            <SourceStructurePathBuilder
              directory={settingsEditor.directory}
              card={settingsEditor.card}
              preview={settingsEditor.preview}
              onRole={(index, role) => applySettingsSegmentRole(setSettingsEditor, index, role)}
            />
            <SourceStructurePreviewTable preview={settingsEditor.preview} />
            <div className="metadata-form source-structure-form">
              <label>
                Path Pattern
                <input
                  value={settingsEditor.card.pathPattern}
                  onChange={(event) => updateSettingsCard(setSettingsEditor, { pathPattern: event.target.value })}
                  placeholder="{scope}/feature/{identifier}"
                />
              </label>
              <div className="repo-field-grid">
                <label>
                  Title
                  <input value={settingsEditor.card.fields.title ?? ''} onChange={(event) => updateSettingsField(setSettingsEditor, 'title', event.target.value)} placeholder="readme_heading" />
                </label>
                <label>
                  Default Status
                  <select value={settingsEditor.card.fields.status ?? 'draft'} onChange={(event) => updateSettingsField(setSettingsEditor, 'status', event.target.value)}>
                    <option value="ideas">Ideas</option>
                    <option value="draft">Draft</option>
                    <option value="in_progress">In Progress</option>
                    <option value="review">Review</option>
                    <option value="done">Done</option>
                  </select>
                </label>
              </div>
              <div className="repo-field-grid">
                <label>
                  Owner
                  <input value={settingsEditor.card.fields.owner ?? ''} onChange={(event) => updateSettingsField(setSettingsEditor, 'owner', event.target.value)} placeholder="{owner} or a name" />
                </label>
                <label>
                  Tags
                  <input value={(settingsEditor.card.fields.tags ?? []).join(', ')} onChange={(event) => updateSettingsField(setSettingsEditor, 'tags', event.target.value)} placeholder="docs, discovery" />
                </label>
              </div>
            </div>
            <footer className="modal-actions">
              {settingsEditor.exists && (
                <button className="secondary danger" type="button" onClick={() => void resetSourceSettings()} disabled={busy}>
                  Reset config
                </button>
              )}
              <button className="secondary" type="button" onClick={() => setSettingsEditor(null)} disabled={busy}>Cancel</button>
              <button className="primary" type="button" onClick={() => void saveSourceSettings()} disabled={busy}>
                <SlidersHorizontal size={15} />
                {busy ? 'Saving...' : 'Save and Scan'}
              </button>
            </footer>
          </div>
        </section>
      )}
    </section>
  );
}

function SourceStructureProposalList({ proposals, selectedCard, onSelect }: { proposals: SourceStructureProposal[]; selectedCard: SourceStructureCard; onSelect: (proposal: SourceStructureProposal) => void }) {
  if (proposals.length === 0) return null;
  return (
    <section className="source-proposals" aria-label="Source structure proposals">
      <div className="source-structure-section-heading">
        <strong>Suggested structures</strong>
        <span>Choose the closest match, then adjust if needed.</span>
      </div>
      <div className="source-proposal-grid">
        {proposals.map((proposal) => {
          const selected = cardsMatch(selectedCard, proposal.card);
          return (
            <button className={selected ? 'source-proposal-card active' : 'source-proposal-card'} type="button" key={proposal.id} onClick={() => onSelect(proposal)}>
              <span className="proposal-confidence">{proposal.confidence}</span>
              <strong>{proposal.label}</strong>
              <span>{proposal.summary}</span>
              <small>{proposal.card.pathPattern}</small>
            </button>
          );
        })}
      </div>
    </section>
  );
}

function SourceStructurePathBuilder({ directory, card, preview, onRole }: { directory: string; card: SourceStructureCard; preview: SourceStructurePreview[]; onRole: (index: number, role: SourceStructureSegmentRole) => void }) {
  const samplePath = preview[0]?.path ?? [directory, ...card.pathPattern.split('/')].filter(Boolean).join('/');
  const segments = previewPathSegments(samplePath, directory);
  if (segments.length === 0) return null;
  return (
    <section className="source-path-builder" aria-label="Path segment builder">
      <div className="source-structure-section-heading">
        <strong>Click path segments</strong>
        <span>Mark which parts become card fields.</span>
      </div>
      <div className="source-segment-list">
        {segments.map((segment, index) => (
          <div className="source-segment-card" key={`${segment}-${index}`}>
            <span className="source-segment-pill">{segment}</span>
            <div className="segment-role-actions" role="group" aria-label={`Role for ${segment}`}>
              <button type="button" onClick={() => onRole(index, 'scope')}>Scope</button>
              <button type="button" onClick={() => onRole(index, 'identifier')}>Identifier</button>
              <button type="button" onClick={() => onRole(index, 'literal')}>Fixed</button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function SourceStructurePreviewTable({ preview }: { preview: SourceStructurePreview[] }) {
  return (
    <section className="source-preview" aria-label="Source structure preview">
      <div className="source-structure-section-heading">
        <strong>Preview</strong>
        <span>{preview.length === 0 ? 'No matching card directories yet.' : `${preview.length} sample cards`}</span>
      </div>
      {preview.length > 0 && (
        <div className="source-preview-table">
          <div className="source-preview-row heading">
            <span>Path</span>
            <span>Scope</span>
            <span>Identifier</span>
            <span>Title</span>
            <span>Status</span>
          </div>
          {preview.map((row) => (
            <div className="source-preview-row" key={row.path}>
              <span title={row.path}>{row.path}</span>
              <span>{row.scope}</span>
              <span>{row.identifier}</span>
              <span>{row.title}</span>
              <span>{row.status}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
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

function settingsEditorFromResult(repo: WorkspaceConfig, directory: string, result: SourceSettingsResult): SettingsEditorState {
  return {
    repo,
    directory,
    exists: result.exists,
    mode: result.mode,
    card: normalizeSettingsCard(result.settings?.cards?.[0], directory),
    warnings: (result.warnings ?? []).map((warning) => warning.message),
    proposals: result.proposals ?? [],
    preview: result.preview ?? []
  };
}

function normalizeSettingsCard(card?: SourceStructureCard, directory = 'source'): SourceStructureCard {
  const legacyFields = card?.fields as SourceStructureCard['fields'] & { service?: string; ticket?: string } | undefined;
  return withInferredCompatibilityFields({
    pathPattern: genericTemplate(card?.pathPattern || '{scope}/feature/{identifier}'),
    fields: {
      scope: genericTemplate(legacyFields?.scope || legacyFields?.service || '{scope}'),
      identifier: genericTemplate(legacyFields?.identifier || legacyFields?.ticket || '{identifier}'),
      title: card?.fields?.title || 'readme_heading',
      status: card?.fields?.status || 'draft',
      owner: card?.fields?.owner || '',
      tags: Array.isArray(card?.fields?.tags) ? card.fields.tags : ['docs']
    }
  }, directory);
}

function genericTemplate(value: string): string {
  return value.replaceAll('{service}', '{scope}').replaceAll('{ticket}', '{identifier}');
}

function withInferredCompatibilityFields(card: SourceStructureCard, directory: string): SourceStructureCard {
  return {
    ...card,
    fields: {
      ...card.fields,
      ...inferCompatibilityFields(card.pathPattern, directory)
    }
  };
}

function updateSettingsCard(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>,
  patch: Partial<SourceStructureCard>
) {
  setSettingsEditor((current) => {
    if (!current) return current;
    return {
      ...current,
      card: withInferredCompatibilityFields({ ...current.card, ...patch }, current.directory)
    };
  });
}

function updateSettingsField(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>,
  field: keyof SourceStructureCard['fields'],
  value: string
) {
  setSettingsEditor((current) => {
    if (!current) return current;
    const nextValue = field === 'tags' ? value.split(',').map((item) => item.trim()).filter(Boolean) : value;
    return {
      ...current,
      card: {
        ...current.card,
        fields: {
          ...current.card.fields,
          [field]: nextValue
        }
      }
    };
  });
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
      preview: proposal.preview
    };
  });
}

function applySettingsSegmentRole(
  setSettingsEditor: Dispatch<SetStateAction<SettingsEditorState | null>>,
  index: number,
  role: SourceStructureSegmentRole
) {
  setSettingsEditor((current) => {
    if (!current) return current;
    const samplePath = current.preview[0]?.path ?? [current.directory, ...current.card.pathPattern.split('/')].filter(Boolean).join('/');
    const sampleSegments = previewPathSegments(samplePath, current.directory);
    const pathPattern = applySegmentRole(current.card.pathPattern, sampleSegments, index, role);
    const nextCard = withInferredCompatibilityFields({ ...current.card, pathPattern }, current.directory);
    const matchingProposal = current.proposals.find((proposal) => cardsMatch(nextCard, normalizeSettingsCard(proposal.card, current.directory)));
    return {
      ...current,
      card: nextCard,
      preview: matchingProposal?.preview ?? current.preview
    };
  });
}

function cardsMatch(left: SourceStructureCard, right: SourceStructureCard): boolean {
  return left.pathPattern === right.pathPattern
    && left.fields.scope === right.fields.scope
    && left.fields.identifier === right.fields.identifier;
}
