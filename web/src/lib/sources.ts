import type { SourceSurface, WorkspaceConfig, WorkspaceSource, WorkspaceSourceLike } from './types';

export const sourceSurfaces: SourceSurface[] = ['workstream', 'knowledge'];

// A source named `plans` is Workstream only, one named `wiki` is Knowledge
// only, anything else is both. The server applies the same defaults.
export function defaultShownIn(path: string): SourceSurface[] {
  const name = path.trim().replace(/\\/g, '/').replace(/\/+$/, '').split('/').filter(Boolean).pop() ?? '';
  if (name === 'plans') return ['workstream'];
  if (name === 'wiki') return ['knowledge'];
  return ['workstream', 'knowledge'];
}

export function toWorkspaceSource(source: WorkspaceSourceLike): WorkspaceSource {
  if (typeof source === 'string') return { path: source, shownIn: defaultShownIn(source) };
  const path = typeof source?.path === 'string' ? source.path : '';
  const shownIn = Array.isArray(source?.shownIn) ? source.shownIn.filter((surface): surface is SourceSurface => sourceSurfaces.includes(surface)) : [];
  return { path, shownIn: shownIn.length > 0 ? sourceSurfaces.filter((surface) => shownIn.includes(surface)) : defaultShownIn(path) };
}

export function normalizeSources(sources: unknown): WorkspaceSource[] {
  if (!Array.isArray(sources)) return [];
  return sources.filter((source) => typeof source === 'string' || (source && typeof source === 'object')).map((source) => toWorkspaceSource(source as WorkspaceSourceLike));
}

export function workspaceSources(workspace?: Pick<WorkspaceConfig, 'sources'> | null): WorkspaceSource[] {
  return normalizeSources(workspace?.sources);
}

export function sourcePaths(workspace?: Pick<WorkspaceConfig, 'sources'> | null): string[] {
  return workspaceSources(workspace).map((source) => source.path);
}

export function sourcesShownIn(workspace: Pick<WorkspaceConfig, 'sources'> | null | undefined, surface: SourceSurface): string[] {
  return workspaceSources(workspace).filter((source) => source.shownIn.includes(surface)).map((source) => source.path);
}
