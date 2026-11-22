import { useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { useHorizontalDrag } from '../../components/useHorizontalDrag';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { boundedLeftPanelWidth, boundedNumber, boundedRightPanelWidth } from './explorerLayout';

const widthPreference = { left: 'workstreamExplorer.leftWidth', right: 'workstreamExplorer.rightWidth' } as const;
const widthProperty = { left: '--explorer-left-width', right: '--explorer-right-width' } as const;

/**
 * The Explorer's side panels: the tree panel's collapse, and both panels' widths, which a grip drags
 * within their bounds and which are remembered when the drag ends. `gridRef` goes on the grid whose
 * width properties a drag updates directly.
 */
export function useExplorerPanels() {
	const gridRef = useRef<HTMLDivElement | null>(null);
	const [leftCollapsed, setLeftCollapsed] = useState(false);
	const [leftWidth, setLeftWidth] = useState(() => boundedNumber(readStringPreference(widthPreference.left), 400, boundedLeftPanelWidth));
	const [rightWidth, setRightWidth] = useState(() => boundedNumber(readStringPreference(widthPreference.right), 300, boundedRightPanelWidth));
	const startDrag = useHorizontalDrag();

	const startResize = (side: 'left' | 'right', event: ReactPointerEvent<HTMLButtonElement>) => {
		const initial = side === 'left' ? leftWidth : rightWidth;
		let latest = initial;
		startDrag(event, {
			move: (deltaX) => {
				latest = side === 'left' ? boundedLeftPanelWidth(initial + deltaX) : boundedRightPanelWidth(initial - deltaX);
				gridRef.current?.style.setProperty(widthProperty[side], `${latest}px`);
				if (side === 'left') setLeftWidth(latest); else setRightWidth(latest);
			},
			end: () => writePreference(widthPreference[side], String(latest)),
		});
	};

	return { gridRef, leftCollapsed, setLeftCollapsed, leftWidth, rightWidth, startResize };
}
