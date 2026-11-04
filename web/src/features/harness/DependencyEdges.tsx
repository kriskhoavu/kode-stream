import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';
import { edgeKey } from './waves';

export interface Anchor { left: number; top: number; width: number; height: number }

/**
 * Where each card sits inside the board, measured relative to the board so a
 * scroll does not move it. Cards register a ref; a ResizeObserver on the
 * board, scrolls and card changes schedule one measurement per frame.
 */
export function useCardAnchors(boardRef: RefObject<HTMLElement | null>, layoutKey: string) {
	const elements = useRef(new Map<string, HTMLElement>());
	const frame = useRef<number | undefined>(undefined);
	const [anchors, setAnchors] = useState<{ map: Map<string, Anchor>; width: number; height: number }>({ map: new Map(), width: 0, height: 0 });

	const measure = useCallback(() => {
		if (frame.current !== undefined) return;
		frame.current = window.requestAnimationFrame(() => {
			frame.current = undefined;
			const board = boardRef.current;
			if (!board) return;
			const base = board.getBoundingClientRect();
			const map = new Map<string, Anchor>();
			elements.current.forEach((element, id) => {
				const rect = element.getBoundingClientRect();
				map.set(id, { left: rect.left - base.left, top: rect.top - base.top, width: rect.width, height: rect.height });
			});
			setAnchors({ map, width: board.scrollWidth, height: board.scrollHeight });
		});
	}, [boardRef]);

	const register = useCallback((id: string, element: HTMLElement | null) => {
		if (element) elements.current.set(id, element);
		else elements.current.delete(id);
	}, []);

	useLayoutEffect(() => {
		measure();
	}, [layoutKey, measure]);

	useEffect(() => {
		const board = boardRef.current;
		if (!board) return;
		const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(() => measure());
		observer?.observe(board);
		window.addEventListener('resize', measure);
		board.addEventListener('scroll', measure, true);
		return () => {
			observer?.disconnect();
			window.removeEventListener('resize', measure);
			board.removeEventListener('scroll', measure, true);
			if (frame.current !== undefined) window.cancelAnimationFrame(frame.current);
			frame.current = undefined;
		};
	}, [boardRef, measure]);

	return { anchors, register };
}

export function edgePath(from: Anchor, to: Anchor): string {
	const x1 = from.left + from.width;
	const y1 = from.top + from.height / 2;
	const x2 = to.left;
	const y2 = to.top + to.height / 2;
	const dx = Math.max(40, (x2 - x1) / 2);
	return `M${x1} ${y1} C${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
}

/**
 * The curves between cards: decoration for sighted readers, so hidden from
 * assistive technology (each card's label and the drawer say what it waits for).
 */
export function DependencyEdges({ edges, anchors, width, height, criticalEdges, showCritical, focusId, liveIds }: {
	edges: [string, string][];
	anchors: Map<string, Anchor>;
	width: number;
	height: number;
	criticalEdges: Set<string>;
	showCritical: boolean;
	focusId?: string;
	liveIds: Set<string>;
}) {
	const drawn = edges.filter(([from, to]) => anchors.has(from) && anchors.has(to));
	const dimming = Boolean(focusId) || showCritical;
	return (
		<svg className={dimming ? 'hx-edges dim' : 'hx-edges'} width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true" focusable="false">
			{drawn.map(([from, to]) => {
				const key = edgeKey(from, to);
				const critical = showCritical && criticalEdges.has(key);
				const related = focusId ? from === focusId || to === focusId : critical;
				const classes = [critical ? 'crit' : '', related ? 'on' : '', liveIds.has(from) && critical ? 'live' : ''].filter(Boolean).join(' ');
				return <path key={key} data-edge={key} className={classes || undefined} d={edgePath(anchors.get(from) as Anchor, anchors.get(to) as Anchor)} />;
			})}
		</svg>
	);
}
