export type ItemStatus = 'unsorted' | 'draft' | 'in_progress' | 'review' | 'done';

export type AuditStatus = 'success' | 'blocked' | 'failed';
export type HealthStatus = 'ok' | 'warning' | 'failed';

export interface AuditEvent {
  id: string;
  time: string;
  ownerUserId?: string;
  actorUserId?: string;
  workspaceId?: string;
  itemId?: string;
  operation: string;
  status: AuditStatus;
  message: string;
  paths: string[];
  durationMs: number;
  error?: string;
}

export interface HealthCheck {
  name: string;
  status: HealthStatus;
  message: string;
  recoveryHint?: string;
}

export interface WorkspaceHealth {
  workspaceId: string;
  checkedAt: string;
  checks: HealthCheck[];
  summary: HealthStatus;
}

export interface SearchResult {
  id: string;
  type: 'item' | 'workspace' | 'branch' | 'savedFilter';
  title: string;
  subtitle: string;
  context: string;
  workspaceId?: string;
  itemId?: string;
  route: string;
  score: number;
}

export interface SavedFilter {
  id: string;
  name: string;
  route: string;
  workspaceId?: string;
  filters: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface RecentItem {
  itemId: string;
  workspaceId: string;
  title: string;
  subtitle?: string;
  route: string;
  openedAt: string;
}

export type SourceSurface = 'workstream' | 'knowledge';
export interface WorkspaceSource { path: string; shownIn: SourceSurface[]; }
// The server accepts a bare path. Read sources through lib/sources.ts.
export type WorkspaceSourceLike = string | WorkspaceSource;
