import { ArrowLeftRight, Database, ExternalLink, FileText, FolderOpen, Save } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { api } from '../../lib/api';
import type { StorageOption, StorageStatus, StorageSyncDirection, StorageSyncResult, SystemConfigPaths } from '../../lib/types';

export function StorageSettings() {
  const [config, setConfig] = useState<SystemConfigPaths | null>(null);
  const [status, setStatus] = useState<StorageStatus | null>(null);
  const [dataDir, setDataDir] = useState('');
  const [selectedOption, setSelectedOption] = useState<StorageOption>('database');
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [syncResult, setSyncResult] = useState<StorageSyncResult | null>(null);

  useEffect(() => {
    let active = true;
    void Promise.all([api.systemConfigPaths(), api.storageStatus()]).then(([paths, storage]) => {
      if (!active) return;
      setConfig(paths);
      setStatus(storage);
      setDataDir(paths.dataDir);
      setSelectedOption(storage.storageOption);
    }).catch((caught) => active && setError(errorMessage(caught)));
    return () => { active = false; };
  }, []);

  const optionChanged = Boolean(status && selectedOption !== status.storageOption);
  const dataDirChanged = Boolean(config && dataDir.trim() !== config.dataDir);
  const syncDisabled = pending || !status || status.mode === 'cloud';
  const databaseLabel = useMemo(() => status?.storageDriver === 'postgres' ? 'Postgres' : 'SQLite', [status?.storageDriver]);

  const browse = async () => {
    setPending(true); setError(''); setMessage(''); setSyncResult(null);
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
    setPending(true); setError(''); setMessage(''); setSyncResult(null);
    try {
      await api.openPath(path);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  };

  const save = async () => {
    setPending(true); setError(''); setMessage(''); setSyncResult(null);
    try {
      let savedConfig = config;
      if (dataDirChanged) {
        savedConfig = await api.updateSystemConfigPaths({ dataDir: dataDir.trim() });
        setConfig(savedConfig);
        setDataDir(savedConfig.dataDir);
      }
      if (optionChanged) {
        await api.saveStorageOption(selectedOption);
      }
      setMessage('Storage settings saved. Restart Kode Stream to apply the selected storage option and paths.');
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  };

  const sync = async (direction: StorageSyncDirection) => {
    const label = direction === 'datadir_to_database' ? 'data-dir into database' : 'database into data-dir';
    if (!window.confirm(`Replace the target ${label} storage after creating a backup?`)) return;
    setPending(true); setError(''); setMessage(''); setSyncResult(null);
    try {
      const result = await api.syncStorage(direction);
      setSyncResult(result);
      setMessage('Storage sync completed.');
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
        <p>Choose the local app-state backend, inspect paths, and manually copy state between supported stores.</p>
      </div>
      {status && <span className="settings-count">{status.mode === 'cloud' ? 'Cloud' : 'Local'} {status.storageOption}</span>}
    </header>
    {config && status ? <div className="storage-settings-fields">
      <div className="storage-option-group" role="radiogroup" aria-label="Storage option">
        <label className={selectedOption === 'database' ? 'selected' : ''}>
          <input type="radio" name="storageOption" value="database" checked={selectedOption === 'database'} disabled={pending || status.environmentLocked || status.mode === 'cloud'} onChange={() => setSelectedOption('database')} />
          <Database size={16} />
          <span><strong>Database</strong><small>{status.mode === 'cloud' ? 'Postgres' : 'SQLite'}</small></span>
        </label>
        <label className={selectedOption === 'datadir' ? 'selected' : ''}>
          <input type="radio" name="storageOption" value="datadir" checked={selectedOption === 'datadir'} disabled={pending || status.environmentLocked || status.mode === 'cloud'} onChange={() => setSelectedOption('datadir')} />
          <FileText size={16} />
          <span><strong>Data-dir</strong><small>YAML and JSONL</small></span>
        </label>
      </div>
      {status.environmentLocked && <p className="storage-settings-warning">Storage option is controlled by {status.storageOptionEnv} or {status.storageDriverEnv}.</p>}
      {optionChanged && <p className="storage-settings-warning">Changing storage option requires an application restart.</p>}
      <label>Data directory
        <div className="storage-settings-path-row">
          <input value={dataDir} onChange={(event) => { setDataDir(event.target.value); setMessage(''); }} />
          <button className="secondary" type="button" onClick={() => void browse()} disabled={pending}><FolderOpen size={15} /> Browse</button>
          <button className="secondary" type="button" onClick={() => void reveal(dataDir)} disabled={pending || !dataDir.trim()}><ExternalLink size={15} /> Reveal</button>
        </div>
      </label>
      <div className="storage-settings-derived">
        <span>{databaseLabel} store</span>
        <code>{status.databasePath || (status.databaseUrlConfigured ? 'Configured with database URL' : 'Not configured')}</code>
        {status.databasePath && <button className="secondary" type="button" onClick={() => void reveal(status.databasePath ?? '')} disabled={pending}><ExternalLink size={15} /> Reveal</button>}
      </div>
      <div className="storage-settings-derived">
        <span>Managed clone directory</span>
        <code>{config.cloneRootDir}</code>
        <button className="secondary" type="button" onClick={() => void reveal(config.cloneRootDir)} disabled={pending}><ExternalLink size={15} /> Reveal</button>
      </div>
      {status.database && <p className={status.database.ok ? 'settings-inline-status storage-ok' : 'settings-error'}>{databaseLabel} migration version {status.database.migrationVersion}{status.database.error ? `: ${status.database.error}` : ''}</p>}
      <div className="storage-sync-panel">
        <div>
          <strong>Manual sync</strong>
          <p>Each sync creates a target backup before replacement. Runtime writes still go only to the active option.</p>
        </div>
        <div className="storage-sync-actions">
          <button className="secondary" type="button" onClick={() => void sync('datadir_to_database')} disabled={syncDisabled}><ArrowLeftRight size={15} /> Data-dir to database</button>
          <button className="secondary" type="button" onClick={() => void sync('database_to_datadir')} disabled={syncDisabled}><ArrowLeftRight size={15} /> Database to data-dir</button>
        </div>
        {status.mode === 'cloud' && <p className="storage-settings-warning">Manual storage sync is only available in local mode.</p>}
        {syncResult && <SyncSummary result={syncResult} />}
      </div>
    </div> : !error && <p className="settings-inline-status" role="status">Loading storage settings...</p>}
    {error && <p className="settings-error" role="alert">{error}</p>}
    <div className="settings-actions">
      {message && <span role="status">{message}</span>}
      <button className="primary" type="button" onClick={() => void save()} disabled={pending || !config || !dataDir.trim() || (!dataDirChanged && !optionChanged)}>
        <Save size={15} /> {pending ? 'Saving...' : 'Save storage settings'}
      </button>
    </div>
  </section>;
}

function SyncSummary({ result }: { result: StorageSyncResult }) {
  return <div className="storage-sync-summary">
    <span>Backup</span>
    <code>{result.backupPath}</code>
    <span>Copied</span>
    <p>{Object.entries(result.summary).map(([key, value]) => `${value} ${key}`).join(', ')}</p>
    {result.skippedStores.length > 0 && <><span>Skipped</span><p>{result.skippedStores.join(', ')}</p></>}
    {result.warnings.length > 0 && <><span>Warnings</span><p>{result.warnings.join(', ')}</p></>}
  </div>;
}

function errorMessage(caught: unknown) {
  return caught instanceof Error ? caught.message : 'Storage settings failed.';
}
