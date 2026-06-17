import { DragEvent, FormEvent, useState } from 'react';
import { CheckCircle2, ExternalLink, FolderGit2, FolderOpen, HardDrive, Pencil, Plus, RotateCw, Trash2, X } from 'lucide-react';
import { api } from '../lib/api';
import type { RepositoryConfig } from '../lib/types';

export function RepositoriesPage({ repositories, onChanged }: { repositories: RepositoryConfig[]; onChanged: () => void | Promise<void> }) {
  const [name, setName] = useState('Plan Manager');
  const [path, setPath] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('main');
  const [planDirectories, setPlanDirectories] = useState('plans');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [pathDragging, setPathDragging] = useState(false);
  const [editingId, setEditingId] = useState('');
  const [editDraft, setEditDraft] = useState({ name: '', path: '', baselineBranch: '', planDirectories: '' });

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      await api.createRepository({
        name,
        path,
        baselineBranch,
        planDirectories: parsePlanDirectories(planDirectories)
      });
      setMessage('Repository registered');
      setName('');
      setPath('');
      setBaselineBranch('main');
      setPlanDirectories('plans');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setBusy(false);
    }
  };

  const scan = async (repo: RepositoryConfig) => {
    setBusy(true);
    setMessage(`Scanning ${repo.name}`);
    try {
      const result = await api.scan(repo.id);
      setMessage(`${result.planCount} plans indexed`);
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Scan failed');
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (repo: RepositoryConfig) => {
    setEditingId(repo.id);
    setEditDraft({
      name: repo.name,
      path: repo.path,
      baselineBranch: repo.baselineBranch,
      planDirectories: repo.planDirectories.join(', ')
    });
    setMessage('');
  };

  const saveEdit = async (repo: RepositoryConfig) => {
    setBusy(true);
    setMessage('');
    try {
      await api.updateRepository(repo.id, {
        name: editDraft.name,
        path: editDraft.path,
        baselineBranch: editDraft.baselineBranch,
        planDirectories: parsePlanDirectories(editDraft.planDirectories)
      });
      setEditingId('');
      setMessage('Repository updated');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Update failed');
    } finally {
      setBusy(false);
    }
  };

  const removeRepo = async (repo: RepositoryConfig) => {
    if (!window.confirm(`Remove ${repo.name}? Cached plans for this repository will be removed from the board.`)) return;
    setBusy(true);
    setMessage('');
    try {
      await api.deleteRepository(repo.id);
      setEditingId('');
      setMessage('Repository removed');
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Remove failed');
    } finally {
      setBusy(false);
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
    <section className="repositories-page">
      <div className="page-title">
        <div>
          <h1>Repositories</h1>
          <span>Register local Git repositories and scan plan folders.</span>
        </div>
      </div>

      <div className="repositories-layout">
        <form className="repo-form repo-create-panel" onSubmit={submit}>
          <header>
            <FolderGit2 size={18} />
            <h2>Register Repository</h2>
          </header>
          <label className="repo-field">Repository Name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="Discovery" /></label>
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
            <label className="repo-field">Baseline Branch<input value={baselineBranch} onChange={(event) => setBaselineBranch(event.target.value)} /></label>
            <PlanDirectoriesField value={planDirectories} onChange={setPlanDirectories} />
          </div>
          <button className="primary repo-submit" disabled={busy}><FolderGit2 size={16} /> Register Repository</button>
          {message && <p className={message.includes('failed') || message.includes('invalid') || message.includes('cancelled') ? 'error' : 'success'}>{message}</p>}
        </form>

        <div className="repo-list-panel">
          <header>
            <div>
              <h2>Registered</h2>
              <span>{repositories.length} repositories</span>
            </div>
          </header>
          <div className="repo-list">
            {repositories.map((repo) => (
              <article className="repo-row" key={repo.id}>
                <div className="repo-row-icon"><HardDrive size={18} /></div>
                {editingId === repo.id ? (
                  <>
                    <div className="repo-row-main repo-edit-form">
                      <label className="repo-field">Repository Name<input value={editDraft.name} onChange={(event) => setEditDraft({ ...editDraft, name: event.target.value })} /></label>
                      <label className="repo-field">Local Path<input value={editDraft.path} onChange={(event) => setEditDraft({ ...editDraft, path: event.target.value })} /></label>
                      <div className="repo-field-grid">
                        <label className="repo-field">Baseline Branch<input value={editDraft.baselineBranch} onChange={(event) => setEditDraft({ ...editDraft, baselineBranch: event.target.value })} /></label>
                        <PlanDirectoriesField value={editDraft.planDirectories} onChange={(value) => setEditDraft({ ...editDraft, planDirectories: value })} />
                      </div>
                    </div>
                    <div className="repo-row-actions">
                      <button className="secondary" type="button" onClick={() => setEditingId('')} disabled={busy}>Cancel</button>
                      <button className="primary" type="button" onClick={() => saveEdit(repo)} disabled={busy}>Save</button>
                      <button className="secondary danger" type="button" onClick={() => removeRepo(repo)} disabled={busy}><Trash2 size={16} /> Remove</button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="repo-row-main">
                      <h2>{repo.name}</h2>
                      <button className="repo-path-link" type="button" onClick={() => revealPath(repo.path)} disabled={busy} title={repo.path}>{repo.path}</button>
                      <span>{repo.baselineBranch}</span>
                      <div className="repo-directory-list">{repo.planDirectories.map((directory) => <span key={directory}>{directory}</span>)}</div>
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
              </article>
            ))}
            {repositories.length === 0 && <div className="empty-inline repo-empty"><CheckCircle2 size={18} /> No repositories registered.</div>}
          </div>
        </div>
      </div>
    </section>
  );
}

function PlanDirectoriesField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const directories = parsePlanDirectories(value);
  return (
    <label className="repo-field">Plan Directories
      <div className="directory-input">
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="plans, docs" />
        <div className="directory-chips">
          {directories.map((directory) => (
            <button type="button" key={directory} onClick={() => onChange(removePlanDirectory(value, directory))}>
              {directory}
              <X size={13} />
            </button>
          ))}
          {['plans', 'docs'].filter((directory) => !directories.includes(directory)).map((directory) => (
            <button type="button" className="add-directory-chip" key={directory} onClick={() => onChange(addPlanDirectory(value, directory))}>
              <Plus size={13} />
              {directory}
            </button>
          ))}
        </div>
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

export function normalizeDroppedPath(value?: string): string {
  if (!value) return '';
  const trimmed = value.trim().replace(/^["']|["']$/g, '');
  if (!trimmed.startsWith('file://')) return trimmed;
  try {
    return decodeURIComponent(new URL(trimmed).pathname);
  } catch {
    return trimmed;
  }
}

function lastPathSegment(value: string): string {
  return value.split(/[\\/]/).filter(Boolean).at(-1) ?? '';
}

export function parsePlanDirectories(value: string): string[] {
  return Array.from(new Set(value.split(',').map((item) => item.trim()).filter(Boolean)));
}

function addPlanDirectory(value: string, directory: string): string {
  return [...parsePlanDirectories(value), directory].join(', ');
}

function removePlanDirectory(value: string, directory: string): string {
  return parsePlanDirectories(value).filter((item) => item !== directory).join(', ');
}
