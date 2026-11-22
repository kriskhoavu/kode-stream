import { useCallback, useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';

export interface HorizontalDrag {
	/** Called with the pointer's distance from where the drag started. */
	move: (deltaX: number) => void;
	/** Called once when the pointer is released; not called when the drag is cancelled. */
	end?: () => void;
}

/**
 * A horizontal pointer drag over the whole window, as a resize grip needs. One drag runs at a time:
 * starting another, or unmounting, cancels the one in progress. While it runs the body carries
 * `bodyClass`, which the stylesheet uses to keep the cursor and stop text selection.
 */
export function useHorizontalDrag(bodyClass = 'is-resizing-panel') {
	const cancelRef = useRef<(() => void) | null>(null);
	useEffect(() => () => cancelRef.current?.(), []);
	return useCallback((event: ReactPointerEvent, drag: HorizontalDrag) => {
		event.preventDefault();
		cancelRef.current?.();
		const startX = event.clientX;
		const onPointerMove = (moveEvent: PointerEvent) => drag.move(moveEvent.clientX - startX);
		const finish = (released: boolean) => {
			document.body.classList.remove(bodyClass);
			window.removeEventListener('pointermove', onPointerMove);
			window.removeEventListener('pointerup', onPointerUp);
			if (cancelRef.current === cancel) cancelRef.current = null;
			if (released) drag.end?.();
		};
		const cancel = () => finish(false);
		function onPointerUp() { finish(true); }
		document.body.classList.add(bodyClass);
		window.addEventListener('pointermove', onPointerMove);
		window.addEventListener('pointerup', onPointerUp, { once: true });
		cancelRef.current = cancel;
	}, [bodyClass]);
}
