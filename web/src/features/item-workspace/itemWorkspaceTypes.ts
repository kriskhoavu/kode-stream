

export type Tab = 'preview' | 'raw' | 'diff';
export type RightPanelTab = 'info' | 'jira' | 'quality' | 'agents';
export type DiffMode = 'review' | 'raw';
export type PendingConfirm = { title: string; message: string; confirmLabel: string; danger?: boolean; onConfirm: () => void };
export type DetailViewMode = 'plan' | 'workspace' | 'git';
export type OpenItemFileTab = { id: string; path: string; name: string; editable: boolean };
