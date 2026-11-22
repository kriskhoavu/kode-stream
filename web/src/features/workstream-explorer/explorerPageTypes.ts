import type { ReactNode } from 'react';
import type { WorkspacePathSearchResult } from '../../lib/types';
import type { WorkspaceContentSearchResult } from '../../lib/types';

export type EditorTab = 'preview' | 'raw' | 'diff';
export type PathDialog = { kind: 'file' | 'directory' | 'rename'; parentPath: string; currentPath?: string; initialName?: string };
export type ExplorerUnifiedResult = { kind: 'path'; result: WorkspacePathSearchResult } | { kind: 'content'; result: WorkspaceContentSearchResult };
export type OpenWorkspaceFileTab = { workspaceId: string; path: string; name: string; editable: boolean };
export type ExplorerRightPanelProps = {
  title?: ReactNode;
  content: ReactNode;
  collapsed?: boolean;
  onToggle?: () => void;
  className?: string;
  collapsedLabel?: string;
  expandedLabel?: string;
};
