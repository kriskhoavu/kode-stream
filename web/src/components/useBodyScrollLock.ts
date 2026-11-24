import { useEffect } from 'react';

export function useBodyScrollLock(locked = true) {
	useEffect(() => {
		if (!locked) return;
		const previousBodyOverflow = document.body.style.overflow;
		const mainContent = document.querySelector<HTMLElement>('.main-content');
		const previousMainContentOverflow = mainContent?.style.overflow ?? '';
		document.body.style.overflow = 'hidden';
		if (mainContent) mainContent.style.overflow = 'hidden';
		return () => {
			document.body.style.overflow = previousBodyOverflow;
			if (mainContent) mainContent.style.overflow = previousMainContentOverflow;
		};
	}, [locked]);
}
