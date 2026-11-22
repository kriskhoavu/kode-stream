import { type FileContent } from './item';

export interface KnowledgeSettings {
  enabled?: boolean;
  enrichExecutable?: string;
  enrichArgs?: string[];
}

export type KnowledgeLinkResolution = 'resolved' | 'unresolved';
export interface KnowledgeLink { sourceSlug: string; rawTarget: string; label?: string; targetSlug?: string; resolution: KnowledgeLinkResolution; }
export interface KnowledgeWarning { workspaceId?: string; wikiRoot?: string; path?: string; slug?: string; code: string; message: string; }
export interface KnowledgePage { slug: string; title: string; path: string; domain: string; bucket?: string; area?: string; tier?: string; pageType?: string; roles: string[]; topics: string[]; summary?: string; sourceRefs: string[]; sourceCount?: number; links: KnowledgeLink[]; backlinks: string[]; }
export interface KnowledgeWiki { workspaceId: string; root: string; displayName: string; journeysBucket?: string; pages: KnowledgePage[]; warnings: KnowledgeWarning[]; indexedAt: string; }
export interface KnowledgePageDetail extends KnowledgePage { content: FileContent; warnings: KnowledgeWarning[]; }
export interface KnowledgePagesResponse { pages: KnowledgePage[]; warnings: KnowledgeWarning[]; }
export interface KnowledgeGraphNode { id: string; title: string; domain: string; bucket?: string; area?: string; tier?: string; pageType?: string; roles: string[]; topics: string[]; path: string; inbound: number; outbound: number; }
export interface KnowledgeGraphEdge { source: string; target: string; }
export interface KnowledgeGraph { nodes: KnowledgeGraphNode[]; edges: KnowledgeGraphEdge[]; totalNodes: number; totalEdges: number; truncated: boolean; }
export interface KnowledgeActionResult { ok: boolean; operation: 'rescan' | 'sync' | 'enrich'; message?: string; wikis: KnowledgeWiki[]; warnings: KnowledgeWarning[]; log?: string; logTruncated: boolean; completedAt: string; }
