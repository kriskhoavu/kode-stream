import { describe, expect, it } from 'vitest';
import type { ItemSummary } from '../../lib/types';
import { applyItemStatus, isDropStatus, isItemDraggable } from './dragAndDrop';
import { filterPlans } from './filtering';

const item: ItemSummary = {
  id: 'item-1',
  workspaceId: 'workspace-1',
  workspaceName: 'Workspace',
  branch: 'main',
  scope: 'platform',
  identifier: 'PM-012',
  title: 'Drag cards',
  status: 'draft',
  tags: [],
  metadataSource: 'plan.yaml'
};

describe('Workstream drag and drop helpers', () => {
  it('allows structured workflow items to move between editable statuses', () => {
    expect(isItemDraggable(item)).toBe(true);
    expect(isDropStatus('review')).toBe(true);
  });

  it('protects unsorted and freestyle docs items', () => {
    expect(isItemDraggable({ ...item, status: 'unsorted' })).toBe(false);
    expect(isItemDraggable({ ...item, metadataSource: 'docs' })).toBe(false);
    expect(isDropStatus('unsorted')).toBe(false);
  });

  it('updates only the matching item without mutating the input', () => {
    const other = { ...item, id: 'item-2', title: 'Other item' };
    const items = [item, other];

    const result = applyItemStatus(items, item.id, 'review');

    expect(result).toEqual([{ ...item, status: 'review' }, other]);
    expect(result).not.toBe(items);
    expect(result[0]).not.toBe(item);
    expect(result[1]).toBe(other);
    expect(item.status).toBe('draft');
  });

  it('lets existing status filters decide visibility after an optimistic move', () => {
    const moved = applyItemStatus([item], item.id, 'review');
    const draftOnly = { sources: [], scopes: [], statuses: ['draft'], branches: [], authors: [] };
    const draftAndReview = { ...draftOnly, statuses: ['draft', 'review'] };

    expect(filterPlans(moved, draftOnly, '')).toEqual([]);
    expect(filterPlans(moved, draftAndReview, '')).toEqual(moved);
  });
});
