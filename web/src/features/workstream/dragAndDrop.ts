import { editableStatusOrder } from '../../lib/api';
import type { ItemStatus, ItemSummary } from '../../lib/types';
import { isDocumentationMetadataSource } from '../../lib/vocabulary';

export function isItemDraggable(item: ItemSummary): boolean {
	return item.status !== 'unsorted' && !isDocumentationMetadataSource(item.metadataSource);
}

export function isDropStatus(status: ItemStatus): boolean {
  return editableStatusOrder.some((candidate) => candidate === status);
}

export function applyItemStatus(items: ItemSummary[], itemId: string, status: ItemStatus): ItemSummary[] {
  return items.map((item) => item.id === itemId ? { ...item, status } : item);
}
