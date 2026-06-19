import type { ItemSummary, WorkspaceConfig } from '../../lib/types';

export type FilterKey = 'sources' | 'scopes' | 'statuses' | 'branches' | 'authors';
export type Filters = Record<FilterKey, string[]>;
export type FacetOption = { value: string; label: string };

export const emptyFilters: Filters = {
  sources: [],
  scopes: [],
  statuses: [],
  branches: [],
  authors: []
};

export function filterPlans(items: ItemSummary[], filters: Filters, text: string, workspace?: WorkspaceConfig): ItemSummary[] {
  const query = text.trim().toLowerCase();
  return items.filter((plan) => {
    if (filters.sources.length > 0 && !filters.sources.includes(sourceRoot(plan, workspace))) return false;
    const scope = plan.scope || 'Unknown';
    if (filters.scopes.length > 0 && !filters.scopes.includes(scope)) return false;
    if (filters.statuses.length > 0 && !filters.statuses.includes(plan.status)) return false;
    if (filters.branches.length > 0 && !filters.branches.includes(plan.branch)) return false;
    const author = plan.author || plan.owner || 'Unknown';
    if (filters.authors.length > 0 && !filters.authors.includes(author)) return false;
    if (query && !planSearchText(plan).includes(query)) return false;
    return true;
  });
}

export function sourceFacetOptions(items: ItemSummary[], workspace?: WorkspaceConfig): FacetOption[] {
  const roots = new Set(items.map((plan) => sourceRoot(plan, workspace)).filter(Boolean));
  return Array.from(roots)
    .sort((a, b) => a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' }))
    .map((root) => ({ value: root, label: root }));
}

export function sourceLabel(plan: ItemSummary, workspace?: WorkspaceConfig): string {
  return sourceRoot(plan, workspace);
}

function sourceRoot(plan: ItemSummary, workspace?: WorkspaceConfig): string {
  const root = plan.itemPath || '';
  const directories = workspace?.sources ?? [];
  const matched = directories
    .filter((directory) => root === directory || root.startsWith(`${directory}/`))
    .sort((a, b) => b.length - a.length)[0];
  if (matched) return matched;
  return root.split('/').filter(Boolean)[0] ?? '';
}

function planSearchText(plan: ItemSummary): string {
  return [
    plan.title,
    plan.identifier,
    plan.scope,
    plan.branch,
    plan.workspaceName,
    plan.author,
    plan.owner,
    plan.description,
    plan.metadataSource,
    ...plan.tags
  ].filter(Boolean).join(' ').toLowerCase();
}
