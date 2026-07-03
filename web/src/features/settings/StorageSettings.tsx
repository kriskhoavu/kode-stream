import { ExternalLink, FolderOpen, Save } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../lib/api';
import type { SystemConfigPaths } from '../../lib/types';

export function StorageSettings() {
  const [config, setConfig] = useState<SystemConfigPaths | null>(null);
  const [dataDir, setDataDir] = useState('');
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    void api.systemConfigPaths().then((result) => {
      if (!active) return;
      setConfig(result);
      setDataDir(result.dataDir);
    }).catch((caught) => active && setError(errorMessage(caught)));
    return () => { active = false; };
  }, []);

  const browse = async () => {
    setPending(true); setError(''); setMessage('');
    try {
      const selection = await api.selectDirectory();
      setDataDir(selection.path);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  };

  const reveal = async (path: string) => {
    setPending(true); setError(''); setMessage('');
    try {
      await api.openPath(path);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  };

  const save = async () => {
    setPending(true); setError(''); setMessage('');
    try {
      const updated = await api.updateSystemConfigPaths({ dataDir: dataDir.trim() });
      setConfig(updated);
      setDataDir(updated.dataDir);
      setMessage('Storage settings saved. Restart Plan Manager to apply registry and index paths.');
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  };

  return <section className="settings-section storage-settings-section">
    <header>
      <div>
        <span className="settings-group-label">Application storage</span>
        <h2>Storage</h2>
        <p>Choose where Plan Manager stores its registry, indexes, and managed repository clones.</p>
      </div>
    </header>
    {config ? <div className="storage-settings-fields">
      <label>Data directory
        <div className="storage-settings-path-row">
          <input value={dataDir} onChange={(event) => { setDataDir(event.target.value); setMessage(''); }} />
          <button className="secondary" type="button" onClick={() => void browse()} disabled={pending}><FolderOpen size={15} /> Browse</button>
          <button className="secondary" type="button" onClick={() => void reveal(dataDir)} disabled={pending || !dataDir.trim()}><ExternalLink size={15} /> Reveal</button>
        </div>
      </label>
      <div className="storage-settings-derived">
        <span>Managed clone directory</span>
        <code>{config.cloneRootDir}</code>
        <button className="secondary" type="button" onClick={() => void reveal(config.cloneRootDir)} disabled={pending}><ExternalLink size={15} /> Reveal</button>
      </div>
      <p className="storage-settings-warning">Changing the data directory requires an application restart. Existing files are not moved automatically.</p>
    </div> : !error && <p className="settings-inline-status" role="status">Loading storage settings...</p>}
    {error && <p className="settings-error" role="alert">{error}</p>}
    <div className="settings-actions">
      {message && <span role="status">{message}</span>}
      <button className="primary" type="button" onClick={() => void save()} disabled={pending || !config || !dataDir.trim() || dataDir.trim() === config.dataDir}>
        <Save size={15} /> {pending ? 'Saving...' : 'Save storage settings'}
      </button>
    </div>
  </section>;
}

function errorMessage(caught: unknown) {
  return caught instanceof Error ? caught.message : 'Storage settings failed.';
}
