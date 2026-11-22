import { useRef, useState } from 'react';
import type { DragEvent } from 'react';
import type { HarnessTask, ItemStatus } from '../../lib/types';
import { isDropStatus } from './dragAndDrop';
import { taskStartColumn } from './boardPreferences';

/** How long after a drag a click on the dragged card is not read as a click. */
const DRAG_CLICK_SUPPRESSION_MS = 350;

/**
 * Drag and drop on the board. On the Plans board a card moves to the column it
 * is dropped on; on the Tasks board a task that has not started may only be
 * dropped on the start column, which starts it.
 */
export function useBoardDrag({ tasksShown, canDragPlan, startableTask, onStartTask, onMovePlan }: {
	tasksShown: boolean;
	canDragPlan: (itemId: string) => boolean;
	startableTask: (itemId: string) => HarnessTask | undefined;
	onStartTask: (task: HarnessTask) => void;
	onMovePlan: (itemId: string, status: ItemStatus) => void;
}) {
	const [activeItemId, setActiveItemId] = useState('');
	const [dragTargetStatus, setDragTargetStatus] = useState<ItemStatus | ''>('');
	const suppressPreviewRef = useRef<{ itemId: string; until: number } | null>(null);

	const onCardDragStart = (event: DragEvent<HTMLElement>, itemId: string) => {
		const draggable = tasksShown ? Boolean(startableTask(itemId)) : canDragPlan(itemId);
		if (!draggable) { event.preventDefault(); return; }
		event.dataTransfer.effectAllowed = 'move';
		event.dataTransfer.setData('text/plain', itemId);
		setActiveItemId(itemId);
		suppressPreviewRef.current = { itemId, until: Number.POSITIVE_INFINITY };
	};

	const onCardDragEnd = (itemId: string) => {
		suppressPreviewRef.current = { itemId, until: window.performance.now() + DRAG_CLICK_SUPPRESSION_MS };
		setActiveItemId('');
		setDragTargetStatus('');
	};

	const onColumnDragOver = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
		if (!activeItemId || !isDropStatus(status) || (tasksShown && status !== taskStartColumn)) return;
		event.preventDefault();
		event.dataTransfer.dropEffect = 'move';
		if (dragTargetStatus !== status) setDragTargetStatus(status);
	};

	const onColumnDragLeave = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
		if (dragTargetStatus === status && !event.currentTarget.contains(event.relatedTarget as Node | null)) setDragTargetStatus('');
	};

	const onColumnDrop = (event: DragEvent<HTMLElement>, status: ItemStatus) => {
		const itemId = event.dataTransfer.getData('text/plain') || activeItemId;
		if (!itemId || !isDropStatus(status)) return;
		if (tasksShown) {
			const task = status === taskStartColumn ? startableTask(itemId) : undefined;
			if (!task) return;
			event.preventDefault();
			setDragTargetStatus('');
			onStartTask(task);
			return;
		}
		event.preventDefault();
		setDragTargetStatus('');
		onMovePlan(itemId, status);
	};

	/** Whether a click on a card opens its preview: not the click that ends a drag. */
	const previewAllowed = (itemId: string) => {
		const suppressed = suppressPreviewRef.current;
		if (suppressed?.itemId === itemId && window.performance.now() <= suppressed.until) return false;
		suppressPreviewRef.current = null;
		return true;
	};

	return { activeItemId, dragTargetStatus, onCardDragStart, onCardDragEnd, onColumnDragOver, onColumnDragLeave, onColumnDrop, previewAllowed };
}
