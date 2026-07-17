import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { StorageSettings } from './StorageSettings';

vi.mock('../../lib/api', () => ({ api: {
  systemConfigPaths: vi.fn(),
  storageStatus: vi.fn(),
  saveStorageOption: vi.fn(),
  syncStorage: vi.fn(),
  selectDirectory: vi.fn(),
  openPath: vi.fn(),
  updateSystemConfigPaths: vi.fn()
} }));

describe('StorageSettings', () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('saves a selected data directory and explains restart behavior', async () => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/old', defaultDataDir: '/default', cloneRootDir: '/old/clones' });
    vi.mocked(api.storageStatus).mockResolvedValue(storageStatus());
    vi.mocked(api.selectDirectory).mockResolvedValue({ path: '/new' });
    vi.mocked(api.updateSystemConfigPaths).mockResolvedValue({ dataDir: '/new', defaultDataDir: '/default', cloneRootDir: '/new/clones', restartRequired: true });
    render(<StorageSettings />);

    fireEvent.click(await screen.findByRole('button', { name: 'Browse' }));
    await waitFor(() => expect(screen.getByDisplayValue('/new')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Save storage settings' }));

    await waitFor(() => expect(api.updateSystemConfigPaths).toHaveBeenCalledWith({ dataDir: '/new' }));
    expect(await screen.findByText(/Restart Kode Stream/)).toBeInTheDocument();
  });

  it('saves a local storage option for restart', async () => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones' });
    vi.mocked(api.storageStatus).mockResolvedValue(storageStatus({ storageOption: 'database', environmentLocked: false }));
    vi.mocked(api.saveStorageOption).mockResolvedValue({ storageOption: 'datadir', restartRequired: true });
    render(<StorageSettings />);

    fireEvent.click(await screen.findByRole('radio', { name: /data-dir/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Save storage settings' }));

    await waitFor(() => expect(api.saveStorageOption).toHaveBeenCalledWith('datadir'));
    expect(await screen.findByText(/Restart Kode Stream/)).toBeInTheDocument();
  });

  it('runs confirmed manual sync and shows backup summary', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones' });
    vi.mocked(api.storageStatus).mockResolvedValue(storageStatus());
    vi.mocked(api.syncStorage).mockResolvedValue({
      ok: true,
      direction: 'datadir_to_database',
      backupPath: '/data/backups/storage-sync/sync',
      summary: { workspaces: 1, items: 2 },
      warnings: [],
      skippedStores: ['knowledge']
    });
    render(<StorageSettings />);

    fireEvent.click(await screen.findByRole('button', { name: 'Data-dir to database' }));

    await waitFor(() => expect(api.syncStorage).toHaveBeenCalledWith('datadir_to_database'));
    expect(await screen.findByText('/data/backups/storage-sync/sync')).toBeInTheDocument();
    expect(screen.getByText(/1 workspaces, 2 items/)).toBeInTheDocument();
  });
});

function storageStatus(overrides: Partial<Awaited<ReturnType<typeof api.storageStatus>>> = {}) {
  return {
    mode: 'local',
    storageOption: 'database',
    storageDriver: 'sqlite',
    environmentLocked: false,
    storageOptionEnv: 'KODE_STREAM_STORAGE_OPTION',
    storageDriverEnv: 'KODE_STREAM_STORAGE_DRIVER',
    dataDir: '/data',
    databasePath: '/data/kode-stream.db',
    databaseUrlConfigured: false,
    database: { driver: 'sqlite', ok: true, migrationVersion: 1 },
    ...overrides
  } as Awaited<ReturnType<typeof api.storageStatus>>;
}
