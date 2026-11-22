import type { FileContent, FileNode, FileSaveInput, NewItemInput, ItemDetail, ItemMetadataUpdateInput, ItemStatusUpdateInput, ItemSummary, WorkspaceContentSearchResponse, ScanResult, WriteResult } from '../../lib/types';
import { request } from './http';
import { contentSearchQuery, normalizeContentSearchResponse, normalizeItem, normalizeItemDetail } from './normalize';

export const itemApi = {
  items: async (params: URLSearchParams, signal?: AbortSignal) => ((await request<ItemSummary[] | null>(`/api/items?${params.toString()}`, { signal }, !signal)) ?? []).map(normalizeItem),
  item: async (id: string, signal?: AbortSignal) => normalizeItemDetail(await request<ItemDetail>(`/api/items/${id}`, { signal }, !signal)),
  files: async (id: string, expectedCommit?: string, signal?: AbortSignal) => {
		const query = expectedCommit ? `?expectedCommit=${encodeURIComponent(expectedCommit)}` : '';
		return (await request<FileNode[] | null>(`/api/items/${encodeURIComponent(id)}/files${query}`, { signal }, !signal)) ?? [];
	},
	searchItemContent: async (id: string, params: { q: string; caseSensitive?: boolean; signal?: AbortSignal }) =>
		normalizeContentSearchResponse(await request<WorkspaceContentSearchResponse>(`/api/items/${encodeURIComponent(id)}/content-search?${contentSearchQuery(params).toString()}`, params.signal ? { signal: params.signal } : undefined, !params.signal)),
  file: (id: string, fileId: string, expectedCommit?: string, signal?: AbortSignal) => {
		const query = expectedCommit ? `?expectedCommit=${encodeURIComponent(expectedCommit)}` : '';
		return request<FileContent>(`/api/items/${encodeURIComponent(id)}/files/${encodeURIComponent(fileId)}${query}`, { signal }, !signal);
	},
  saveFile: (id: string, fileId: string, input: FileSaveInput) =>
    request<FileContent>(`/api/items/${id}/files/${fileId}`, { method: 'POST', body: JSON.stringify(input) }),
  revertFile: (id: string, fileId: string) =>
    request<ScanResult>(`/api/items/${id}/files/${fileId}/revert`, { method: 'POST' }),
  saveMetadata: (id: string, input: ItemMetadataUpdateInput) =>
    request<WriteResult>(`/api/items/${id}/metadata`, { method: 'PATCH', body: JSON.stringify(input) }),
  updateStatus: (id: string, input: ItemStatusUpdateInput) =>
    request<WriteResult>(`/api/items/${id}/status`, { method: 'PATCH', body: JSON.stringify(input) }),
  createItem: (input: NewItemInput) => request<WriteResult>('/api/items', { method: 'POST', body: JSON.stringify(input) }),
  diff: (id: string, signal?: AbortSignal) => request<{ diff: string }>(`/api/items/${id}/diff`, { signal }, !signal)
};
