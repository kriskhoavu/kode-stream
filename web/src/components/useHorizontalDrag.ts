import { useCallback, useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';

export interface HorizontalDrag {
	move: (deltaX: number) => void;
	/** Not called when the drag is cancelled. */
	end?: () => void;
}

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
