import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { StorageSettings } from './StorageSettings';

vi.mock('../../lib/api', () => ({ api: {
  systemConfigPaths: vi.fn(), selectDirectory: vi.fn(), openPath: vi.fn(), updateSystemConfigPaths: vi.fn()
} }));

describe('StorageSettings', () => {
  afterEach(() => vi.clearAllMocks());

  it('saves a selected data directory and explains restart behavior', async () => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/old', defaultDataDir: '/default', cloneRootDir: '/old/clones' });
    vi.mocked(api.selectDirectory).mockResolvedValue({ path: '/new' });
    vi.mocked(api.updateSystemConfigPaths).mockResolvedValue({ dataDir: '/new', defaultDataDir: '/default', cloneRootDir: '/new/clones', restartRequired: true });
    render(<StorageSettings />);

    fireEvent.click(await screen.findByRole('button', { name: 'Browse' }));
    await waitFor(() => expect(screen.getByDisplayValue('/new')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Save storage settings' }));

    await waitFor(() => expect(api.updateSystemConfigPaths).toHaveBeenCalledWith({ dataDir: '/new' }));
    expect(await screen.findByText(/Restart Kode Stream/)).toBeInTheDocument();
  });
});
