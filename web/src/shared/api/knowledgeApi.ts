import type { E2ERunbookList, KnowledgeActionResult, KnowledgeGraph, KnowledgePageDetail, KnowledgePagesResponse, KnowledgeWiki } from '../../lib/types';
import { request } from './http';
import { knowledgeURL } from './normalize';

export const knowledgeApi = {
	knowledgeWikis: async (workspaceId: string, signal?: AbortSignal) => (await request<KnowledgeWiki[] | null>(`/api/knowledge/wikis?workspaceId=${encodeURIComponent(workspaceId)}`, { signal }, !signal)) ?? [],
	knowledgePages: (workspaceId: string, root: string, signal?: AbortSignal) => request<KnowledgePagesResponse>(knowledgeURL(workspaceId, root, 'pages'), { signal }, !signal),
	knowledgePage: (workspaceId: string, root: string, slug: string, signal?: AbortSignal) => request<KnowledgePageDetail>(`${knowledgeURL(workspaceId, root, 'pages')}/${encodeURIComponent(slug)}`, { signal }, !signal),
	knowledgeE2ERunbook: (workspaceId: string, root: string, slug: string, signal?: AbortSignal) => request<E2ERunbookList>(`${knowledgeURL(workspaceId, root, 'pages')}/${encodeURIComponent(slug)}/e2e-runbook`, { signal }, !signal),
	knowledgeGraph: (workspaceId: string, root: string, signal?: AbortSignal) => request<KnowledgeGraph>(knowledgeURL(workspaceId, root, 'graph'), { signal }, !signal),
	rescanKnowledge: (workspaceId: string, root: string) => request<KnowledgeActionResult>(knowledgeURL(workspaceId, root, 'rescan'), { method: 'POST' }),
	syncKnowledge: (workspaceId: string, confirm = false) => request<KnowledgeActionResult>(`/api/knowledge/workspaces/${encodeURIComponent(workspaceId)}/sync`, { method: 'POST', body: JSON.stringify({ confirm }) }),
	enrichKnowledge: (workspaceId: string, confirm: boolean) => request<KnowledgeActionResult>(`/api/knowledge/workspaces/${encodeURIComponent(workspaceId)}/enrich`, { method: 'POST', body: JSON.stringify({ confirm }) })
};
