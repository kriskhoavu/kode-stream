import type { ItemDetail, ItemVerificationTests, ItemSummary, WorkspaceContentSearchResponse } from '../../lib/types';

export function contentSearchQuery(params: { q: string; caseSensitive?: boolean }): URLSearchParams {
	const query = new URLSearchParams({ q: params.q });
	if (params.caseSensitive) query.set('caseSensitive', 'true');
	return query;
}

export function normalizeContentSearchResponse(response: WorkspaceContentSearchResponse): WorkspaceContentSearchResponse {
	return {
		...response,
		results: Array.isArray(response.results) ? response.results : [],
		truncated: Boolean(response.truncated),
		filesVisited: Number(response.filesVisited) || 0,
		bytesRead: Number(response.bytesRead) || 0,
		skippedFiles: Number(response.skippedFiles) || 0
	};
}

export function normalizeItemVerificationTests(input: ItemVerificationTests): ItemVerificationTests {
  return {
    selection: {
      selectedSpecs: Array.isArray(input.selection?.selectedSpecs) ? input.selection.selectedSpecs : [],
      environment: input.selection?.environment ?? '',
      displayMode: input.selection?.displayMode === 'visible' ? 'visible' : 'silent',
      updatedAt: input.selection?.updatedAt
    },
    discoveredSpecs: Array.isArray(input.discoveredSpecs) ? input.discoveredSpecs : []
  };
}

export function normalizeItem(item: ItemSummary): ItemSummary {
  return {
    ...item,
    sourceMode: item.sourceMode === 'snapshot' ? 'snapshot' : 'working_tree',
    editable: Boolean(item.editable),
    tags: Array.isArray(item.tags) ? item.tags : []
  };
}

export function normalizeItemDetail(item: ItemDetail): ItemDetail {
  return {
    ...normalizeItem(item),
    documents: Array.isArray(item.documents) ? item.documents : [],
    metadata: item.metadata ?? {},
    counts: item.counts ?? { files: 0 }
  };
}
