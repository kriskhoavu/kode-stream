import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';

/**
 * Closes an open menu or popover when the pointer goes down outside `ref`, or Escape is pressed.
 * Listens only while `open`; `onDismiss` may change on every render without resubscribing.
 */
export function useDismiss(ref: RefObject<HTMLElement | null>, open: boolean, onDismiss: () => void) {
	const dismissRef = useRef(onDismiss);
	dismissRef.current = onDismiss;
	useEffect(() => {
		if (!open) return;
		const outside = (event: PointerEvent) => {
			if (ref.current && !ref.current.contains(event.target as Node | null)) dismissRef.current();
		};
		const escape = (event: KeyboardEvent) => {
			if (event.key === 'Escape') dismissRef.current();
		};
		document.addEventListener('pointerdown', outside);
		document.addEventListener('keydown', escape);
		return () => {
			document.removeEventListener('pointerdown', outside);
			document.removeEventListener('keydown', escape);
		};
	}, [open, ref]);
}
