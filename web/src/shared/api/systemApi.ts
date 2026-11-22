import type { PathSelection, SystemConfigPaths, StorageOption, StorageStatus, StorageSyncDirection, StorageSyncResult } from '../../lib/types';
import { request } from './http';
import { normalizeSystemConfigPaths, normalizeStorageStatus, normalizeStorageSyncResult } from './normalize';

export const systemApi = {
  selectDirectory: () => request<PathSelection>('/api/system/select-directory', { method: 'POST' }),
	selectYAMLFile: () => request<PathSelection>('/api/system/select-file', { method: 'POST', body: JSON.stringify({ extensions: ['yaml', 'yml'] }) }),
  openPath: (path: string) => request<{ ok: boolean }>('/api/system/open-path', { method: 'POST', body: JSON.stringify({ path }) }),
  systemConfigPaths: (signal?: AbortSignal) => request<SystemConfigPaths>('/api/system/config-paths', { signal }, !signal).then(normalizeSystemConfigPaths),
  updateSystemConfigPaths: (input: { dataDir: string }) =>
    request<SystemConfigPaths>('/api/system/config-paths', { method: 'PUT', body: JSON.stringify(input) }).then(normalizeSystemConfigPaths),
  storageStatus: (signal?: AbortSignal) => request<StorageStatus>('/api/storage/status', { signal }, !signal).then(normalizeStorageStatus),
  saveStorageOption: (storageOption: StorageOption) =>
    request<{ storageOption: StorageOption; restartRequired: boolean }>('/api/storage/option', { method: 'PUT', body: JSON.stringify({ storageOption }) }),
  syncStorage: (direction: StorageSyncDirection) =>
    request<StorageSyncResult>('/api/storage/sync', { method: 'POST', body: JSON.stringify({ direction, confirm: true }) }).then(normalizeStorageSyncResult)
};
