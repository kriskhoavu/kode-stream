import { FormEvent, useState } from 'react';
import { CheckCircle2, FolderGit2, RotateCw } from 'lucide-react';
import { api } from '../lib/api';
import type { RepositoryConfig } from '../lib/types';

export function RepositoriesPage({ repositories, onChanged }: { repositories: RepositoryConfig[]; onChanged: () => void }) {
  const [name, setName] = useState('Plan Manager');
  const [path, setPath] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('main');
  const [planDirectories, setPlanDirectories] = useState('plans');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      await api.createRepository({
        name,
        path,
        baselineBranch,
        planDirectories: planDirectories.split(',').map((item) => item.trim()).filter(Boolean)
      });
      setMessage('Repository registered');
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

  return (
    <section className="repositories-page">
      <div className="page-title">
        <h1>Repositories</h1>
      </div>
      <form className="repo-form" onSubmit={submit}>
        <label>Repository Name<input value={name} onChange={(event) => setName(event.target.value)} /></label>
        <label>Local Path<input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/Users/me/workspace/repo" /></label>
        <label>Baseline Branch<input value={baselineBranch} onChange={(event) => setBaselineBranch(event.target.value)} /></label>
        <label>Plan Directories<input value={planDirectories} onChange={(event) => setPlanDirectories(event.target.value)} placeholder="plans, docs/plans" /></label>
        <button className="primary" disabled={busy}><FolderGit2 size={16} /> Register Repository</button>
        {message && <p className={message.includes('failed') || message.includes('invalid') ? 'error' : 'success'}>{message}</p>}
      </form>
      <div className="repo-list">
        {repositories.map((repo) => (
          <article className="repo-row" key={repo.id}>
            <div>
              <h2>{repo.name}</h2>
              <p>{repo.path}</p>
              <span>{repo.baselineBranch} · {repo.planDirectories.join(', ')}</span>
            </div>
            <button className="secondary" onClick={() => scan(repo)} disabled={busy}><RotateCw size={16} /> Scan</button>
          </article>
        ))}
        {repositories.length === 0 && <div className="empty-inline"><CheckCircle2 size={18} /> No repositories registered.</div>}
      </div>
    </section>
  );
}
