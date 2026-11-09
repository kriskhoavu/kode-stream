import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';

const HEADER_HEIGHT = 68;

export type DrawerStyle = CSSProperties & Record<'--drawer-width' | '--drawer-top', string>;

/**
 * The right-hand drawer's geometry: a width the grip on its left edge drags between `min` and `max`,
 * and a top that follows the page header out of view as the page scrolls.
 */
export function useResizableDrawer({ initial, min = 460, max = 1120 }: { initial: number; min?: number; max?: number }) {
	const [width, setWidth] = useState(initial);
	const [topOffset, setTopOffset] = useState(() => Math.max(0, HEADER_HEIGHT - window.scrollY));
	const cleanupRef = useRef<(() => void) | null>(null);

	useEffect(() => {
		const update = () => setTopOffset(Math.max(0, HEADER_HEIGHT - window.scrollY));
		window.addEventListener('scroll', update, { passive: true });
		return () => window.removeEventListener('scroll', update);
	}, []);

	useEffect(() => () => cleanupRef.current?.(), []);

	const startResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
		event.preventDefault();
		cleanupRef.current?.();
		const startX = event.clientX;
		const startingWidth = width;
		const onPointerMove = (moveEvent: PointerEvent) => {
			setWidth(Math.min(max, Math.max(min, startingWidth + startX - moveEvent.clientX)));
		};
		const cleanup = () => {
			document.body.classList.remove('is-resizing-drawer');
			window.removeEventListener('pointermove', onPointerMove);
			window.removeEventListener('pointerup', cleanup);
			if (cleanupRef.current === cleanup) cleanupRef.current = null;
		};
		document.body.classList.add('is-resizing-drawer');
		window.addEventListener('pointermove', onPointerMove);
		window.addEventListener('pointerup', cleanup, { once: true });
		cleanupRef.current = cleanup;
	};

	const style: DrawerStyle = { '--drawer-width': `${width}px`, '--drawer-top': `${topOffset}px` };
	return { width, style, startResize };
}
