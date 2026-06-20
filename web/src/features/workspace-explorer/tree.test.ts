import { describe, expect, it } from 'vitest';
import { buildItemDecorations, directoryCacheKey, explorerNodeId, flattenVisibleTree } from './tree';
import { treeKeyboardAction } from './keyboard';
import type { DirectoryCacheEntry } from './types';
import type { ItemSummary, WorkspaceConfig } from '../../lib/types';

const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: [], createdAt: '' } satisfies WorkspaceConfig;

describe('workspace explorer tree', () => {
  it('flattens only expanded cached directories and decorates item paths', () => {
    const expanded = new Set([explorerNodeId('ws', ''), explorerNodeId('ws', 'plans')]);
    const cache = new Map<string, DirectoryCacheEntry>([
      [directoryCacheKey('ws', '', false), { state: 'loaded', hiddenCount: 0, entries: [{ id: 'plans', name: 'plans', path: 'plans', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]}],
      [directoryCacheKey('ws', 'plans', false), { state: 'loaded', hiddenCount: 0, entries: [{ id: 'ticket', name: 'PM-007', path: 'plans/PM-007', type: 'directory', hasChildren: false, ignored: false, hidden: false, editable: false }]}]
    ]);
    const item: ItemSummary = {
      id: 'item', workspaceId: 'ws', workspaceName: 'Workspace', itemPath: 'plans/PM-007', identifier: 'PM-007',
      title: 'Explorer', status: 'draft', tags: [], branch: 'main', scope: 'platform', metadataSource: 'plan.yaml'
    };
    const rows = flattenVisibleTree({ workspaces: [workspace], expandedNodeIds: expanded, cache, includeIgnored: false, decorations: buildItemDecorations([item]) });
    expect(rows.map((row) => row.node.path)).toEqual(['', 'plans', 'plans/PM-007']);
    expect(rows[2].item?.identifier).toBe('PM-007');
  });

  it('supports roving tree keyboard actions', () => {
    const rows = flattenVisibleTree({ workspaces: [workspace], expandedNodeIds: new Set(), cache: new Map(), includeIgnored: false, decorations: new Map() });
    expect(treeKeyboardAction('ArrowRight', rows, 0, new Set())).toEqual({ activeIndex: 0, toggleNodeId: 'ws:' });
    expect(treeKeyboardAction('Enter', rows, 0, new Set()).select?.workspaceId).toBe('ws');
  });
});
