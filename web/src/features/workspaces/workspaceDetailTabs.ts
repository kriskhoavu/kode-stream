

export type WorkspaceDetailTab = 'overview' | 'health' | 'integrations' | 'agents';

export const detailTabs: WorkspaceDetailTab[] = ['overview', 'health', 'integrations', 'agents'];
export const detailTabLabels: Record<WorkspaceDetailTab, string> = { overview: 'Overview', health: 'Health', integrations: 'Integrations', agents: 'AI and harness' };
export type WorkspaceEditSection = 'general' | 'sources' | 'jira' | 'knowledge' | 'runtime' | '';
export type OverviewSectionKey = 'general' | 'sources';
export function workspaceDetailSubtitle(tab: WorkspaceDetailTab): string {
  switch (tab) {
  case 'health':
    return 'Workspace health and diagnostics';
  case 'integrations':
    return 'Services and task sources';
  case 'agents':
    return 'Provider, harness and models';
  default:
    return 'General and sources';
  }
}

export function isIntegrationEditSection(section: WorkspaceEditSection): section is 'jira' | 'knowledge' | 'runtime' {
  return section === 'jira' || section === 'knowledge' || section === 'runtime';
}

export function integrationSaveLabel(section: 'jira' | 'knowledge' | 'runtime'): string {
  switch (section) {
  case 'jira':
    return 'Save Jira';
  case 'knowledge':
    return 'Save Knowledge';
  default:
    return 'Save runtime';
  }
}
