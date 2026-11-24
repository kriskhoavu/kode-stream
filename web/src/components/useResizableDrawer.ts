import { useEffect, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { useHorizontalDrag } from './useHorizontalDrag';

const HEADER_HEIGHT = 68;

export type DrawerStyle = CSSProperties & Record<'--drawer-width' | '--drawer-top', string>;

export function useResizableDrawer({ initial, min = 460, max = 1120 }: { initial: number; min?: number; max?: number }) {
	const [width, setWidth] = useState(initial);
	const [topOffset, setTopOffset] = useState(() => Math.max(0, HEADER_HEIGHT - window.scrollY));

	useEffect(() => {
		const update = () => setTopOffset(Math.max(0, HEADER_HEIGHT - window.scrollY));
		window.addEventListener('scroll', update, { passive: true });
		return () => window.removeEventListener('scroll', update);
	}, []);

	const startDrag = useHorizontalDrag('is-resizing-drawer');
	const startResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
		const startingWidth = width;
		startDrag(event, { move: (deltaX) => setWidth(Math.min(max, Math.max(min, startingWidth - deltaX))) });
	};

	const style: DrawerStyle = { '--drawer-width': `${width}px`, '--drawer-top': `${topOffset}px` };
	return { width, style, startResize };
}
