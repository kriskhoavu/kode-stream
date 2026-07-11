import type { ItemSummary, WorkspaceConfig, WorkspaceTreeEntry } from '../../lib/types';

import type { ExplorerTreeMode } from '../../lib/types';

export interface ExplorerLocation {
  workspaceId?: string;
  path?: string;
  mode?: ExplorerTreeMode;
}

export type ExplorerNodeKind = 'workspace' | 'directory' | 'file';

export interface ExplorerSelection {
  nodeId: string;
  kind: ExplorerNodeKind;
  workspaceId: string;
  path: string;
}

export interface ExplorerItemDecoration {
  itemId: string;
  identifier: string;
  title: string;
  status: ItemSummary['status'];
  owner?: string;
  branch: string;
  warnings: number;
}

export interface WorkspaceRootNode {
  id: string;
  name: string;
  path: string;
  type: 'workspace';
  workspace: WorkspaceConfig;
  hasChildren: boolean;
}

export interface DirectoryCacheEntry {
  state: 'idle' | 'loading' | 'loaded' | 'error';
  entries: WorkspaceTreeEntry[];
  hiddenCount: number;
  error?: string;
}

export interface VisibleExplorerRow {
  node: WorkspaceTreeEntry | WorkspaceRootNode;
  level: number;
  parentId?: string;
  positionInSet: number;
  setSize: number;
  workspaceId: string;
  item?: ExplorerItemDecoration;
}
