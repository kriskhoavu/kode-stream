import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';
import { useReactFlow } from '@xyflow/react';
import type { CanvasViewport } from '../../lib/types';

export function CanvasSearchViewport({ query }: { query: string }) {
	const { fitView } = useReactFlow();
	const previousQuery = useRef(query);
	useEffect(() => {
		if (previousQuery.current === query) return;
		previousQuery.current = query;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		const frame = requestAnimationFrame(() => void fitView({ padding: 0.22, maxZoom: 1.15, duration: reduceMotion ? 0 : 180 }));
		return () => cancelAnimationFrame(frame);
	}, [fitView, query]);
	return null;
}

/** Saves the viewport that was current when Focus is switched on and restores
 * it when Focus is switched off; the prior viewport is kept only in a ref
 * hoisted into CanvasBoard, so it survives the `<ReactFlow key={layoutId}>`
 * remount that would otherwise drop a ref living inside it. Focus never
 * writes to the saved layout.
 *
 * Fitting the view is handled separately, keyed on the rendered node list:
 * it re-runs (mount included) whenever that list changes while Focus is on,
 * after a double requestAnimationFrame so the forced-expanded session nodes
 * have been measured before the fit reads their size. */
export function CanvasFocusViewport({ focusOn, nodeIdsKey, savedViewportRef, previousFocusOnRef }: { focusOn: boolean; nodeIdsKey: string; savedViewportRef: RefObject<CanvasViewport | undefined>; previousFocusOnRef: RefObject<boolean> }) {
	const { fitView, getViewport, setViewport } = useReactFlow();
	useEffect(() => {
		if (previousFocusOnRef.current === focusOn) return;
		previousFocusOnRef.current = focusOn;
		if (focusOn) {
			savedViewportRef.current = getViewport();
			return;
		}
		if (!savedViewportRef.current) return;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		void setViewport(savedViewportRef.current, { duration: reduceMotion ? 0 : 200 });
	}, [focusOn, getViewport, previousFocusOnRef, savedViewportRef, setViewport]);
	useEffect(() => {
		if (!focusOn) return;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		const duration = reduceMotion ? 0 : 200;
		let innerFrame = 0;
		const outerFrame = requestAnimationFrame(() => {
			innerFrame = requestAnimationFrame(() => void fitView({ padding: 0.15, maxZoom: 1.2, duration }));
		});
		return () => { cancelAnimationFrame(outerFrame); cancelAnimationFrame(innerFrame); };
	}, [fitView, focusOn, nodeIdsKey]);
	return null;
}
